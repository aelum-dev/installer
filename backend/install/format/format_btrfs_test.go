package format

import (
	"errors"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// listOutput is a realistic `btrfs subvolume list` output covering the canonical
// scheme, so the default subvolume (@) can be located by path.
const listOutput = `ID 258 gen 42 top level 5 path @
ID 260 gen 42 top level 5 path @home
ID 262 gen 42 top level 5 path @srv
`

// hasCall reports whether calls contains argv exactly.
func hasCall(calls [][]string, want ...string) bool {
	for _, argv := range calls {
		if len(argv) != len(want) {
			continue
		}
		match := true
		for i := range want {
			if argv[i] != want[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// TestSetupBtrfsSubvolumes_Success verifies the sequence: mount the top level,
// create each subvolume, set @ as default by its listed ID, apply per-subvolume
// compression, then unmount and clean up.
func TestSetupBtrfsSubvolumes_Success(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"btrfs", "subvolume", "list"}, listOutput)
	defer primitives.SetCommandRunner(rec)()

	subvols := shared.GetBtrfsSubvolumes()
	if err := SetupBtrfsSubvolumes(&shared.InstallContext{}, "/dev/vda2", subvols); err != nil {
		t.Fatalf("SetupBtrfsSubvolumes() = %v, want nil", err)
	}

	tmp := shared.BtrfsSetupTmpMount
	if !hasCall(rec.CallsFor("mount"), "mount", "-o", "subvolid=5", "/dev/vda2", tmp) {
		t.Errorf("top-level mount not issued: %v", rec.CallsFor("mount"))
	}
	creates := rec.CallsFor("btrfs")
	created := 0
	for _, sv := range subvols {
		if hasCall(creates, "btrfs", "subvolume", "create", "-p", tmp+"/"+sv.Name) {
			created++
		}
	}
	if created != len(subvols) {
		t.Errorf("created %d/%d subvolumes", created, len(subvols))
	}
	if !hasCall(creates, "btrfs", "subvolume", "set-default", "258", tmp) {
		t.Errorf("default subvolume (@ ID 258) was not set: %v", creates)
	}
	if !hasCall(creates, "btrfs", "property", "set", tmp+"/@home", "compression", "zstd") {
		t.Errorf("compression was not applied to @home: %v", creates)
	}
	if !hasCall(rec.CallsFor("umount"), "umount", tmp) {
		t.Errorf("top-level mount was not undone: %v", rec.CallsFor("umount"))
	}
	if !hasCall(rec.CallsFor("rmdir"), "rmdir", tmp) {
		t.Errorf("temp mount point was not removed: %v", rec.CallsFor("rmdir"))
	}
}

func TestSetupBtrfsSubvolumes_UnmountFails(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"btrfs", "subvolume", "list"}, listOutput).
		OnArgs([]string{"umount"}, primitivestest.Result{
			Output: []byte("umount: /tmp/btrfs-setup: target is busy"),
			Err:    errors.New("exit status 32"),
		})
	defer primitives.SetCommandRunner(rec)()

	err := SetupBtrfsSubvolumes(&shared.InstallContext{}, "/dev/vda2", shared.GetBtrfsSubvolumes())
	if err == nil || !strings.Contains(err.Error(), "unmount btrfs top-level") {
		t.Fatalf("SetupBtrfsSubvolumes() error = %v, want unmount failure", err)
	}
}

func TestSetupBtrfsSubvolumes_MountFails(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsError([]string{"mount", "-o"}, errors.New("exit status 32"))
	defer primitives.SetCommandRunner(rec)()

	err := SetupBtrfsSubvolumes(&shared.InstallContext{}, "/dev/vda2", shared.GetBtrfsSubvolumes())
	if err == nil || !strings.Contains(err.Error(), "mount btrfs top-level") {
		t.Fatalf("SetupBtrfsSubvolumes() error = %v, want top-level mount failure", err)
	}
}

func TestSetupBtrfsSubvolumes_CreateFails(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsError([]string{"btrfs", "subvolume", "create"}, errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	err := SetupBtrfsSubvolumes(&shared.InstallContext{}, "/dev/vda2", shared.GetBtrfsSubvolumes())
	if err == nil || !strings.Contains(err.Error(), "create subvolume") {
		t.Fatalf("SetupBtrfsSubvolumes() error = %v, want create failure", err)
	}
}

func TestSetupBtrfsSubvolumes_ListFails(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsError([]string{"btrfs", "subvolume", "list"}, errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	err := SetupBtrfsSubvolumes(&shared.InstallContext{}, "/dev/vda2", shared.GetBtrfsSubvolumes())
	if err == nil || !strings.Contains(err.Error(), "list subvolumes") {
		t.Fatalf("SetupBtrfsSubvolumes() error = %v, want list failure", err)
	}
}

// TestSetupBtrfsSubvolumes_DefaultNotFound verifies a list output without the
// default subvolume aborts rather than setting an arbitrary default.
func TestSetupBtrfsSubvolumes_DefaultNotFound(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"btrfs", "subvolume", "list"}, "ID 300 gen 1 top level 5 path @other\n")
	defer primitives.SetCommandRunner(rec)()

	err := SetupBtrfsSubvolumes(&shared.InstallContext{}, "/dev/vda2", shared.GetBtrfsSubvolumes())
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("SetupBtrfsSubvolumes() error = %v, want not-found error", err)
	}
}

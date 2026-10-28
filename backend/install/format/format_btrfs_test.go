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
// scheme, so the initial snapshot under @ can be located by path.
const listOutput = `ID 258 gen 42 top level 5 path @
ID 260 gen 42 top level 5 path @home
ID 262 gen 42 top level 5 path @srv
ID 264 gen 42 top level 258 path @/.snapshots
ID 266 gen 42 top level 264 path @/.snapshots/1/snapshot
`

// withMemFS gives the format step an in-memory filesystem. The initial root
// snapshot writes its info.xml through the file seam, and the host running the
// tests has no /tmp/btrfs-setup to write it into.
func withMemFS() func() {
	return primitives.SetFileSystem(primitivestest.NewMemFS())
}

// btrfsSetupList is what `btrfs subvolume list` reports for a scheme that was just
// created: the subvolumes, the history under the root one, and the snapshot the
// system is installed into.
const btrfsSetupList = "ID 256 gen 1 top level 5 path @\n" +
	"ID 258 gen 1 top level 256 path @/.snapshots\n" +
	"ID 260 gen 1 top level 258 path @/.snapshots/1/snapshot\n"

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
// create each subvolume, build the history and the snapshot the system is installed
// into, set that snapshot as the default by its listed ID, apply per-subvolume
// compression to the subvolume that holds files, then unmount and clean up.
func TestSetupBtrfsSubvolumes_Success(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"btrfs", "subvolume", "list"}, listOutput)
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

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
	if !hasCall(creates, "btrfs", "subvolume", "create", tmp+"/@/.snapshots") {
		t.Errorf("the history of the root subvolume was not created: %v", creates)
	}
	live := tmp + "/" + shared.LiveSubvolumeName(shared.RootSubvolume())
	if !hasCall(creates, "btrfs", "subvolume", "create", live) {
		t.Errorf("the snapshot the system is installed into was not created: %v", creates)
	}
	if !hasCall(creates, "btrfs", "subvolume", "set-default", "266", tmp) {
		t.Errorf("default subvolume (@/.snapshots/1/snapshot, ID 266) was not set: %v", creates)
	}
	if !hasCall(creates, "btrfs", "property", "set", tmp+"/@home", "compression", "zstd") {
		t.Errorf("compression was not applied to @home: %v", creates)
	}
	if !hasCall(creates, "btrfs", "property", "set", live, "compression", "zstd") {
		t.Errorf("compression was not applied to the subvolume that holds the system: %v", creates)
	}
	// snapper only treats .snapshots/<n>/ as a snapshot when info.xml is there; with
	// none it treats the directory as a leftover and removes it.
	info, ok := fs.Read(tmp + "/@/.snapshots/1/info.xml")
	if !ok {
		t.Error("the initial snapshot has no info.xml, so snapper would not see the live root as a snapshot")
	} else {
		for _, want := range []string{"<type>single</type>", "<num>1</num>", "<date>", "first root filesystem"} {
			if !strings.Contains(info, want) {
				t.Errorf("initial snapshot info.xml is missing %q:\n%s", want, info)
			}
		}
	}
	if !hasCall(rec.CallsFor("mkdir"), "mkdir", "-p", live+"/"+shared.BtrfsSnapshotsDirName) {
		t.Error("the live root has no mount point for /.snapshots, so the history could not be mounted in it")
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
	defer withMemFS()()

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
	defer withMemFS()()

	err := SetupBtrfsSubvolumes(&shared.InstallContext{}, "/dev/vda2", shared.GetBtrfsSubvolumes())
	if err == nil || !strings.Contains(err.Error(), "list subvolumes") {
		t.Fatalf("SetupBtrfsSubvolumes() error = %v, want list failure", err)
	}
}

// TestSetupBtrfsSubvolumes_DefaultNotFound verifies a list output without the
// subvolume the system is installed into aborts rather than setting an arbitrary
// default.
func TestSetupBtrfsSubvolumes_DefaultNotFound(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"btrfs", "subvolume", "list"}, "ID 300 gen 1 top level 5 path @other\n")
	defer primitives.SetCommandRunner(rec)()
	defer withMemFS()()

	err := SetupBtrfsSubvolumes(&shared.InstallContext{}, "/dev/vda2", shared.GetBtrfsSubvolumes())
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("SetupBtrfsSubvolumes() error = %v, want not-found error", err)
	}
}

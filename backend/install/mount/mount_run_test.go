package mount

import (
	"errors"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// findmntRule is a convenience: register a findmnt result for a mount point.
func findmntRule(rec *primitivestest.Recorder, mountpoint, source string) *primitivestest.Recorder {
	return rec.OnArgsOutput([]string{"findmnt", "-n", "-o", "SOURCE", mountpoint}, source+"\n")
}

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

// TestRun_Ext4 verifies the standard ext4 flow: mount root, verify it, mount the
// ESP at /mnt/efi, then bind this install's boot directory at /mnt/boot.
func TestRun_Ext4(t *testing.T) {
	rec := findmntRule(findmntRule(primitivestest.NewRecorder(),
		shared.MountRoot, "/dev/vda2"),
		shared.ESPMountPoint, "/dev/vda1")
	defer primitives.SetCommandRunner(rec)()

	if err := Run(baseCtx()); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	mounts := rec.CallsFor("mount")
	if !hasCall(mounts, "mount", "--mkdir", "/dev/vda2", shared.MountRoot) {
		t.Errorf("root mount not issued; mount calls = %v", mounts)
	}
	if !hasCall(mounts, "mount", "-t", shared.FileSystemVfat, "/dev/vda1", shared.ESPMountPoint) {
		t.Errorf("EFI mount not issued; mount calls = %v", mounts)
	}
	bootDir := shared.ESPMountPoint + "/" + shared.InstallBootDir(shared.InstallID(baseCtx().Partitions.RootUUID))
	if !hasCall(mounts, "mount", "--bind", bootDir, shared.BootMountPoint) {
		t.Errorf("per-install boot bind mount not issued; mount calls = %v", mounts)
	}
	if !hasCall(rec.CallsFor("mkdir"), "mkdir", "-p", shared.ESPMountPoint) {
		t.Errorf("ESP mount point was not created: %v", rec.CallsFor("mkdir"))
	}
	if !hasCall(rec.CallsFor("mkdir"), "mkdir", "-p", bootDir) {
		t.Errorf("per-install boot directory was not created: %v", rec.CallsFor("mkdir"))
	}
	if !hasCall(rec.CallsFor("mkdir"), "mkdir", "-p", shared.BootMountPoint) {
		t.Errorf("boot mount point was not created: %v", rec.CallsFor("mkdir"))
	}
}

// TestRun_BtrfsSubvolumes verifies the root subvolume is mounted with its
// options and each additional subvolume is mounted under /mnt.
func TestRun_BtrfsSubvolumes(t *testing.T) {
	ctx := baseCtx()
	ctx.FileSystem = shared.FileSystemBtrfs
	ctx.Partitions.MountPoints = map[string]string{
		"/":     "/dev/vda2",
		"/home": "", // empty → falls back to the root device
	}

	rec := findmntRule(findmntRule(primitivestest.NewRecorder(),
		shared.MountRoot, "/dev/vda2"),
		shared.ESPMountPoint, "/dev/vda1")
	defer primitives.SetCommandRunner(rec)()

	if err := Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	mounts := rec.CallsFor("mount")
	if !hasCall(mounts, "mount", "--mkdir", "-o", "subvol=@,compress=zstd", "/dev/vda2", shared.MountRoot) {
		t.Errorf("root subvolume mount not issued; mount calls = %v", mounts)
	}
	if !hasCall(mounts, "mount", "-o", "subvol=@home,compress=zstd", "/dev/vda2", shared.MountRoot+"/home") {
		t.Errorf("@home subvolume mount not issued; mount calls = %v", mounts)
	}
}

// TestRun_LuksAlreadyOpen verifies a live mapper is not reopened, and the mapper
// is used as the root device.
func TestRun_LuksAlreadyOpen(t *testing.T) {
	ctx := baseCtx()
	ctx.Luks = true
	ctx.Partitions.RootMapper = shared.LuksMapperPath
	ctx.LuksPassphrase = []byte("pw")

	rec := findmntRule(findmntRule(primitivestest.NewRecorder(),
		shared.MountRoot, shared.LuksMapperPath),
		shared.ESPMountPoint, "/dev/vda1")
	rec.OnOutput("cryptsetup", "") // status succeeds → mapper already open
	defer primitives.SetCommandRunner(rec)()

	if err := Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	for _, argv := range rec.CallsFor("cryptsetup") {
		if len(argv) > 1 && argv[1] == "open" {
			t.Errorf("mapper was reopened even though it is live: %v", argv)
		}
	}
	if !hasCall(rec.CallsFor("mount"), "mount", "--mkdir", shared.LuksMapperPath, shared.MountRoot) {
		t.Errorf("root mapper mount not issued: %v", rec.CallsFor("mount"))
	}
}

// TestRun_LuksOpensMapper verifies a closed mapper is reopened with the
// passphrase on stdin before mounting.
func TestRun_LuksOpensMapper(t *testing.T) {
	ctx := baseCtx()
	ctx.Luks = true
	ctx.Partitions.RootMapper = shared.LuksMapperPath
	ctx.LuksPassphrase = []byte("secret")

	rec := findmntRule(findmntRule(primitivestest.NewRecorder(),
		shared.MountRoot, shared.LuksMapperPath),
		shared.ESPMountPoint, "/dev/vda1")
	rec.OnArgsError([]string{"cryptsetup", "status"}, errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	if err := Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	opened := false
	for _, c := range rec.Calls() {
		if len(c.Args) > 1 && c.Args[0] == "cryptsetup" && c.Args[1] == "open" {
			opened = true
			if c.Stdin != "secret" {
				t.Errorf("luks open stdin = %q, want the passphrase", c.Stdin)
			}
		}
	}
	if !opened {
		t.Error("closed mapper was not reopened")
	}
}

// TestRun_RootMountFails verifies a failed root mount aborts the step.
func TestRun_RootMountFails(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgs([]string{"mount", "--mkdir"}, primitivestest.Result{
		Output: []byte("permission denied"),
		Err:    errors.New("exit status 32"),
	})
	defer primitives.SetCommandRunner(rec)()

	err := Run(baseCtx())
	if err == nil {
		t.Fatal("Run() = nil, want root mount failure")
	}
}

// TestRun_VerifyRootMismatch verifies findmnt reporting the wrong device fails
// verification, catching a silently wrong mount.
func TestRun_VerifyRootMismatch(t *testing.T) {
	rec := findmntRule(primitivestest.NewRecorder(), shared.MountRoot, "/dev/sdz9")
	defer primitives.SetCommandRunner(rec)()

	err := Run(baseCtx())
	if err == nil {
		t.Fatal("Run() = nil, want verification mismatch")
	}
	if want := "expected /dev/vda2"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to mention %q", err, want)
	}
}

// TestRun_EfiMountFails verifies a failed ESP mount is surfaced.
func TestRun_EfiMountFails(t *testing.T) {
	rec := findmntRule(primitivestest.NewRecorder(), shared.MountRoot, "/dev/vda2")
	rec.OnArgs([]string{"mount", "-t", shared.FileSystemVfat, "/dev/vda1"}, primitivestest.Result{
		Output: []byte("wrong fs type"),
		Err:    errors.New("exit status 32"),
	})
	defer primitives.SetCommandRunner(rec)()

	err := Run(baseCtx())
	if err == nil || !strings.Contains(err.Error(), "mount EFI failed") {
		t.Fatalf("Run() error = %v, want EFI mount failure", err)
	}
}

// TestRun_BtrfsSubvolumeMountFails verifies an additional subvolume mount
// failure is reported with the subvolume name.
func TestRun_BtrfsSubvolumeMountFails(t *testing.T) {
	ctx := baseCtx()
	ctx.FileSystem = shared.FileSystemBtrfs
	ctx.Partitions.MountPoints = map[string]string{"/": "/dev/vda2", "/home": ""}

	rec := findmntRule(primitivestest.NewRecorder(), shared.MountRoot, "/dev/vda2")
	rec.OnArgs([]string{"mount", "-o"}, primitivestest.Result{
		Err: errors.New("exit status 32"),
	})
	defer primitives.SetCommandRunner(rec)()

	err := Run(ctx)
	if err == nil || !strings.Contains(err.Error(), "subvolume") {
		t.Fatalf("Run() error = %v, want subvolume mount failure", err)
	}
}

func TestRun_RequiresRootUUID(t *testing.T) {
	ctx := baseCtx()
	ctx.Partitions.RootUUID = ""
	if err := Run(ctx); err == nil || !strings.Contains(err.Error(), "root UUID not set") {
		t.Fatalf("Run() error = %v, want a missing-root-UUID error", err)
	}
}

func TestVerifyBindMount_Mismatch(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"stat", "-c", "%d:%i", "/src"}, "8:1\n").
		OnArgsOutput([]string{"stat", "-c", "%d:%i", "/dst"}, "8:2\n")
	defer primitives.SetCommandRunner(rec)()

	err := verifyBindMount("/src", "/dst")
	if err == nil || !strings.Contains(err.Error(), "not a bind mount") {
		t.Fatalf("verifyBindMount() error = %v, want a bind-mount mismatch", err)
	}
}

// TestRun_BindMountVerificationFails verifies a silently failed bind mount is
// caught before pacstrap writes kernels into an unrelated directory.
func TestRun_BindMountVerificationFails(t *testing.T) {
	rec := findmntRule(findmntRule(primitivestest.NewRecorder(),
		shared.MountRoot, "/dev/vda2"),
		shared.ESPMountPoint, "/dev/vda1")
	rec.OnOutput("stat", "8:1\n")
	rec.OnArgsOutput([]string{"stat", "-c", "%d:%i", shared.BootMountPoint}, "8:2\n")
	defer primitives.SetCommandRunner(rec)()

	err := Run(baseCtx())
	if err == nil || !strings.Contains(err.Error(), "not a bind mount") {
		t.Fatalf("Run() error = %v, want bind-mount verification to fail", err)
	}
}

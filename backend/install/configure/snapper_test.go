package configure

import (
	"errors"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// snapperContext is a btrfs install context: ConfigureSnapper needs the chosen
// filesystem and nothing else.
func snapperContext() *shared.InstallContext {
	return &shared.InstallContext{FileSystem: shared.FileSystemBtrfs}
}

// TestSnapperTargetsMatchSubvolumeScheme pins the snapper list to the subvolume
// scheme it snapshots. ConfigureSnapper skips a target whose mount point is not
// in shared.GetBtrfsSubvolumes(), so a drift here would silently install
// snapper with no configs.
func TestSnapperTargetsMatchSubvolumeScheme(t *testing.T) {
	subvolumes := shared.GetBtrfsSubvolumes()
	for _, target := range snapperTargets {
		sv := shared.FindSubvolumeByMountPoint(subvolumes, target.mountPoint)
		if sv == nil {
			t.Errorf("snapper target %s mounts at %q, which is not in the subvolume scheme", target.configName, target.mountPoint)
			continue
		}
		if sv.Name != target.subvolume {
			t.Errorf("snapper target %s names subvolume %q, but %q is mounted at %q",
				target.configName, target.subvolume, sv.Name, target.mountPoint)
		}
	}
}

// snapperCalled reports whether the recorder ran exactly this argv.
func snapperCalled(rec *primitivestest.Recorder, want ...string) bool {
	for _, c := range rec.Calls() {
		if hasCall([][]string{c.Args}, want...) {
			return true
		}
	}
	return false
}

func TestConfigureSnapper_Btrfs(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := ConfigureSnapper(snapperContext()); err != nil {
		t.Fatalf("ConfigureSnapper() = %v, want nil", err)
	}

	for _, target := range snapperTargets {
		content, ok := fs.Read(shared.MountRoot + snapperConfigPath(target.configName))
		if !ok {
			t.Errorf("snapper config %s was not written", target.configName)
			continue
		}
		for _, want := range []string{
			`SUBVOLUME="` + target.mountPoint + `"`,
			`FSTYPE="btrfs"`,
			`TIMELINE_CREATE="yes"`,
			`TIMELINE_CLEANUP="yes"`,
			`NUMBER_CLEANUP="yes"`,
			`NUMBER_LIMIT="10"`,
			`TIMELINE_LIMIT_HOURLY="6"`,
			`ALLOW_GROUPS="wheel"`,
			`SPACE_LIMIT="0.5"`,
		} {
			if !strings.Contains(content, want) {
				t.Errorf("config %s is missing %q:\n%s", target.configName, want, content)
			}
		}
	}

	// Snapper refuses to use a .snapshots that is not a subvolume, so the
	// installer creates it inside each snapshotted subvolume.
	for _, want := range []string{
		shared.MountRoot + "/.snapshots",
		shared.MountRoot + "/home/.snapshots",
	} {
		if !snapperCalled(rec, "btrfs", "subvolume", "create", want) {
			t.Errorf("no btrfs subvolume create for %s; calls = %v", want, rec.Calls())
		}
	}

	sysconfig, ok := fs.Read(shared.MountRoot + snapperSysconfigPath)
	if !ok {
		t.Fatal("SNAPPER_CONFIGS file was not written")
	}
	if !strings.Contains(sysconfig, `SNAPPER_CONFIGS="root home"`) {
		t.Errorf("SNAPPER_CONFIGS not written:\n%s", sysconfig)
	}

	if !snapperCalled(rec, "btrfs", "quota", "enable", shared.MountRoot) {
		t.Errorf("btrfs quotas were not enabled; calls = %v", rec.Calls())
	}
	for _, unit := range snapperUnits {
		if !snapperCalled(rec, "arch-chroot", shared.MountRoot, "systemctl", "enable", unit) {
			t.Errorf("unit %s was not enabled; calls = %v", unit, rec.Calls())
		}
	}
}

func TestConfigureSnapper_SkipsNonBtrfs(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	for _, fsType := range []string{shared.FileSystemExt4, ""} {
		if err := ConfigureSnapper(&shared.InstallContext{FileSystem: fsType}); err != nil {
			t.Fatalf("ConfigureSnapper(%q) = %v, want nil", fsType, err)
		}
	}

	if fs.Exists(shared.MountRoot + snapperConfigPath("root")) {
		t.Error("a snapper config was written for a non-btrfs install")
	}
	if fs.Exists(shared.MountRoot + snapperSysconfigPath) {
		t.Error("SNAPPER_CONFIGS was written for a non-btrfs install")
	}
	for _, c := range rec.Calls() {
		if len(c.Args) > 1 && c.Args[0] == "btrfs" {
			t.Errorf("btrfs command run for a non-btrfs install: %v", c.Args)
		}
	}
}

// TestConfigureSnapper_RequiresMountedSystem matches the other configure
// sub-steps: they run against a mounted /mnt and say so when it is not.
func TestConfigureSnapper_RequiresMountedSystem(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("findmnt", errors.New("not mounted"))
	defer primitives.SetCommandRunner(rec)()

	if err := ConfigureSnapper(snapperContext()); err == nil {
		t.Fatal("ConfigureSnapper() = nil, want the not-mounted error")
	}
}

// TestConfigureSnapper_SkipsConfigWithoutSnapshotDir checks that a target whose
// .snapshots subvolume cannot be created gets no config file at all. A snapper
// config with no .snapshots makes every snapper command for it fail, which is
// worse than not configuring it, and it must not take the whole install down.
func TestConfigureSnapper_SkipsConfigWithoutSnapshotDir(t *testing.T) {
	rec := mountedRunner().OnArgsError(
		[]string{"btrfs", "subvolume", "create", shared.MountRoot + "/home/.snapshots"},
		errors.New("read-only filesystem"))
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := ConfigureSnapper(snapperContext()); err != nil {
		t.Fatalf("ConfigureSnapper() = %v, want nil", err)
	}

	if !fs.Exists(shared.MountRoot + snapperConfigPath("root")) {
		t.Error("the root config was skipped although its .snapshots was created")
	}
	if fs.Exists(shared.MountRoot + snapperConfigPath("home")) {
		t.Error("the home config was written although its .snapshots failed")
	}
	sysconfig, ok := fs.Read(shared.MountRoot + snapperSysconfigPath)
	if !ok {
		t.Fatal("SNAPPER_CONFIGS file was not written")
	}
	if !strings.Contains(sysconfig, `SNAPPER_CONFIGS="root"`) {
		t.Errorf("SNAPPER_CONFIGS should list only the configs that exist:\n%s", sysconfig)
	}
}

// TestSnapperSysconfigContent pins the file format snapper parses.
func TestSnapperSysconfigContent(t *testing.T) {
	got := snapperSysconfigContent([]string{"root", "home"})
	if !strings.Contains(got, `SNAPPER_CONFIGS="root home"`) {
		t.Errorf("unexpected content:\n%s", got)
	}
	if strings.Contains(got, "SNAPPER_CONFIGS=\"\"") {
		t.Errorf("empty SNAPPER_CONFIGS:\n%s", got)
	}
}

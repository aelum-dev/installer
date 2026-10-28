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
	return snapperCallIndex(rec, want...) >= 0
}

// snapperCallIndex is snapperCalled with the position of the call, which is what
// a test needs when the order of two commands is the behaviour being pinned.
func snapperCallIndex(rec *primitivestest.Recorder, want ...string) int {
	for i, c := range rec.Calls() {
		if hasCall([][]string{c.Args}, want...) {
			return i
		}
	}
	return -1
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

	// The history of the subvolume that holds the system is part of the layout (see
	// format.root_snapshot) and is mounted at /.snapshots: the live root is itself a
	// snapshot, and a snapshot cannot contain a subvolume, so this step must not try
	// to create one inside it. The @home history is created here, inside its own
	// subvolume.
	if !snapperCalled(rec, "findmnt", "-n", "-o", "SOURCE", shared.MountRoot+shared.BtrfsSnapshotsMountPoint) {
		t.Errorf("the history of the root subvolume was not checked at %s; calls = %v",
			shared.MountRoot+shared.BtrfsSnapshotsMountPoint, rec.Calls())
	}
	if snapperCalled(rec, "btrfs", "subvolume", "create", shared.MountRoot+"/"+shared.BtrfsSnapshotsDirName) {
		t.Error("the root history was created inside the live root, where it would end up inside its own snapshots")
	}
	if !snapperCalled(rec, "btrfs", "subvolume", "create", shared.MountRoot+"/home/"+shared.BtrfsSnapshotsDirName) {
		t.Errorf("no btrfs subvolume create for %s; calls = %v", shared.MountRoot+"/home/.snapshots", rec.Calls())
	}

	// The state this installer produced is the one a restore goes back to, so it is
	// snapshotted at the end of the install and marked so cleanup never removes it.
	snapshot := snapperCallIndex(rec, "arch-chroot", shared.MountRoot, "snapper", "--no-dbus", "-c", snapperRootConfig,
		"create", "-t", "single", "-d", "after installation", "-c", "number", "-u", "important=yes")
	if snapshot < 0 {
		t.Errorf("no \"after installation\" snapshot; calls = %v", rec.Calls())
	}

	// The menu is written by the script the install just installed - not by a second
	// copy of its logic here - and after the snapshot it lists exists. rEFInd reads
	// its configuration before any part of the installed system runs, so this call is
	// the install's only chance to have a snapshot entry on its own first boot.
	menu := snapperCallIndex(rec, shared.MountRoot+snapshotScriptPath, "--root", shared.MountRoot)
	switch {
	case menu < 0:
		t.Errorf("the snapshot menu was not written for the first boot; calls = %v", rec.Calls())
	case snapshot >= 0 && menu < snapshot:
		t.Error("the snapshot menu was written before the snapshot it lists exists")
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

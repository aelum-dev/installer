package configure

import (
	"errors"
	"strconv"
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

// snapperConfigValue reads one KEY="value" line out of a rendered config, which is
// how a test asserts a value without restating it.
func snapperConfigValue(t *testing.T, content, key string) string {
	t.Helper()
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+"=") {
			return strings.Trim(strings.TrimPrefix(line, key+"="), "\"")
		}
	}
	t.Errorf("config has no %s line", key)
	return ""
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
			`QGROUP="` + target.qgroup + `"`,
			`SPACE_LIMIT="0.5"`,
			`FREE_LIMIT="0.2"`,
			`ALLOW_GROUPS="wheel"`,
			`NUMBER_CLEANUP="yes"`,
			`TIMELINE_CLEANUP="yes"`,
			// The retention the config is meant to have, read from the policy rather
			// than repeated here: a test that copies the numbers only proves the
			// numbers were typed twice.
			`TIMELINE_CREATE="` + target.retention.timelineCreate + `"`,
			`NUMBER_LIMIT="` + target.retention.numberLimit + `"`,
			`NUMBER_LIMIT_IMPORTANT="` + target.retention.numberLimitImportant + `"`,
			`TIMELINE_LIMIT_HOURLY="` + target.retention.hourly + `"`,
			`TIMELINE_LIMIT_DAILY="` + target.retention.daily + `"`,
			`TIMELINE_LIMIT_WEEKLY="` + target.retention.weekly + `"`,
			`TIMELINE_LIMIT_MONTHLY="` + target.retention.monthly + `"`,
			`TIMELINE_LIMIT_YEARLY="` + target.retention.yearly + `"`,
		} {
			if !strings.Contains(content, want) {
				t.Errorf("config %s is missing %q:\n%s", target.configName, want, content)
			}
		}
		// Every limit is a range. snapper skips its space-aware cleanup when the
		// limits are degenerated (min == max), so a single number here silently
		// switches SPACE_LIMIT / FREE_LIMIT off - the exact bug this pins.
		for _, key := range []string{
			"NUMBER_LIMIT", "NUMBER_LIMIT_IMPORTANT",
			"TIMELINE_LIMIT_HOURLY", "TIMELINE_LIMIT_DAILY", "TIMELINE_LIMIT_WEEKLY",
			"TIMELINE_LIMIT_MONTHLY", "TIMELINE_LIMIT_YEARLY",
		} {
			value := snapperConfigValue(t, content, key)
			if !strings.Contains(value, "-") {
				t.Errorf("config %s has a degenerated %s=%q; snapper would ignore the space limits",
					target.configName, key, value)
			}
		}
		// QGROUP is what snapper measures the snapshots against. Without it
		// prepare_quota throws "qgroup not set" and no space limit is ever applied.
		if snapperConfigValue(t, content, "QGROUP") != target.qgroup {
			t.Errorf("config %s names qgroup %q, want %q",
				target.configName, snapperConfigValue(t, content, "QGROUP"), target.qgroup)
		}
		if !snapperCalled(rec, "btrfs", "qgroup", "create", target.qgroup, shared.MountRoot) {
			t.Errorf("qgroup %s was not created for config %s; calls = %v",
				target.qgroup, target.configName, rec.Calls())
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

// TestConfigureSnapper_QuotaWithoutQGroup keeps the degradation honest: if the
// aggregate group cannot be created, the config is still written and QGROUP stays
// empty - snapper's own spelling for "no space accounting". A config naming a
// group that does not exist makes every cleanup say "quota not working" for no
// gain, and the count limits still bound the history either way.
func TestConfigureSnapper_QuotaWithoutQGroup(t *testing.T) {
	rec := mountedRunner().OnArgsError(
		[]string{"btrfs", "qgroup", "create", snapperRootQGroup, shared.MountRoot},
		errors.New("quota not supported on this filesystem"))
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := ConfigureSnapper(snapperContext()); err != nil {
		t.Fatalf("ConfigureSnapper() = %v, want nil", err)
	}

	content, ok := fs.Read(shared.MountRoot + snapperConfigPath(snapperRootConfig))
	if !ok {
		t.Fatal("the root config was not written")
	}
	if got := snapperConfigValue(t, content, "QGROUP"); got != "" {
		t.Errorf("QGROUP=%q for a group that could not be created", got)
	}
	if got := snapperConfigValue(t, content, "NUMBER_LIMIT"); !strings.Contains(got, "-") {
		t.Errorf("count limits lost when the qgroup failed: %q", got)
	}
}

// TestSnapperConfigQGroupIsPerConfig pins the reason the two configs do not share
// one group: SPACE_LIMIT is a property of a config, so a shared aggregate would
// let @home snapshots delete the system's restore points.
func TestSnapperConfigQGroupIsPerConfig(t *testing.T) {
	seen := map[string]bool{}
	for _, target := range snapperTargets {
		if target.qgroup == "" {
			t.Errorf("snapper config %s has no qgroup", target.configName)
		}
		if seen[target.qgroup] {
			t.Errorf("snapper configs share qgroup %s", target.qgroup)
		}
		seen[target.qgroup] = true
	}
}

// TestSnapperRetentionIsBounded pins the decisions the retention encodes: every
// limit is a range (snapper only looks at space for ranges), the important quota
// is not narrower than the ordinary one, the root has no timeline - the state
// openSUSE's own default setup leaves it in - and @home, which shares the
// filesystem with @, keeps the smaller history.
func TestSnapperRetentionIsBounded(t *testing.T) {
	for _, target := range snapperTargets {
		if target.retention.timelineCreate != "no" {
			t.Errorf("config %s takes timeline snapshots (%q); the history of a root is transactions, not a timer",
				target.configName, target.retention.timelineCreate)
		}
		for _, value := range []string{target.retention.numberLimit, target.retention.numberLimitImportant} {
			min, max := parseLimitRange(t, value)
			if max < 1 || min > max {
				t.Errorf("config %s has an unusable limit %q", target.configName, value)
			}
		}
		if rangeMax(t, target.retention.numberLimitImportant) < rangeMax(t, target.retention.numberLimit) {
			t.Errorf("config %s keeps fewer important snapshots (%q) than ordinary ones (%q)",
				target.configName, target.retention.numberLimitImportant, target.retention.numberLimit)
		}
	}

	home := snapperTargets[len(snapperTargets)-1]
	if home.configName != "home" {
		t.Fatalf("expected the home config last, got %q", home.configName)
	}
	if rangeMax(t, home.retention.numberLimit) >= rangeMax(t, snapperTargets[0].retention.numberLimit) {
		t.Errorf("home retention %q is not smaller than the root's %q",
			home.retention.numberLimit, snapperTargets[0].retention.numberLimit)
	}
}

// parseLimitRange reads snapper's "min-max" (or a bare number) as the two values
// snapper's own Range::parse takes from it.
func parseLimitRange(t *testing.T, value string) (int, int) {
	t.Helper()
	minText, maxText := value, value
	if i := strings.Index(value, "-"); i >= 0 {
		minText, maxText = value[:i], value[i+1:]
	}
	min, err := strconv.Atoi(minText)
	if err != nil {
		t.Errorf("limit %q is not a number or a range: %v", value, err)
		return 0, 0
	}
	max, err := strconv.Atoi(maxText)
	if err != nil {
		t.Errorf("limit %q has no usable max: %v", value, err)
		return min, 0
	}
	return min, max
}

func rangeMax(t *testing.T, value string) int {
	t.Helper()
	_, max := parseLimitRange(t, value)
	return max
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

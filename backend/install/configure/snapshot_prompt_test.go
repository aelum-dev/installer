package configure

import (
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// TestSnapshotPromptUnit is the gate, spelled once: the unit runs only when the
// marker exists - which is only in a snapshot boot - and the marker is also how
// the dialog learns the snapshot number. Everything else in the unit is the shape
// the seeded session units already have (PartOf/Requisite the session target).
func TestSnapshotPromptUnit(t *testing.T) {
	unit := snapshotPromptUnit
	for _, want := range []string{
		"ConditionPathExists=" + snapshotBootMarkerPath,
		"EnvironmentFile=" + snapshotBootMarkerPath,
		// Its own Quickshell instance, and only our config: -c selects the named
		// config so a user's own ~/.config/quickshell config is not pulled in too.
		"ExecStart=/usr/bin/quickshell -c aelum-snapshot",
		"Environment=QS_INSTANCE_ID=aelum-snapshot",
		"PartOf=graphical-session.target",
		"After=graphical-session.target",
		"Requisite=graphical-session.target",
		"WantedBy=hyprland-session.target",
	} {
		if !strings.Contains(unit, want) {
			t.Errorf("prompt unit is missing %q:\n%s", want, unit)
		}
	}
	if !strings.Contains(unit, "[Install]") {
		t.Error("prompt unit has no [Install] section; the wants link names it")
	}
}

// TestSnapshotPromptIsSystemSide is the property the per-home version did not
// have: the prompt belongs to the system, not to the account the installer
// happened to create. Every path it writes is under /etc, nothing is written
// into a home, and the unit and its pull-in live where the user manager loads
// units for every login (/etc/systemd/user) and Quickshell finds packaged
// configs (/etc/xdg/quickshell). A user created later - or recreated after
// deletion - gets the prompt with no extra work.
func TestSnapshotPromptIsSystemSide(t *testing.T) {
	for _, dir := range []string{snapshotPromptConfigDir, snapshotPromptUnitDir, snapshotPromptWantsDir} {
		if !strings.HasPrefix(dir, "/etc/") {
			t.Errorf("prompt path %q is not system-side", dir)
		}
	}
	if strings.Contains(snapshotPromptConfigDir, "$HOME") || strings.Contains(snapshotPromptUnit, "%h") {
		t.Error("the prompt references a home directory; it must not belong to one user")
	}

	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := configureSnapshotPrompt(snapperContext()); err != nil {
		t.Fatalf("configureSnapshotPrompt() = %v, want nil", err)
	}
	for _, p := range fs.Paths() {
		if strings.HasPrefix(p, shared.MountRoot+"/home/") {
			t.Errorf("%s was written into a home; the prompt is system-side", p)
		}
	}
}

// TestSnapshotPromptQML pins the dialog against the two things it must not become:
// a dependency on anything Quickshell does not ship, and a question that can run
// the restore without the narrow sudoers path.
func TestSnapshotPromptQML(t *testing.T) {
	qml := snapshotPromptQML
	for _, want := range []string{
		"import Quickshell",
		"import Quickshell.Io",
		"import QtQuick",
		// The dialog shape: a layer surface the size of the card, centered by
		// margins. A full-screen anchored surface would swallow every click in
		// the session (Wayland input follows geometry, not opacity), so the
		// margins that shrink it to the card are part of the contract.
		"PanelWindow",
		"exclusionMode: ExclusionMode.Ignore",
		`color: "transparent"`,
		"anchors { top: true; bottom: true; left: true; right: true }",
		"margins {",
		"screen.width",
		"Math.max(0",
		// The height follows the content: the Column reports its own height and
		// the surface margins are computed from it, so no phase reserves dead
		// space under its buttons.
		"id: content",
		"property int cardHeight: content.height",
		// The number arrives through the unit's EnvironmentFile.
		`Quickshell.env("SNAPSHOT")`,
		// OK runs exactly the helper the sudoers rule allows, with -n so a missing
		// rule fails visibly instead of prompting for a password nobody typed.
		`command: ["sudo", "-n", "` + snapshotRestoreScriptPath + `"]`,
		// The reboot is detached (closing the dialog would kill a tracked child
		// first) and reboot-only (the OK call already rolled back).
		`Quickshell.execDetached(["sudo", "-n", "` + snapshotRestoreScriptPath + `", "--reboot-only"])`,
		// CANCEL is a first-class outcome: a snapshot boot is non-destructive and
		// plenty of people boot one just to look.
		`text: "CANCEL"`,
		"Qt.quit()",
		// The colors are the user's live DMS scheme: the file DMS writes when
		// matugen recolors, watched so the dialog follows it, with the mode read
		// from the same file. The fallback is the brand scheme - the exact values
		// the installer seeds - not invented colors.
		"dms-colors.json",
		"watchChanges: true",
		"onFileChanged: this.reload()",
		"JSON.parse",
		`dms.mode === "light"`,
		`color("surface_container")`,
		`color("primary")`,
		`color("on_primary")`,
		`color("outline_variant")`,
		"#29272d", // the seeded brand dark surface_container, as the fallback
	} {
		if !strings.Contains(qml, want) {
			t.Errorf("prompt config is missing %q", want)
		}
	}
	// QtQuick.Controls is a separate QML module; depending on it would make the
	// dialog depend on a package the install does not name. The card is plain
	// QtQuick items for exactly that reason.
	if strings.Contains(qml, "import QtQuick.Controls") {
		t.Error("prompt config imports QtQuick.Controls, which is not a Quickshell dependency")
	}
	// No regression to a fixed palette: the colors the dialog hardcoded before it
	// read the live scheme must be gone from the QML entirely.
	for _, gone := range []string{"#1e1e2e", "#89b4fa", "#cdd6f4", "#313244", "#f38ba8"} {
		if strings.Contains(qml, gone) {
			t.Errorf("prompt config hardcodes %q; colors must come from the live DMS scheme", gone)
		}
	}
}

// TestConfigureSnapshotPrompt writes the config, the unit, and the session
// pull-in into the installed system's /etc. It needs no username: the prompt is
// the same for every account, which is the point.
func TestConfigureSnapshotPrompt(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := configureSnapshotPrompt(snapperContext()); err != nil {
		t.Fatalf("configureSnapshotPrompt() = %v, want nil", err)
	}

	qmlPath := shared.MountRoot + snapshotPromptConfigDir + "/" + snapshotPromptConfigName
	content, ok := fs.Read(qmlPath)
	if !ok {
		t.Fatalf("%s was not written (paths: %v)", qmlPath, fs.Paths())
	}
	if !strings.Contains(content, "PanelWindow") {
		t.Errorf("%s is not the dialog:\n%s", qmlPath, content)
	}

	unitPath := shared.MountRoot + snapshotPromptUnitDir + "/" + snapshotPromptUnitName
	if !fs.Exists(unitPath) {
		t.Fatalf("%s was not written", unitPath)
	}

	// The pull-in. A target's .wants/ directory is collected from every load
	// path, so this system-side link reaches the per-user session target; the
	// link and its target are both chroot paths, resolved inside the install.
	if snapperCallIndex(rec, "arch-chroot", shared.MountRoot, "mkdir", "-p", snapshotPromptWantsDir) < 0 {
		t.Errorf("the wants directory was not created; calls = %v", rec.Calls())
	}
	if snapperCallIndex(rec, "arch-chroot", shared.MountRoot, "ln", "-sf",
		snapshotPromptUnitDir+"/"+snapshotPromptUnitName,
		snapshotPromptWantsDir+"/"+snapshotPromptUnitName) < 0 {
		t.Errorf("the prompt was not linked into hyprland-session.target.wants; calls = %v", rec.Calls())
	}
}

// TestConfigureSnapper_InstallsPrompt: the prompt belongs to the snapshot
// feature, so it arrives with it - before the factory snapshot, because a
// snapshot runs the units and configs it contains - and an ext4 install, which
// has no snapshot, gets none of it.
func TestConfigureSnapper_InstallsPrompt(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := ConfigureSnapper(snapperContext()); err != nil {
		t.Fatalf("ConfigureSnapper() = %v, want nil", err)
	}
	qmlPath := shared.MountRoot + snapshotPromptConfigDir + "/" + snapshotPromptConfigName
	unitPath := shared.MountRoot + snapshotPromptUnitDir + "/" + snapshotPromptUnitName
	if !fs.Exists(qmlPath) {
		t.Errorf("%s was not installed by the snapper step", qmlPath)
	}
	if !fs.Exists(unitPath) {
		t.Errorf("%s was not installed by the snapper step", unitPath)
	}

	snapshot := snapperCallIndex(rec, "arch-chroot", shared.MountRoot, "snapper", "--no-dbus", "-c", snapperRootConfig,
		"create", "-t", "single", "-d", "after installation", "-c", "number", "-u", "important=yes")
	if snapshot < 0 {
		t.Fatalf("no \"after installation\" snapshot; calls = %v", rec.Calls())
	}
	written := snapperCallIndex(rec, "arch-chroot", shared.MountRoot, "ln", "-sf",
		snapshotPromptUnitDir+"/"+snapshotPromptUnitName,
		snapshotPromptWantsDir+"/"+snapshotPromptUnitName)
	if written < 0 {
		t.Fatalf("the prompt was not linked; calls = %v", rec.Calls())
	}
	if written > snapshot {
		t.Error("the prompt was installed after the snapshot was taken, so it is not inside it")
	}

	fs2 := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs2)()
	if err := ConfigureSnapper(&shared.InstallContext{FileSystem: shared.FileSystemExt4}); err != nil {
		t.Fatalf("ConfigureSnapper(ext4) = %v, want nil", err)
	}
	if fs2.Exists(qmlPath) || fs2.Exists(unitPath) {
		t.Error("the snapshot prompt was installed for an ext4 install")
	}
}

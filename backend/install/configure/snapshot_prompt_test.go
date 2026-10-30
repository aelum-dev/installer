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

// TestSnapshotPromptIsSystemSide is the property the per-home version of the
// prompt did not have: the two files the installer writes belong to the system,
// not to the account the installer happened to create. Every path it writes is
// under /etc, nothing is written into a home, and the unit and its config live
// where the user manager loads units for every login (/etc/systemd/user) and
// Quickshell finds packaged configs (/etc/xdg/quickshell). A user created later
// - or recreated after deletion - gets the prompt with no extra work. The
// session pull-in is the deliberate exception: it is per-home by design, seeded
// into /etc/skel by live-iso, because a system-side pull-in would ask the
// greeter too - and the greeter cannot run the restore and did not choose a
// snapshot. The installer must not write one.
func TestSnapshotPromptIsSystemSide(t *testing.T) {
	for _, dir := range []string{snapshotPromptConfigDir, snapshotPromptUnitDir} {
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
// the restore without the polkit action it names.
func TestSnapshotPromptQML(t *testing.T) {
	qml := snapshotPromptQML
	for _, want := range []string{
		"import Quickshell",
		"import Quickshell.Io",
		"import QtQuick",
		// The dialog is a real window, not a layer surface: a layer surface is
		// fixed to the screen by its anchors and the compositor cannot move
		// one, which is what made the dialog impossible to drag. The title is
		// the handle the seeded Hyprland window rule matches (float + center),
		// so it is part of the contract too.
		"FloatingWindow",
		`title: "aelum-snapshot-prompt"`,
		// Pressing the card body asks the compositor to move the window
		// (xdg_toplevel's start_move, honored by Hyprland 0.56+): dragging the
		// dialog needs no compositor-side mouse bind.
		"startSystemMove",
		`color: "transparent"`,
		// The window is the card: the width is fixed so the text does not
		// reflow while it is read. The text is anchored to the card's top and
		// the buttons are a footer anchored to its bottom-right corner - the
		// dialog convention, primary action rightmost. The body reserves the
		// ask phase's height, measured from the ask copy itself laid out
		// hidden, so the card keeps one size through the flow and pressing OK
		// swaps the buttons in place.
		"id: content",
		"implicitWidth: cardWidth",
		"implicitHeight: content.height",
		"id: buttons",
		"anchors.bottom: parent.bottom",
		"anchors.right: parent.right",
		"id: askBody",
		"height: Math.max(implicitHeight, askBody.height)",
		// The number arrives through the unit's EnvironmentFile.
		`Quickshell.env("SNAPSHOT")`,
		// OK runs exactly the helper the polkit action names, through pkexec -
		// never a shell, never sudo. The action is resolved from the policy's
		// exec.path annotation, so the QML only ever spells the program.
		`command: ["pkexec", "` + snapshotRestoreScriptPath + `"]`,
		// The reboot is detached (closing the dialog would kill a tracked child
		// first) and reboot-only (the OK call already rolled back).
		`Quickshell.execDetached(["pkexec", "` + snapshotRestoreScriptPath + `", "--reboot-only"])`,
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
	// The footer's declaration order is its left-to-right order: CANCEL before
	// OK, CLOSE before REBOOT NOW - the primary action rightmost in both
	// phases, which is the corner a hand reaches for by default.
	for _, pair := range [][2]string{{`text: "CANCEL"`, `text: "OK"`}, {`text: "CLOSE"`, `text: "REBOOT NOW"`}} {
		first, second := strings.Index(qml, pair[0]), strings.Index(qml, pair[1])
		if first < 0 || second < 0 || first > second {
			t.Errorf("buttons are not primary-rightmost: %q at %d, %q at %d", pair[0], first, pair[1], second)
		}
	}

	// QtQuick.Controls is a separate QML module; depending on it would make the
	// dialog depend on a package the install does not name. The card is plain
	// QtQuick items for exactly that reason.
	if strings.Contains(qml, "import QtQuick.Controls") {
		t.Error("prompt config imports QtQuick.Controls, which is not a Quickshell dependency")
	}
	// A layer surface again would undo the drag fix: Hyprland cannot move a
	// surface that is anchored to the screen edges.
	if strings.Contains(qml, "PanelWindow") {
		t.Error("prompt config is a PanelWindow again; a layer surface cannot be dragged")
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
	if !strings.Contains(content, "FloatingWindow") {
		t.Errorf("%s is not the dialog:\n%s", qmlPath, content)
	}

	unitPath := shared.MountRoot + snapshotPromptUnitDir + "/" + snapshotPromptUnitName
	if !fs.Exists(unitPath) {
		t.Fatalf("%s was not written", unitPath)
	}

	// The pull-in is not written here. It is the wants link seeded into
	// /etc/skel by live-iso and copied into every real home with the rest of
	// the session config; a system-side link would pull the prompt into the
	// greeter's session too, and the greeter must not be asked.
	for _, call := range rec.Calls() {
		for _, arg := range call.Args {
			if strings.Contains(arg, "hyprland-session.target.wants") {
				t.Errorf("the installer wrote a system-side wants link (%v); the pull-in belongs to /etc/skel", call.Args)
			}
		}
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
	// That the prompt is inside the snapshot (written before it) is a property
	// of the call order in configureSnapper; verify-installed.sh checks it for
	// real by reading the factory snapshot's own contents.

	fs2 := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs2)()
	if err := ConfigureSnapper(&shared.InstallContext{FileSystem: shared.FileSystemExt4}); err != nil {
		t.Fatalf("ConfigureSnapper(ext4) = %v, want nil", err)
	}
	if fs2.Exists(qmlPath) || fs2.Exists(unitPath) {
		t.Error("the snapshot prompt was installed for an ext4 install")
	}
}

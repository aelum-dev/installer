package configure

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"aelum-installer/backend/shared"
)

// The restore path for a booted snapshot is source in the live ISO: the helper, its
// polkit action and grant, the dialog, its translated copy, and the unit that runs
// it. The installer copies them and never rewrites them, so what these tests can
// hold is the shipped files themselves - the same text the installed system ends up
// with, checked against the constants the rest of the feature matches on.
//
// Reading another repository is why they skip rather than fail when that repository
// is not beside this one: the four repos of this product sit side by side in one
// workspace, but a standalone clone of the installer has nothing to read. The real
// disk check for the same properties is docker/scripts/verify-installed.sh.

const (
	// liveISODir is the sibling live-iso tree; AELUM_LIVE_ISO_DIR overrides it for
	// a layout that is not the default one, as live-iso/scripts/gen-refind-theme.py
	// does for the shared font.
	liveISODir    = "../../../../live-iso"
	liveISODirEnv = "AELUM_LIVE_ISO_DIR"

	// promptCopyPrefix is the namespace of the dialog's strings in the installer's
	// own UI translations, and the namespace gen-snapshot-prompt-i18n.go ships.
	promptCopyPrefix = "snapshot.prompt."

	// promptConfigName is the file Quickshell loads for a named config: a config
	// directory is only a config because this file is in it.
	promptConfigName = "shell.qml"
)

// promptCopyKeys are the strings the dialog reads. Every shipped locale must carry
// all of them: a dialog missing a string falls back to English mid-sentence, which
// is a worse outcome than a dialog that is simply in the wrong language.
var promptCopyKeys = []string{
	"title", "titleUnknown", "question", "body",
	"restoring", "restored", "failed", "failedUnknown",
	"cancel", "ok", "close", "reboot",
}

func liveISOFile(t *testing.T, rel string) string {
	t.Helper()
	dir := os.Getenv(liveISODirEnv)
	if dir == "" {
		dir = liveISODir
	}
	path := filepath.Join(dir, "airootfs", rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("cannot read the shipped %s (%v): this test needs the live-iso tree beside the installer", rel, err)
	}
	return string(data)
}

func requireContains(t *testing.T, text, want, why string) {
	t.Helper()
	if !strings.Contains(text, want) {
		t.Errorf("missing %q (%s)", want, why)
	}
}

// TestShippedSnapshotRestoreHelper is the rollback the OK button runs, as the live
// ISO ships it: the same command the polkit action names, the refusal that keeps it
// from rolling a normal boot back onto itself, and the mode pkexec requires.
func TestShippedSnapshotRestoreHelper(t *testing.T) {
	script := liveISOFile(t, strings.TrimPrefix(shared.SnapshotRestoreScriptPath, "/"))

	for _, want := range []string{
		"#!/bin/sh",
		"snapper --no-dbus -c root rollback",
		// Its own gate: no marker, no rollback. "snapper rollback" with no number
		// means "the state I am booted in", which is only the right answer from
		// inside a snapshot.
		shared.SnapshotBootMarkerPath,
		// Overridable for the same reason the menu generator takes --root: so the
		// script can be run against a state it is not booted in.
		"AELUM_SNAPSHOT_MARKER",
		// The prompt's REBOOT button: reboot, and only reboot - the OK call already
		// rolled back, and a second rollback would take a second undo snapshot.
		"--reboot-only)",
		"exec systemctl reboot",
		// An unknown argument is a usage error, never a silent success.
		"usage: aelum-snapshot-restore",
	} {
		requireContains(t, script, want, "the shipped restore helper")
	}

	// The mode the ISO must carry: pkexec authenticates any program an action execs
	// (root-owned, not group- or world-writable), and mkarchiso only applies modes
	// it is told about. Both halves are checked: the file here, and the entry in
	// profiledef.sh that puts them in the ISO.
	dir := os.Getenv(liveISODirEnv)
	if dir == "" {
		dir = liveISODir
	}
	info, err := os.Stat(filepath.Join(dir, "airootfs", strings.TrimPrefix(shared.SnapshotRestoreScriptPath, "/")))
	if err != nil {
		t.Skipf("cannot stat the shipped helper: %v", err)
	}
	if mode := info.Mode().Perm(); mode&0o111 == 0 {
		t.Errorf("%s is not executable (%v): the prompt runs it, it does not read it", shared.SnapshotRestoreScriptPath, mode)
	} else if mode&0o022 != 0 {
		t.Errorf("%s is group/world-writable (%v): pkexec refuses to exec such a program", shared.SnapshotRestoreScriptPath, mode)
	}

	profile := liveISOFile(t, "../profiledef.sh")
	requireContains(t, profile, `["`+shared.SnapshotRestoreScriptPath+`"]="0:0:755"`,
		"mkarchiso does not carry exec bits, so the mode has to be declared")
}

// TestShippedSnapshotRestorePolkit is the authorization for that helper: the action
// pkexec resolves, which refuses everywhere by default, and the one grant that makes
// the OK button need no asking.
func TestShippedSnapshotRestorePolkit(t *testing.T) {
	policy := liveISOFile(t, strings.TrimPrefix(shared.SnapshotRestorePolicyPath, "/"))
	for _, want := range []string{
		`<action id="` + shared.SnapshotRestorePolkitAction + `">`,
		`<annotate key="org.freedesktop.policykit.exec.path">` + shared.SnapshotRestoreScriptPath + `</annotate>`,
		"<allow_any>no</allow_any>",
		"<allow_inactive>no</allow_inactive>",
		"<allow_active>no</allow_active>",
	} {
		requireContains(t, policy, want, "the shipped polkit action")
	}

	rules := liveISOFile(t, strings.TrimPrefix(shared.SnapshotRestoreRulesPath, "/"))
	for _, want := range []string{
		"polkit.addRule",
		`action.id === "` + shared.SnapshotRestorePolkitAction + `"`,
		"subject.local",
		"subject.active",
		`subject.isInGroup("wheel")`,
	} {
		requireContains(t, rules, want, "the shipped grant")
	}
	// Measured twice, the expensive way: polkit's global object has addRule,
	// addAdminRule, log and spawn and nothing else. A rule that calls user_is_admin
	// throws, polkitd treats it as not-handled, and the policy's "no" answers. The
	// file may explain that in its comments; what must not appear is a call.
	for _, line := range strings.Split(rules, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.Contains(trimmed, "user_is_admin") {
			t.Error("the grant calls polkit.user_is_admin, which does not exist; use subject.isInGroup")
		}
	}
}

// TestShippedSnapshotPromptUnit is the gate, spelled once: the unit runs only when
// the marker exists - which is only in a snapshot boot - and the marker is also how
// the dialog learns the snapshot number. Everything else is the shape the seeded
// session units already have.
func TestShippedSnapshotPromptUnit(t *testing.T) {
	unit := liveISOFile(t, strings.TrimPrefix(shared.SnapshotPromptUnitDir+"/"+shared.SnapshotPromptUnitName, "/"))

	for _, want := range []string{
		"ConditionPathExists=" + shared.SnapshotBootMarkerPath,
		"EnvironmentFile=" + shared.SnapshotBootMarkerPath,
		// Its own Quickshell instance, and only our config: -c selects the named
		// config so a user's own ~/.config/quickshell config is not pulled in too.
		"ExecStart=/usr/bin/quickshell -c aelum-snapshot",
		// The prompt and dms.service are pulled in by the same wants link; the delay
		// lets Dux Shell draw its bar before the dialog maps over an empty desktop
		// (and reads a freshly written dms-colors.json).
		"ExecStartPre=/usr/bin/sleep 5",
		"Environment=QS_INSTANCE_ID=aelum-snapshot",
		"PartOf=graphical-session.target",
		"After=graphical-session.target",
		"Requisite=graphical-session.target",
		"WantedBy=hyprland-session.target",
	} {
		requireContains(t, unit, want, "the shipped prompt unit")
	}
	if !strings.Contains(unit, "[Install]") {
		t.Error("the prompt unit has no [Install] section; the wants link names it")
	}

	// The unit is system-side and the pull-in is per-home. A unit that named a home
	// would belong to one account, and a system-side wants link would pull the
	// prompt into the greeter's session too - the greeter is not in wheel and did
	// not choose a snapshot, so it must not be asked.
	if strings.Contains(unit, "%h") || strings.Contains(unit, "/home/") {
		t.Error("the prompt unit references a home directory; it must not belong to one user")
	}
	for _, line := range strings.Split(unit, "\n") {
		trimmed := strings.TrimSpace(line)
		// Comments describe the mechanism; only a unit line could actually put the
		// prompt in the wrong place.
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, "hyprland-session.target.wants") {
			t.Errorf("the unit names a wants directory (%q); the link is seeded into /etc/skel", trimmed)
		}
	}
}

// TestShippedSnapshotPromptQML pins the dialog against the things it must not
// become: a dependency on anything Quickshell does not ship, a question that can run
// the restore without the polkit action it names, an undisplaceable layer surface,
// a palette of its own, or an English-only dialog.
func TestShippedSnapshotPromptQML(t *testing.T) {
	qml := liveISOFile(t, strings.TrimPrefix(shared.SnapshotPromptConfigDir+"/"+promptConfigName, "/"))

	for _, want := range []string{
		"import Quickshell",
		"import Quickshell.Io",
		"import QtQuick",
		// The dialog is a real window, not a layer surface: a layer surface is fixed
		// to the screen by its anchors and the compositor cannot move one, which is
		// what made the first dialog impossible to drag. The title is the handle the
		// seeded Hyprland window rule matches (float + center), so it is part of the
		// contract too.
		"FloatingWindow",
		`title: "aelum-snapshot-prompt"`,
		// Pressing the card body asks the compositor to move the window
		// (xdg_toplevel's start_move, honored by Hyprland 0.56+): dragging the dialog
		// needs no compositor-side mouse bind.
		"startSystemMove",
		`color: "transparent"`,
		// The window is the card: its width is measured once from the buttons and
		// fixed from then on, the text is anchored to the top, the buttons are a
		// footer on the bottom-right corner, and the body reserves the ask phase's
		// height so pressing OK swaps the buttons in place instead of moving them
		// out from under the hand. Each button is as wide as its own label (with a
		// floor), and the window takes the widest footer the flow could need - no
		// language gets its label cut by a fixed width, and the card never resizes
		// mid-conversation.
		"id: content",
		"implicitWidth: Math.max(cardWidth, footerWidth + 2 * sidePadding)",
		"implicitHeight: content.height",
		"Math.max(dialog.buttonMinWidth, cancelLabel.implicitWidth",
		"Math.max(dialog.buttonMinWidth, okLabel.implicitWidth",
		"Math.max(dialog.buttonMinWidth, closeLabel.implicitWidth",
		"Math.max(dialog.buttonMinWidth, rebootLabel.implicitWidth",
		"id: buttons",
		"anchors.bottom: parent.bottom",
		"anchors.right: parent.right",
		"id: askBody",
		"height: Math.max(implicitHeight, askBody.height)",
		// The number, the phrase and the date arrive through the unit's
		// EnvironmentFile; the phrase and date are the menu's own words for the
		// booted snapshot, and the dialog shows them under the title so its
		// question names the state the way the menu entry did.
		`Quickshell.env("SNAPSHOT")`,
		`Quickshell.env("SNAPSHOT_PHRASE")`,
		`Quickshell.env("SNAPSHOT_DATE")`,
		// OK runs exactly the helper the polkit action names, through pkexec - never
		// a shell, never sudo. The action is resolved from the policy's exec.path
		// annotation, so the QML only ever spells the program.
		`command: ["pkexec", "` + shared.SnapshotRestoreScriptPath + `"]`,
		// The reboot is detached (closing the dialog would kill a tracked child
		// first) and reboot-only (the OK call already rolled back).
		`Quickshell.execDetached(["pkexec", "` + shared.SnapshotRestoreScriptPath + `", "--reboot-only"])`,
		// Cancelling is a first-class outcome: a snapshot boot is non-destructive and
		// plenty of people boot one just to look.
		`tr("cancel")`,
		"Qt.quit()",
		// The colors are the user's live DMS scheme: the file DMS writes when matugen
		// recolors, watched so the dialog follows it, with the mode read from the same
		// file. The fallback is the seeded scheme - the file /etc/skel carries - not a
		// palette the dialog invented.
		"dms-colors.json",
		"watchChanges: true",
		"onFileChanged: this.reload()",
		"JSON.parse",
		`dms.mode === "light"`,
		`/etc/skel/.cache/dms/dms-colors.json`,
		`color("surface_container")`,
		`color("primary")`,
		`color("on_primary")`,
		`color("outline_variant")`,
		// The copy is the user's language, resolved when the dialog runs: the session's
		// own locale first, the system's after it, English last.
		`Quickshell.env("LANG")`,
		"/etc/locale.conf",
		promptCopyPrefix,
		shared.SnapshotPromptI18nDirName,
		// Right-to-left locales mirror the whole card, footer included.
		"LayoutMirroring.enabled",
	} {
		requireContains(t, qml, want, "the shipped dialog")
	}

	// The footer's declaration order is its left-to-right order: cancel before OK,
	// close before reboot - the primary action rightmost in both phases, which is the
	// corner a hand reaches for by default.
	for _, pair := range [][2]string{{`tr("cancel")`, `tr("ok")`}, {`tr("close")`, `tr("reboot")`}} {
		first, second := strings.Index(qml, pair[0]), strings.Index(qml, pair[1])
		if first < 0 || second < 0 || first > second {
			t.Errorf("buttons are not primary-rightmost: %q at %d, %q at %d", pair[0], first, pair[1], second)
		}
	}

	// The last-resort block is real and complete: a dialog whose i18n files are
	// unreadable still asks the whole question, in English, rather than half of it.
	defaults := copyDefaultsBlock(qml)
	if defaults == "" {
		t.Error("the dialog has no English last-resort copy; an unreadable i18n file would leave it silent")
	}
	for _, key := range promptCopyKeys {
		if !strings.Contains(defaults, `"`+key+`":`) {
			t.Errorf("the last-resort copy is missing %q", key)
		}
	}

	// Every string the dialog shows goes through the translated namespace: a literal
	// sentence in the QML is a string no locale can change. The one place it may
	// spell its own copy is the block of English last resorts.
	for _, literal := range []string{
		`text: "CANCEL"`, `text: "OK"`, `text: "CLOSE"`, `text: "REBOOT NOW"`,
		`"You booted`, `"Restore this state`, `"Restoring`, `"Restored.`,
	} {
		if strings.Contains(withoutCopyDefaults(qml), literal) {
			t.Errorf("the dialog spells copy literally (%q); it must come from %s", literal, promptCopyPrefix)
		}
	}

	// QtQuick.Controls is a separate QML module; depending on it would make the dialog
	// depend on a package the install does not name. The card is plain QtQuick items
	// for exactly that reason.
	if strings.Contains(qml, "import QtQuick.Controls") {
		t.Error("the dialog imports QtQuick.Controls, which is not a Quickshell dependency")
	}
	// A layer surface again would undo the drag fix: Hyprland cannot move a surface
	// that is anchored to the screen edges.
	if strings.Contains(qml, "PanelWindow") {
		t.Error("the dialog is a PanelWindow again; a layer surface cannot be dragged")
	}
	// No palette of its own. The brand values live in the file /etc/skel carries; the
	// only colors the dialog may write down are the ones it uses when even that file
	// is unreadable.
	for _, gone := range []string{"#29272d", "#46434a", "#e3d6ff", "#2b1b52", "#938e99", "#ece6ee", "#fdf7ff", "#3c2d63"} {
		if strings.Contains(qml, gone) {
			t.Errorf("the dialog hardcodes the brand color %q; the fallback must read /etc/skel", gone)
		}
	}
}

// TestShippedSnapshotPromptI18n is the cross-repo half of the feature: the dialog
// asks in the user's language out of files the installer generates into the live ISO,
// so the languages the dialog can ask in are exactly the languages the installer
// speaks - no more, and none missing.
func TestShippedSnapshotPromptI18n(t *testing.T) {
	uiDir := "../../../frontend/src/translations"
	uiLocales, err := localeFiles(uiDir)
	if err != nil {
		t.Skipf("cannot read the installer's UI locales (%v)", err)
	}

	dir := os.Getenv(liveISODirEnv)
	if dir == "" {
		dir = liveISODir
	}
	copyDir := filepath.Join(dir, "airootfs", strings.TrimPrefix(shared.SnapshotPromptConfigDir, "/"), shared.SnapshotPromptI18nDirName)
	shipped, err := localeFiles(copyDir)
	if err != nil {
		t.Skipf("cannot read the shipped copy directory %s (%v)", copyDir, err)
	}

	if strings.Join(uiLocales, " ") != strings.Join(shipped, " ") {
		t.Errorf("the dialog can ask in %v, but the installer speaks %v", shipped, uiLocales)
	}

	english := readCopy(t, copyDir, "en-US")
	for _, code := range shipped {
		messages := readCopy(t, copyDir, code)
		for _, key := range promptCopyKeys {
			want := promptCopyPrefix + key
			if messages[want] == "" {
				t.Errorf("%s: %s is missing; the dialog would fall back to English mid-sentence", code, want)
				continue
			}
			if got, wantPlaceholders := placeholders(messages[want]), placeholders(english[want]); strings.Join(got, ",") != strings.Join(wantPlaceholders, ",") {
				t.Errorf("%s %s: placeholders %v, want %v", code, want, got, wantPlaceholders)
			}
		}
		for key := range messages {
			if !strings.HasPrefix(key, promptCopyPrefix) {
				t.Errorf("%s: %s is outside the %s namespace", code, key, promptCopyPrefix)
			}
		}
	}
}

func copyDefaultsBlock(qml string) string {
	const open = `readonly property var copyDefaults: ({`
	start := strings.Index(qml, open)
	if start < 0 {
		return ""
	}
	end := strings.Index(qml[start:], "})")
	if end < 0 {
		return ""
	}
	return qml[start : start+end+2]
}

// withoutCopyDefaults is the dialog minus its block of English last resorts, which
// is the one place it is allowed to spell its own copy.
func withoutCopyDefaults(qml string) string {
	block := copyDefaultsBlock(qml)
	if block == "" {
		return qml
	}
	return strings.Replace(qml, block, "", 1)
}

func localeFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var codes []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		codes = append(codes, strings.TrimSuffix(entry.Name(), ".json"))
	}
	sort.Strings(codes)
	return codes, nil
}

func readCopy(t *testing.T, dir, code string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, code+".json"))
	if err != nil {
		t.Errorf("%s: %v", code, err)
		return nil
	}
	var messages map[string]string
	if err := json.Unmarshal(data, &messages); err != nil {
		t.Errorf("%s: not JSON: %v", code, err)
		return nil
	}
	return messages
}

func placeholders(message string) []string {
	var found []string
	for {
		start := strings.Index(message, "{")
		if start < 0 {
			return found
		}
		end := strings.Index(message[start:], "}")
		if end < 0 {
			return found
		}
		name := message[start+1 : start+end]
		if name != "" && !strings.ContainsAny(name, " \t\n") {
			found = append(found, name)
		}
		message = message[start+end+1:]
	}
}

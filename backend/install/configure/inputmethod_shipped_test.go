package configure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aelum-installer/backend/shared"
)

// The input method wiring is live-ISO source like the snapshot path: the
// session unit and the seeded Hyprland env lines are copied unchanged (the unit
// by an isocopy job, the config with the whole /etc/skel tree). The isocopy job
// is Optional — an installer newer than its ISO must not fail over a missing
// file — which is exactly why the source's existence is asserted here: a
// silently skipped copy is an IME that never starts. The real-disk counterpart
// of these properties is docker/scripts/verify-installed.sh.

// fcitx5WantsLink is the seeded pull-in: the wants link lives in the skeleton,
// so every real home starts the unit and the greeter — which is not seeded
// from skel — is not asked.
const fcitx5WantsLink = "etc/skel/.config/systemd/user/hyprland-session.target.wants/"

// fcitx5AutostartEntry is the per-home override of the autostart entry the
// fcitx5 package ships. It is seeded into ~/.config/autostart so the session
// launcher leaves the daemon to the unit above.
const fcitx5AutostartEntry = "org.fcitx.Fcitx5.desktop"

// TestShippedFcitx5Unit holds the session unit as the live ISO ships it: the
// gate that keeps the live session inert, the daemon it starts, and the
// [Install] section the seeded wants link matches.
func TestShippedFcitx5Unit(t *testing.T) {
	unit := liveISOFile(t, strings.TrimPrefix(shared.Fcitx5UnitPath, "/"))

	requireContains(t, unit, "[Unit]", "a systemd unit")
	// The gate: the live image carries this unit (it is the source the installer
	// copies) but not fcitx5, so the unit must refuse to run there on its own —
	// no per-home mask, same shape as the snapshot prompt's marker gate.
	requireContains(t, unit, "ConditionPathExists=/usr/bin/fcitx5", "the gate that keeps the live session inert")
	requireContains(t, unit, "ExecStart=/usr/bin/fcitx5", "the daemon to start")
	requireContains(t, unit, "WantedBy=hyprland-session.target", "the [Install] section the seeded wants link matches")
}

// TestShippedInputMethodSourcesExist is the strict half of the tests in this file:
// the copy jobs for these paths are Optional, so an install cannot notice a source
// the live ISO stopped shipping — a silently skipped copy is an input method
// feature that never arrives. If the live-iso tree is here, the files have to be.
func TestShippedInputMethodSourcesExist(t *testing.T) {
	dir := os.Getenv(liveISODirEnv)
	if dir == "" {
		dir = liveISODir
	}
	root := filepath.Join(dir, "airootfs")
	if _, err := os.Stat(root); err != nil {
		t.Skipf("no live-iso tree at %s (%v)", root, err)
	}
	for _, rel := range []string{
		strings.TrimPrefix(shared.Fcitx5UnitPath, "/"),
		strings.TrimPrefix(shared.Fcitx5ConfigPath, "/"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("the live ISO does not ship %s: the install's copy job for it is Optional, so nothing else would say so (%v)", rel, err)
		}
	}
}

// TestShippedFcitx5WantsLink checks the seeded pull-in actually points at the
// unit's system path — the same reason the snapshot prompt's link is asserted:
// a link to anywhere else is a unit no session will ever start.
func TestShippedFcitx5WantsLink(t *testing.T) {
	dir := os.Getenv(liveISODirEnv)
	if dir == "" {
		dir = liveISODir
	}
	link := filepath.Join(dir, "airootfs", fcitx5WantsLink+shared.Fcitx5UnitName)
	target, err := os.Readlink(link)
	if err != nil {
		t.Skipf("cannot read the seeded wants link %s (%v): this test needs the live-iso tree beside the installer", link, err)
	}
	if target != shared.Fcitx5UnitPath {
		t.Errorf("the seeded wants link points at %q, want %s", target, shared.Fcitx5UnitPath)
	}
}

// TestShippedHyprlandIMEEnvironment holds the client half of the wiring in the
// seeded session config. Qt and XWayland clients need the module selectors —
// Qt's own Wayland text-input-v2 exists only in KWin — and GTK_IM_MODULE must
// NOT be set: Hyprland implements the Wayland input-method protocol, so a
// native GTK client composes through that, and a set GTK_IM_MODULE makes
// fcitx5 post a first-login advisory asking for it to be unset. Measured on an
// installed system: Pinyin preedit and the candidate window worked in GNOME
// Text Editor (GTK4) and in Firefox's URL bar (GTK3) with the variable unset.
func TestShippedHyprlandIMEEnvironment(t *testing.T) {
	config := liveISOFile(t, "etc/skel/.config/hypr/hyprland.lua")

	for _, want := range []string{
		`hl.env("QT_IM_MODULE", "fcitx")`,
		`hl.env("XMODIFIERS", "@im=fcitx")`,
	} {
		requireContains(t, config, want, "the input method environment for session clients")
	}
	if strings.Contains(config, `hl.env("GTK_IM_MODULE"`) {
		t.Error(`the seeded session config sets GTK_IM_MODULE: a native Wayland client composes through the compositor's input-method protocol, and the framework warns about this variable on every first session`)
	}
}

// TestShippedFcitx5Hotkeys holds the framework's own config as the live ISO ships
// it: the switch key is Alt+Shift, on both Shift keys because the framework needs
// the keysym rather than the modifier word (Key::Key in 5.1.23 only recognises
// "Shift+" with a trailing plus, so a bare trailing "Shift" is dropped and the key
// becomes Alt alone), and the key that reaches more than two input methods is kept
// as well — a Japanese or Korean install lists its keyboard beside its engine, and
// toggling only ever reaches English and the engine.
func TestShippedFcitx5Hotkeys(t *testing.T) {
	config := liveISOFile(t, strings.TrimPrefix(shared.Fcitx5ConfigPath, "/"))

	for _, want := range []string{
		"[Hotkey/TriggerKeys]",
		"0=Alt+Shift_L",
		"1=Alt+Shift_R",
		"[Hotkey/EnumerateForwardKeys]",
	} {
		requireContains(t, config, want, "the input method switch key")
	}
	for _, line := range strings.Split(config, "\n") {
		if strings.HasSuffix(strings.TrimSpace(line), "=Alt+Shift") {
			t.Error(`a hotkey is written "Alt+Shift": the framework parses that as Alt alone, because its parser looks for "Shift+" with a trailing plus`)
		}
	}
}

// TestShippedSessionLeavesLayoutSwitchingToTheFramework: the seeded session config
// must not carry an XKB layout toggle. The framework owns the key now, and a
// keyboard layout is one of its input methods — with both in place, one press of
// Alt+Shift would move the compositor's layout and the session's input method at
// the same time, and the two would disagree about which one is active.
func TestShippedSessionLeavesLayoutSwitchingToTheFramework(t *testing.T) {
	config := liveISOFile(t, "etc/skel/.config/hypr/hyprland.lua")

	for _, line := range strings.Split(config, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue // the comment explaining why the option is gone
		}
		if strings.Contains(trimmed, "kb_options") {
			t.Error("the seeded session config still assigns an XKB option; Alt+Shift belongs to the input method framework now")
			break
		}
	}
	// The English baseline stays: it is what the greeter (which never starts the
	// framework) and a session with the input method switched off type with, and it
	// is the same layout the framework's own first entry uses.
	requireContains(t, config, `kb_layout = "us"`, "the English baseline the profile's first entry matches")
}

// TestShippedFcitx5AutostartHidden holds the seeded override of the autostart
// entry the fcitx5 package ships. The session launcher runs that entry, so
// without the override two things start the daemon: the autostart copy wins the
// D-Bus name, the unit's process sees an existing instance and exits 0, and the
// daemon ends up running outside the unit that carries the gate and the restart
// policy — measured on an installed system, where the framework lived in
// app-org.fcitx.Fcitx5@autostart.service while aelum-fcitx5.service read
// "inactive (dead)".
func TestShippedFcitx5AutostartHidden(t *testing.T) {
	entry := liveISOFile(t, "etc/skel/.config/autostart/"+fcitx5AutostartEntry)

	requireContains(t, entry, "Hidden=true", "the desktop-entry spec's way of turning the packaged autostart entry off")
	// A per-home autostart file is merged with the system one by filename, so it
	// has to stay a complete entry: a missing Type or a different Exec is an
	// entry no launcher trusts, and the name is how the two files meet at all.
	requireContains(t, entry, "Exec=/usr/bin/fcitx5", "the same entry the package ships")
	requireContains(t, entry, "Type=Application", "a complete desktop entry")
}

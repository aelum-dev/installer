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

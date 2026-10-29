package configure

import (
	"fmt"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// The prompt a snapshot boot asks
// ============================================================================
//
// A user who chose a snapshot in the boot menu is standing in a previous state.
// The right question to ask them, in the session that comes up, is the one this
// file installs:
//
//     "You booted snapshot N. Restore this state as the system?"   OK / CANCEL
//
// Three pieces, all system-side:
//
//   /etc/xdg/quickshell/aelum-snapshot/shell.qml                    the dialog
//   /etc/systemd/user/aelum-snapshot-prompt.service                 runs it,
//       gated on the /run marker the state generator writes (an ordinary boot
//       never starts it)
//   /etc/systemd/user/hyprland-session.target.wants/…service        the pull-in
//
// Why system-side and not in the install user's home: nothing about the prompt
// is per-user. The dialog is static, the unit is generic, and both its gate and
// its snapshot number come from system state (/run). A file in one account's
// home belongs to that account: delete the user, create a new one, and the new
// user has no prompt. These three files belong to the system, so every user -
// the install user, one created later, one recreated after deletion - gets the
// prompt, and no home directory is written into at all.
//
// The placement is the documented one for each half:
//
//   - Quickshell scans every XDG config directory, /etc/xdg included, for
//     quickshell/<name>/shell.qml; its own distribution guide tells packaged
//     configs to live under $XDG_CONFIG_DIRS. `quickshell -c aelum-snapshot`
//     selects this config for any user, and a user's own ~/.config copy of the
//     same name would take precedence - a deliberate override, not a conflict.
//   - systemd's user manager loads units from /etc/systemd/user for every
//     login ("user units created by the administrator"), and a target's .wants/
//     directory is collected from every load path - that is how `systemctl
//     enable` pulls a packaged unit into a target. So this system-side link
//     pulls the prompt into the per-user hyprland-session.target that the
//     installer seeds into every home, without writing anything into a home.
//
// Like the restore helper and its sudoers rule, these files are inside the
// root and are written before the "after installation" snapshot: a snapshot
// boots the prompt it contains, which is the same rule that makes the OK button
// work from inside an old state.
//
// Why Quickshell and not a widget inside Dux Shell: DMS has no user-facing
// modal dialog API and its settings.json is a seeded generated baseline;
// Quickshell is the engine DMS itself runs on, is installed as its dependency,
// and a standalone singleton of its own instance keeps this recovery-critical
// flow out of a third party's config format. The unit passes QS_INSTANCE_ID so
// the prompt never shares an IPC socket with Dux Shell.

const (
	// snapshotPromptConfigDir is the system-side Quickshell config. Quickshell
	// treats each named subfolder of an XDG config directory's quickshell/ that
	// contains a shell.qml as a config; the unit selects this one by name (-c).
	snapshotPromptConfigDir  = "/etc/xdg/quickshell/aelum-snapshot"
	snapshotPromptConfigName = "shell.qml"

	// snapshotPromptUnitName is the user unit. It is not "enabled" per user -
	// the system-side wants link below pulls it into every session.
	snapshotPromptUnitName = "aelum-snapshot-prompt.service"
	snapshotPromptUnitDir  = "/etc/systemd/user"
	snapshotPromptWantsDir = snapshotPromptUnitDir + "/hyprland-session.target.wants"
)

// configureSnapshotPrompt writes the dialog, its unit, and the session pull-in
// into the installed system's own /etc. It is btrfs-only with the rest of the
// snapshot machinery, and it belongs before the "after installation" snapshot
// for the same reason the restore helper does: a snapshot runs the units and
// configs it contains.
func configureSnapshotPrompt(ctx *shared.InstallContext) error {
	if err := primitives.WriteFile(snapshotPromptConfigDir+"/"+snapshotPromptConfigName, snapshotPromptQML); err != nil {
		return fmt.Errorf("failed to write the snapshot prompt config: %w", err)
	}
	unitPath := snapshotPromptUnitDir + "/" + snapshotPromptUnitName
	if err := primitives.WriteFile(unitPath, snapshotPromptUnit); err != nil {
		return fmt.Errorf("failed to write %s: %w", snapshotPromptUnitName, err)
	}

	// The pull-in. systemd starts a user unit that a target's .wants/ directory
	// names, and it collects that directory from every load path - so this link
	// reaches the per-user hyprland-session.target the installer seeds into
	// every home. There is no user manager running during the install to ask,
	// which is why this is a link and not `systemctl --user enable`.
	linkPath := snapshotPromptWantsDir + "/" + snapshotPromptUnitName
	if out, err := primitives.RunChroot("mkdir", "-p", snapshotPromptWantsDir); err != nil {
		return fmt.Errorf("failed to create %s: %w: %s", snapshotPromptWantsDir, err, strings.TrimSpace(string(out)))
	}
	if out, err := primitives.RunChroot("ln", "-sf", unitPath, linkPath); err != nil {
		return fmt.Errorf("failed to link %s into hyprland-session.target.wants: %w: %s", snapshotPromptUnitName, err, strings.TrimSpace(string(out)))
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Snapshot restore prompt installed: a booted snapshot asks whether to keep it")
	}
	return nil
}

// snapshotPromptUnit is the user unit. ConditionPathExists is the whole gate: the
// marker exists only in a snapshot boot (see snapshot_state.go), so an ordinary
// boot never starts this unit at all. EnvironmentFile reads the same file, which
// is how the dialog learns the snapshot number without parsing anything itself.
const snapshotPromptUnit = `# SPDX-License-Identifier: GPL-3.0
#
# Ask whether to restore the snapshot this system booted into.
#
# This unit is system-side (/etc/systemd/user, a load path of every user
# manager) and is pulled in by
# /etc/systemd/user/hyprland-session.target.wants/ - a target's .wants/ is
# collected from every load path, so this one link reaches the session target in
# every home, including homes created after this install. It runs only when
# ` + snapshotBootMarkerPath + ` exists, which the aelum-snapshot-state generator
# writes exactly when the root is not the default subvolume. An ordinary boot
# never runs this unit.
#
# The dialog is /etc/xdg/quickshell/aelum-snapshot/shell.qml, run as its own
# Quickshell instance (QS_INSTANCE_ID) so it never shares an IPC socket with Dux
# Shell. OK runs /usr/local/bin/aelum-snapshot-restore through the sudoers rule
# the installer writes for it; CANCEL closes and changes nothing.

[Unit]
Description=Aelum: ask about restoring the snapshot this system booted into
PartOf=graphical-session.target
After=graphical-session.target
Requisite=graphical-session.target
ConditionPathExists=` + snapshotBootMarkerPath + `

[Service]
Type=simple
Environment=QS_INSTANCE_ID=aelum-snapshot
EnvironmentFile=` + snapshotBootMarkerPath + `
ExecStart=/usr/bin/quickshell -c aelum-snapshot
Restart=on-failure
RestartSec=1

[Install]
WantedBy=hyprland-session.target
`

// snapshotPromptQML is the dialog. It is deliberately plain QtQuick - no
// QtQuick.Controls, no theme files - so the only thing it depends on is
// Quickshell itself. Its colors are the user's live Dux Shell scheme: the QML
// reads ~/.cache/dms/dms-colors.json (mode + Material color roles), watches it
// for changes, and falls back per role to the brand scheme only when that file
// is missing or unreadable.
const snapshotPromptQML = `// SPDX-License-Identifier: GPL-3.0
//
// aelum-snapshot-prompt - the question a snapshot boot asks.
//
// This system booted a snapshot: the boot menu offered a previous state and the
// user chose it. A snapshot boot is read-only and non-destructive - rebooting out
// of it loses nothing - so the only thing worth doing from inside one is making it
// the system again. That is snapper rollback, and this is the permission request.
//
// It runs only on a snapshot boot: aelum-snapshot-prompt.service is gated on the
// marker the aelum-snapshot-state generator writes, and the snapshot number comes
// through that same file (EnvironmentFile), so the question names the state it is
// about. OK runs /usr/local/bin/aelum-snapshot-restore - the sudoers rule the
// installer writes allows exactly that program without a password. CANCEL closes
// and changes nothing, which is the point of a snapshot boot.
//
// The colors are the user's live Dux Shell scheme, read from
// ~/.cache/dms/dms-colors.json - the file DMS writes whenever matugen recolors
// the desktop - watched for changes, so the dialog is the current theme in
// whichever mode (dark or light) the desktop is in, and recolors live if the
// theme changes while it is open. Only when that file is missing or unreadable
// does it fall back to the brand scheme (the seeded palette), so the dialog is
// never unstyled and never invents its own colors.

import Quickshell
import Quickshell.Io
import QtQuick

Scope {
  id: prompt

  // The number the unit's EnvironmentFile carried in. 0 means "a snapshot,
  // number unknown" - the marker's existence is the fact, the number is detail.
  property int snapshot: parseInt(Quickshell.env("SNAPSHOT") || "0", 10) || 0

  // The dialog's whole state: ask, then the result of pressing OK.
  property string phase: "ask"
  property string detail: ""

  // The live DMS scheme. watchChanges + reload is what makes "always the current
  // DMS colors" true rather than "the colors as of when the dialog opened":
  // matugen recolors DMS by rewriting this file, and the view follows it.
  // printErrors off: a missing file is a supported case (the fallback below),
  // not something to complain about in the logs.
  FileView {
    id: dmsColors
    path: Quickshell.env("HOME") + "/.cache/dms/dms-colors.json"
    watchChanges: true
    onFileChanged: this.reload()
    printErrors: false
  }

  property var dms: {
    try {
      return JSON.parse(dmsColors.text());
    } catch (e) {
      return null;
    }
  }

  // DMS names its current mode in the same file, so the dialog follows the
  // desktop's dark/light choice rather than guessing from the clock.
  readonly property string dmsMode: (dms && dms.mode === "light") ? "light" : "dark"

  // The brand scheme - the exact values the installer seeds into
  // ~/.cache/dms/dms-colors.json - used only as the fallback per role.
  readonly property var brandPalette: ({
    "dark": {
      "surface_container": "#29272d", "surface_bright": "#46434a",
      "on_surface": "#ffffff", "on_surface_variant": "#e0dae5",
      "primary": "#e3d6ff", "on_primary": "#2b1b52",
      "outline_variant": "#938e99", "error": "#ffd2cc"
    },
    "light": {
      "surface_container": "#ece6ee", "surface_bright": "#fdf7ff",
      "on_surface": "#121016", "on_surface_variant": "#38353d",
      "primary": "#3c2d63", "on_primary": "#ffffff",
      "outline_variant": "#6f6b75", "error": "#740006"
    }
  })

  // One role lookup: the live scheme first, the brand scheme per role after it.
  // Per role, not per file: a scheme missing one role still renders in the
  // user's colors.
  function color(role) {
    var live = (dms && dms.colors) ? dms.colors[dmsMode] : null;
    if (live && live[role]) {
      return live[role];
    }
    return brandPalette[dmsMode][role] || brandPalette["dark"][role];
  }

  function close() {
    Qt.quit()
  }

  Process {
    id: restoreProc
    command: ["sudo", "-n", "/usr/local/bin/aelum-snapshot-restore"]
    onExited: (code) => { prompt.phase = code === 0 ? "done" : "failed"; }
    // Both channels collected so snapper's progress output never blocks on a
    // closed pipe, and its error is the message the failure line shows.
    stdout: StdioCollector { onStreamFinished: prompt.detail = text.trim() }
    stderr: StdioCollector { onStreamFinished: prompt.detail = text.trim() }
  }

  // The REBOOT button is detached on purpose: closing the dialog quits
  // Quickshell, and a tracked child would be killed with it before it reached
  // systemctl. --reboot-only because the OK button's call already rolled back;
  // a second rollback would take a second undo snapshot of a restored system.
  function reboot() {
    Quickshell.execDetached(["sudo", "-n", "/usr/local/bin/aelum-snapshot-restore", "--reboot-only"]);
  }

  PanelWindow {
    id: dialog
    visible: true

    // The Quickshell shape for a dialog: a layer surface the size of the card,
    // centered by margins. Wayland input follows surface geometry, not opacity -
    // a full-screen transparent surface swallows every click in the session and
    // holds the keyboard, which makes a question you may want to think about
    // block everything. Anchoring all four sides forces each dimension to the
    // screen size, and margins pull the surface back to the card, so every
    // click and key outside the card reaches the session as if the prompt were
    // not there. The width is fixed; the height is the Column's own height plus
    // the card's vertical padding, so no phase reserves space its text and
    // buttons do not use. On a screen smaller than the card the margins clamp
    // to zero and the card fills the screen.
    property int cardWidth: 540
    property int cardHeight: content.height + 56 // 28px of padding above and below

    anchors { top: true; bottom: true; left: true; right: true }
    margins {
      left: Math.max(0, Math.round((screen.width - cardWidth) / 2))
      right: Math.max(0, Math.round((screen.width - cardWidth) / 2))
      top: Math.max(0, Math.round((screen.height - cardHeight) / 2))
      bottom: Math.max(0, Math.round((screen.height - cardHeight) / 2))
    }
    exclusionMode: ExclusionMode.Ignore
    color: "transparent"
    focusable: true

    Rectangle {
      id: card
      anchors.fill: parent
      radius: 18
      color: prompt.color("surface_container")
      border.color: prompt.phase === "failed" ? prompt.color("error") : prompt.color("primary")
      border.width: 2
      focus: true

      Keys.onEscapePressed: prompt.close()
      Keys.onReturnPressed: {
        if (prompt.phase === "ask") {
          prompt.phase = "restoring";
          restoreProc.running = true;
        } else if (prompt.phase === "done") {
          prompt.reboot();
          prompt.close();
        } else {
          prompt.close();
        }
      }

      Column {
        id: content
        x: 32
        y: 28
        width: parent.width - 64
        spacing: 16

        Text {
          text: prompt.snapshot > 0 ? "You booted snapshot " + prompt.snapshot : "You booted a snapshot"
          color: prompt.color("on_surface")
          font.pixelSize: 22
          font.bold: true
        }

        Text {
          width: parent.width
          wrapMode: Text.WordWrap
          color: prompt.phase === "failed" ? prompt.color("error") : prompt.color("on_surface_variant")
          font.pixelSize: 15
          text: {
            if (prompt.phase === "ask") {
              return "Restore this state as the system?\n\n"
                  + "The system you were using is kept as an undo snapshot, and the "
                  + "restored state boots after a reboot. Nothing changes until you "
                  + "press OK.";
            }
            if (prompt.phase === "restoring") {
              return "Restoring\u2026";
            }
            if (prompt.phase === "done") {
              return "Restored. The restored system boots on the next reboot.";
            }
            return "Restore failed: "
                + (prompt.detail || "aelum-snapshot-restore exited with an error");
          }
        }

        Row {
          spacing: 12

          Rectangle {
            visible: prompt.phase === "ask"
            width: 130
            height: 42
            radius: 10
            color: prompt.color("primary")
            Text {
              anchors.centerIn: parent
              text: "OK"
              color: prompt.color("on_primary")
              font.pixelSize: 15
              font.bold: true
            }
            MouseArea {
              anchors.fill: parent
              cursorShape: Qt.PointingHandCursor
              onClicked: {
                prompt.phase = "restoring";
                restoreProc.running = true;
              }
            }
          }

          Rectangle {
            visible: prompt.phase === "ask"
            width: 130
            height: 42
            radius: 10
            color: prompt.color("surface_bright")
            border.color: prompt.color("outline_variant")
            Text {
              anchors.centerIn: parent
              text: "CANCEL"
              color: prompt.color("on_surface")
              font.pixelSize: 15
            }
            MouseArea {
              anchors.fill: parent
              cursorShape: Qt.PointingHandCursor
              onClicked: prompt.close()
            }
          }

          Rectangle {
            visible: prompt.phase === "done"
            width: 170
            height: 42
            radius: 10
            color: prompt.color("primary")
            Text {
              anchors.centerIn: parent
              text: "REBOOT NOW"
              color: prompt.color("on_primary")
              font.pixelSize: 15
              font.bold: true
            }
            MouseArea {
              anchors.fill: parent
              cursorShape: Qt.PointingHandCursor
              onClicked: {
                prompt.reboot();
                prompt.close();
              }
            }
          }

          Rectangle {
            visible: prompt.phase === "done" || prompt.phase === "failed"
            width: 130
            height: 42
            radius: 10
            color: prompt.color("surface_bright")
            border.color: prompt.color("outline_variant")
            Text {
              anchors.centerIn: parent
              text: "CLOSE"
              color: prompt.color("on_surface")
              font.pixelSize: 15
            }
            MouseArea {
              anchors.fill: parent
              cursorShape: Qt.PointingHandCursor
              onClicked: prompt.close()
            }
          }
        }
      }
    }
  }

  // Answering is the whole job; once the dialog is gone the process has nothing
  // left to do.
  Connections {
    target: Quickshell
    function onLastWindowClosed() {
      Qt.quit();
    }
  }
}
`

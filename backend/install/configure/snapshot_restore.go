package configure

import (
	"fmt"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// Restoring the snapshot a snapshot boot is running
// ============================================================================
//
// A snapshot entry in the boot menu boots a previous state (refind_snapshots.go)
// and the state directories it needs (snapshot_state.go). The one thing worth
// doing from inside it is making it the system again - `snapper rollback`, which
// test-snapshot-rollback.sh runs end to end against the real disk. This file is
// the two pieces the desktop prompt needs to offer that as a button:
//
//   /usr/local/bin/aelum-snapshot-restore   the rollback, wrapped once
//   /etc/sudoers.d/aelum-snapshot-restore   the OK button's passwordless path
//
// Both are root-side files and both are written before the "after installation"
// snapshot: a snapshot boots the binaries and rules it contains, so the restore
// path has to exist inside every state the menu can boot you into.

const (
	// snapshotRestoreScriptPath is the helper. Same territory as the menu
	// generator - /usr/local is the hierarchy no package writes to, so a snapper
	// or sudo upgrade can never remove it or the rule naming it.
	snapshotRestoreScriptPath = "/usr/local/bin/aelum-snapshot-restore"

	// snapshotRestoreSudoersPath is the narrow rule that lets the prompt's OK
	// button run the helper without a password. It names one absolute program and
	// nothing else; the program itself refuses a non-snapshot boot.
	snapshotRestoreSudoersPath = "/etc/sudoers.d/aelum-snapshot-restore"

	// sudoersMode: sudo refuses a world-writable sudoers file (visudo's
	// "sudoers file is world-writable" error), so the drop-in is 0440 like the
	// wheel rule install/users writes.
	sudoersMode = 0o440
)

// configureSnapshotRestore installs the restore path: the helper script and the
// narrow sudoers rule that lets the prompt's OK button run it without a password.
// Like the rest of the snapshot machinery it is btrfs-only (an ext4 install has no
// snapshot to restore) and it runs before the "after installation" snapshot,
// because a snapshot boots the binaries and rules it contains.
func configureSnapshotRestore(ctx *shared.InstallContext) error {
	if err := primitives.Run("mkdir", "-p", shared.MountRoot+"/usr/local/bin").Run(); err != nil {
		return fmt.Errorf("failed to create %s: %w", "/usr/local/bin", err)
	}
	if err := primitives.WriteFileMode(shared.MountRoot+snapshotRestoreScriptPath, snapshotRestoreScript, scriptMode); err != nil {
		return fmt.Errorf("failed to write %s: %w", snapshotRestoreScriptPath, err)
	}
	if err := primitives.WriteFileMode(shared.MountRoot+snapshotRestoreSudoersPath, snapshotRestoreSudoers, sudoersMode); err != nil {
		return fmt.Errorf("failed to write %s: %w", snapshotRestoreSudoersPath, err)
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("Snapshot restore installed: a booted snapshot can be made the system again")
	}
	return nil
}

const snapshotRestoreScript = `#!/bin/sh
# SPDX-License-Identifier: GPL-3.0
#
# aelum-snapshot-restore - make the snapshot this system is booted into the system again.
#
# This is the OK button of the snapshot prompt (aelum-snapshot-prompt), spelled
# once so the sudoers rule, the prompt, the acceptance test and a person in a
# terminal all run the same command.
#
# Why it refuses anything but a snapshot boot: "snapper rollback" with no number
# means "the state I am booted in" (snapper resolves the current subvolume and
# accepts only its own numbered snapshots), which is only the right answer from
# inside a snapshot. Run from an ordinary boot it would roll the system back onto
# itself. The marker /run/aelum-snapshot-boot is written by the
# aelum-snapshot-state generator exactly when this root is a snapshot, so this
# script refuses when it is not.
#
# What a rollback does, in snapper's own words: an undo snapshot of the state that
# was live ("read-only snapshot of default subvolume"), a writable copy of the
# snapshot this system is booted in ("read-write snapshot of current subvolume"),
# and the default subvolume repointed at it. This installation's fstab and its own
# boot entry name no subvolume - the whole point of the layout - so the next
# reboot boots the restored state with no menu or fstab change. /home and the
# other live subvolumes are untouched, and the session you pressed OK in is still
# the old read-only snapshot until that reboot.

set -u

# The sudoers rule runs this as root; anything else is a mistake worth naming.
[ "$(id -u)" = 0 ] || { echo "aelum-snapshot-restore: this must run as root" >&2; exit 1; }

marker=${AELUM_SNAPSHOT_MARKER:-` + snapshotBootMarkerPath + `}
[ -f "$marker" ] || {
    echo "aelum-snapshot-restore: this boot is not a snapshot; there is no state to restore" >&2
    exit 1
}

# --no-dbus for the reason the pacman hooks use it: the D-Bus path adds a service
# that can fail, and here that failure would look like the restore failing.
case "${1-}" in
    "")
        snapper --no-dbus -c root rollback || exit 1
        ;;
    --reboot)
        # Rollback and reboot in one step - the terminal spelling.
        snapper --no-dbus -c root rollback || exit 1
        exec systemctl reboot
        ;;
    --reboot-only)
        # The prompt's REBOOT button: the rollback already happened in the call
        # the OK button made. Running rollback a second time here would take a
        # second undo snapshot of a system that is already restored.
        exec systemctl reboot
        ;;
    *)
        echo "usage: aelum-snapshot-restore [--reboot | --reboot-only]" >&2
        exit 1
        ;;
esac
exit 0
`

const snapshotRestoreSudoers = `# SPDX-License-Identifier: GPL-3.0
# Written by the Aelum installer: the snapshot prompt's OK button runs
# /usr/local/bin/aelum-snapshot-restore without a password. The rule names one
# absolute program and nothing else; the program itself refuses a boot that is
# not a snapshot (see it for what a rollback does).
%wheel ALL=(ALL) NOPASSWD: ` + snapshotRestoreScriptPath + `
`

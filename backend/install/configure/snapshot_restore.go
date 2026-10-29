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
// the three pieces the desktop prompt needs to offer that as a button:
//
//   /usr/local/bin/aelum-snapshot-restore                          the rollback, wrapped once
//   /usr/share/polkit-1/actions/dev.aelum.snapshot.restore.policy the action pkexec runs it through
//   /etc/polkit-1/rules.d/aelum-snapshot-restore.rules            who may run it without further auth
//
// All three are root-side files and all are written before the "after
// installation" snapshot: a snapshot boots the binaries and rules it contains, so
// the restore path has to exist inside every state the menu can boot you into.
//
// Why polkit and not sudo. The OK button needs root, and the first version asked
// for it with a NOPASSWD sudoers drop-in. That failed on the real install for a
// reason worth keeping: sudoers is last-match-wins across the whole includedir,
// and the installer's own /etc/sudoers.d/wheel (`%wheel ALL=(ALL:ALL) ALL`)
// sorts after `aelum-snapshot-restore`, so the broad rule answered last and the
// button got "sudo: a password is required". Even without that collision a
// NOPASSWD line is an exception carved out of a mechanism that cannot describe
// actions - it cannot say who, from where, in what session. polkit is the
// mechanism that can: pkexec finds the action through the policy file's
// exec.path annotation, the policy's defaults refuse it everywhere, and the
// local rules file grants exactly one case - the local, active session of an
// administrator (polkit's admin is the wheel group on Arch, the group the
// installer puts its users in), which is the session of the person who just
// pressed OK. An SSH session (local but not active), the greeter (active but
// not admin), and every other user get the refusal, and a refusal is the right
// answer for a request to replace the system. The helper still refuses a
// non-snapshot boot on its own, so the action can only ever do the one thing.

const (
	// snapshotRestoreScriptPath is the helper. Same territory as the menu
	// generator - /usr/local is the hierarchy no package writes to, so a snapper
	// upgrade can never remove it. pkexec requires it to be root-owned and not
	// group- or world-writable (its authenticity check for any program an action
	// execs); the 0755 it is written with satisfies that.
	snapshotRestoreScriptPath = "/usr/local/bin/aelum-snapshot-restore"

	// snapshotRestorePolkitAction is the action id. pkexec resolves it from the
	// exec.path annotation, so the desktop only ever names the helper; the id is
	// what the rules file, the tests and the acceptance scripts match on.
	snapshotRestorePolkitAction = "dev.aelum.snapshot.restore"
	snapshotRestorePolicyPath   = "/usr/share/polkit-1/actions/" + snapshotRestorePolkitAction + ".policy"
	snapshotRestoreRulesPath    = "/etc/polkit-1/rules.d/aelum-snapshot-restore.rules"
)

// configureSnapshotRestore installs the restore path: the helper script and the
// polkit policy + rule that let the prompt's OK button run it through pkexec.
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
	if err := primitives.Run("mkdir", "-p", shared.MountRoot+"/usr/share/polkit-1/actions").Run(); err != nil {
		return fmt.Errorf("failed to create %s: %w", "/usr/share/polkit-1/actions", err)
	}
	if err := primitives.WriteFile(shared.MountRoot+snapshotRestorePolicyPath, snapshotRestorePolicy); err != nil {
		return fmt.Errorf("failed to write %s: %w", snapshotRestorePolicyPath, err)
	}
	if err := primitives.Run("mkdir", "-p", shared.MountRoot+"/etc/polkit-1/rules.d").Run(); err != nil {
		return fmt.Errorf("failed to create %s: %w", "/etc/polkit-1/rules.d", err)
	}
	if err := primitives.WriteFile(shared.MountRoot+snapshotRestoreRulesPath, snapshotRestoreRules); err != nil {
		return fmt.Errorf("failed to write %s: %w", snapshotRestoreRulesPath, err)
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
# once so the polkit action, the prompt, the acceptance test and a person in a
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

# pkexec (through the polkit action) runs this as root; anything else is a mistake worth naming.
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

// snapshotRestorePolicy is the polkit action. The defaults refuse it everywhere -
// the grant is the local rules file's job, so a missing or broken rules file
// fails closed. The exec.path annotation is what makes `pkexec <helper>` resolve
// to this action: the desktop names the program, never the action, and pkexec
// authenticates the program (root-owned, not group/world-writable) before it asks
// polkit anything at all.
const snapshotRestorePolicy = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE policyconfig PUBLIC
 "-//freedesktop//DTD PolicyKit Policy Configuration 1.0//EN"
 "https://specifications.freedesktop.org/PolicyKit/1.0/policyconfig.dtd">
<policyconfig>
  <vendor>Aelum</vendor>
  <vendor_url>https://aelum.dev</vendor_url>

  <action id="` + snapshotRestorePolkitAction + `">
    <description>Restore the snapshot this system booted into</description>
    <message>Restoring makes the booted snapshot the system again; the state it replaces is kept as an undo snapshot.</message>
    <defaults>
      <allow_any>no</allow_any>
      <allow_inactive>no</allow_inactive>
      <allow_active>no</allow_active>
    </defaults>
    <annotate key="org.freedesktop.policykit.exec.path">` + snapshotRestoreScriptPath + `</annotate>
  </action>
</policyconfig>
`

// snapshotRestoreRules is the grant, and the only place that gives one. Local
// rules are consulted before the policy defaults, so this decides the one case
// that needs no asking and nothing else changes.
const snapshotRestoreRules = `// SPDX-License-Identifier: GPL-3.0
// Written by the Aelum installer: who may run the snapshot-restore helper without
// further authentication.
//
// The grant is the session of the person who pressed OK: a session on the local
// machine (subject.local), in the foreground of a seat (subject.active), and an
// administrator - polkit's admin is the wheel group on Arch, the group the
// installer puts its users in. Everything else - an SSH session (local but not
// active), the greeter (active but not admin), another user, another seat -
// falls through to the policy's refusal, and a refusal is the right answer for a
// request to replace the system.
polkit.addRule(function(action, subject) {
    if (action.id === "` + snapshotRestorePolkitAction + `" &&
        subject.local && subject.active && polkit.user_is_admin) {
        return polkit.Result.YES;
    }
});
`

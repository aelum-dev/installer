package configure

import (
	"fmt"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// Snapshot boots: the state directories a read-only root still needs
// ============================================================================
//
// A snapshot entry in the boot menu boots a read-only btrfs subvolume as / (see
// refind_snapshots.go). There is no overlayfs under it: the kernel mounts that
// subvolume and everything inside the root inherits it. Most of the layout is
// unaffected - /home, /srv, /var/cache, /var/tmp and /var/log are separate
// subvolumes and stay the live ones - but the directories under /var/lib are
// inside the root, and one of them is what a greeter needs.
//
// Measured on a real snapshot boot of a real install, before this existed:
//
//     systemd-logind[xxx]: Failed to set up special execution directory in /var/lib: Read-only file system
//     systemd-logind[xxx]: Failed at step STATE_DIRECTORY spawning /usr/lib/systemd/systemd-logind: Read-only file system
//     … retried to its start limit … then getty@tty1: a text login, not the greeter.
//
// A snapshot that boots to a TTY is not a recovery entry a user can use, so those
// directories get a tmpfs: transient state (login sessions, user records, inhibit
// locks, the greeter's scratch) that should not survive anything and must not be
// written into the snapshot - which is what keeps a snapshot boot non-destructive.
//
// Why a generator and not a unit with a condition, since the obvious condition is
// wrong. `ConditionPathIsReadWrite=!/` is *unmet* in a real snapshot boot: a
// btrfs read-only subvolume mounted with rw looks read-write to the mount table
// even though every write to it fails with EROFS. Measured both ways on the test
// disk - the snapshot boot's journal is full of "Read-only file system" while
// systemd called the same root read-write, and a probe unit with that condition
// was skipped on a mount whose writes failed. A generator is what systemd itself
// uses to turn runtime state into mount units (systemd-fstab-generator,
// systemd-gpt-auto-generator), it runs before any unit is loaded, so what it
// emits lands in the same window as a fstab mount: before local-fs.target, and
// therefore before systemd-logind.
//
// And why the detector is the subvolume and not a write probe, which is what this
// had before a real boot disagreed with it: during the generator phase of an
// *ordinary* boot the root is mounted read-only too (measured: mountinfo says
// `0:32 /@/.snapshots/1/snapshot / ro,relatime … - btrfs /dev/vda2 rw,…`, and
// systemd-remount-fs makes it writable afterwards), so "can anything be written
// here" answers no in every boot. What does separate them is the definition of
// the feature - a snapshot boot is a root that is not the subvolume the system
// normally boots - and both halves of that are readable at generator time:
// mountinfo's entry for / names the subvolume the root is on, and `btrfs
// subvolume get-default` names the pointer a rollback moves. Same id in an
// ordinary boot, different id in a snapshot.
//
// Why not a separate @var subvolume, which is what SUSE does: /var/lib/pacman is
// the package database. A root rollback that left it current would make pacman
// claim the packages the rollback just removed are installed.

const (
	// systemdGeneratorDir is where a generator lives. systemd runs every
	// executable in it at the start of every boot, with the directory to write
	// units into as its first argument; a generator is not enabled, it is found.
	systemdGeneratorDir = "/usr/lib/systemd/system-generators/"

	// snapshotStateGeneratorPath is the generator itself. Like the menu generator
	// it is a shell script in /usr/local-adjacent territory owned by no package:
	// a systemd or snapper upgrade can never remove it.
	snapshotStateGeneratorPath = systemdGeneratorDir + "aelum-snapshot-state"

	// scriptMode: both shell files the installer writes are executed, and
	// mkarchiso-style copies without exec bits are a failure mode this project has
	// already hit.
	snapshotStateScriptMode = 0o755
)

// snapshotStateDirs are the directories a snapshot boot must be able to write to
// and cannot, because they are inside the root. A generator emits one tmpfs mount
// per directory, so the list is the whole feature: add a directory here and a
// snapshot boot gets it back as transient state.
var snapshotStateDirs = []string{
	// systemd-logind's execution and state directories (sessions, users, inhibit)
	// are here. Without logind there is no greeter, only getty@tty1.
	"/var/lib/systemd",
	// The DMS greeter's own state directory, which tmpfiles.d creates and chmods
	// at boot: with tmpfiles failing on it the greeter has nowhere to write.
	"/var/lib/greeter",
}

// snapshotStateUnitName is systemd's own escaping of a mount path into a mount
// unit name: the path without its leading "/", each remaining "/" replaced by
// "-", plus the ".mount" suffix. A mount unit whose name does not match its
// Where= is refused, so the name is derived from the path rather than written
// next to it.
func snapshotStateUnitName(where string) string {
	return strings.ReplaceAll(strings.TrimPrefix(where, "/"), "/", "-") + ".mount"
}

// snapshotStateGenerator is the generator installed into the target. It is a
// systemd generator, so it is run by systemd at the start of every boot - in a
// snapshot too, because it lives inside the root and a snapshot boots the
// generators the snapshot contains.
var snapshotStateGenerator = `#!/bin/sh
# SPDX-License-Identifier: GPL-3.0
#
# aelum-snapshot-state - give a snapshot boot the state directories it needs.
#
# A snapshot entry boots a read-only btrfs subvolume as /. Everything inside the
# root is read-only with it, and systemd-logind keeps its state directories
# inside the root: on such a boot logind fails at STATE_DIRECTORY, retries to its
# start limit and the system comes up at a text login instead of the greeter.
# This generator gives those directories a tmpfs - transient state that should
# not survive anything, and that must not be written into the snapshot.
#
# How it knows this is a snapshot boot. Not by asking whether the root can be
# written to: measured in a real boot, an ordinary boot has its root mounted
# read-only for the whole generator phase too (mountinfo: "0:32
# /@/.snapshots/1/snapshot / ro,relatime … - btrfs /dev/vda2 rw,…"), and it is
# made writable later by systemd-remount-fs. A writability probe therefore answers
# the same question in every boot. What separates them is the definition of the
# feature: a snapshot boot is a root that is not the subvolume the system normally
# boots. mountinfo's entry for / names the subvolume the root is on, and
# "btrfs subvolume get-default" names the pointer a rollback moves - the same
# subvolume id in an ordinary boot, a different one in a snapshot. Both are
# readable at generator time.
#
# It is a generator rather than a unit with a condition because the obvious
# condition is wrong too: ConditionPathIsReadWrite=!/ is unmet in a real snapshot
# boot, because a btrfs read-only subvolume mounted with rw looks read-write to
# the mount table even though every write into it fails with EROFS. A generator is
# what systemd itself uses to turn runtime state into mount units, and it runs
# before units are loaded, so what it emits lands in the same window as a fstab
# mount: before local-fs.target, and therefore before systemd-logind.
#
# Two shell notes, both learned from the same real boot. systemd calls a generator
# with three arguments, so every positional read here has a default: a bare $2
# under "set -u" fails the script on every real boot. And a failed redirection on
# a *special builtin* - ": > file" - exits a non-interactive shell, which is how
# this script used to die with status 1 before writing anything.

set -u

# systemd calls a generator with the directory to write units into as its first
# argument. With nowhere to write there is nothing to do, and a generator that
# fails must not fail the boot.
gen_dir=${1:-}
[ -n "$gen_dir" ] && [ -d "$gen_dir" ] || exit 0

# The two facts the decision needs, and the seam that lets them be tested without
# a filesystem: AELUM_MOUNTINFO names a mountinfo file to read instead of the
# running one, AELUM_GET_DEFAULT a command to run instead of the one below. Both
# are where the information comes from, not what the answer is - the parsing is
# still done by this script, which is where it once went wrong.
mountinfo=${AELUM_MOUNTINFO:-/proc/self/mountinfo}
get_default=${AELUM_GET_DEFAULT:-btrfs subvolume get-default /}

# The subvolume the running root is on. mountinfo's entry for / carries it twice -
# as its mount root, and as subvolid= inside the comma-separated super options -
# and the id is the value the other lookup returns, so the id is what is compared.
# The sub options are one field ("rw,discard=async,subvolid=270,subvol=/@/..."),
# so subvolid= has to be found inside it rather than assumed to start one.
root_subvol=$(awk '$5 == "/" && match($0, /subvolid=[0-9]+/) {
    print substr($0, RSTART + 9, RLENGTH - 9); exit
}' "$mountinfo" 2>/dev/null)

# The subvolume the system boots by default: the pointer a rollback moves, and the
# one an ordinary boot is on.
default_subvol=$($get_default 2>/dev/null |
    awk '{ for (i = 1; i < NF; i++) if ($i == "ID") { print $(i + 1); exit } }')

# Anything unknown is treated as an ordinary boot. The worst this can do is leave a
# snapshot boot without its tmpfs; guessing the other way would hide a real state
# directory behind a tmpfs in a normal boot.
[ -n "$root_subvol" ] || exit 0
[ -n "$default_subvol" ] || exit 0
[ "$root_subvol" != "$default_subvol" ] || exit 0

# A mount unit must be named after the path it mounts, each "/" replaced by "-"
# and the ".mount" suffix added.
unit_name() {
    printf '%s.mount' "${1#/}" | tr '/' '-'
}

emit_mount() { # <where>
    where=$1
    name=$(unit_name "$where")
    {
        echo "# Generated by aelum-snapshot-state for this boot. Do not edit by hand."
        echo "# $where is inside a root that cannot be written to, and something has"
        echo "# to be able to write to it. A tmpfs: it is gone at the next boot, and"
        echo "# nothing is ever written into the snapshot - which is what makes a"
        echo "# snapshot boot something you can reboot out of having lost nothing."
        echo ""
        echo "[Unit]"
        echo "Description=Aelum: writable $where for a snapshot boot"
        # The ordering systemd gives a local fstab mount (systemd.mount(5): After=
        # local-fs-pre.target, Before=local-fs.target), spelled out here because a
        # generated unit is read exactly as written, plus the one unit this exists
        # for.
        echo "After=local-fs-pre.target"
        echo "Before=local-fs.target systemd-logind.service"
        echo ""
        echo "[Mount]"
        echo "What=tmpfs"
        echo "Where=$where"
        echo "Type=tmpfs"
        # X-mount.mkdir so the mount point exists even in a snapshot that predates
        # the directory: mount(8) can create it, nothing else on a read-only root
        # can.
        echo "Options=defaults,X-mount.mkdir"
    } > "$gen_dir/$name"

    # Writing the unit is only half of it. A mount unit runs when something
    # requires it, and nothing requires a unit nobody has heard of. This is how
    # systemd's own generators do it: systemd-fstab-generator writes the .mount
    # and a link from local-fs.target.requires/, which is what makes the mount
    # happen on the way to local-fs.target.
    #
    # Measured on a real snapshot boot before this line existed:
    # /var/lib/systemd came up anyway - systemd-logind, systemd-rfkill,
    # systemd-timesyncd, shadow.timer and five others have RequiresMountsFor= on
    # it - while /var/lib/greeter, which nobody asks for, sat inactive and
    # systemd-tmpfiles logged "fchmod() of /var/lib/greeter failed: Read-only
    # file system". The half of this feature that nobody else happens to need
    # was the half that did not run.
    mkdir -p "$gen_dir/local-fs.target.requires" 2>/dev/null
    ln -sf "../$name" "$gen_dir/local-fs.target.requires/$name" 2>/dev/null
}

for where in ` + strings.Join(snapshotStateDirs, " ") + `; do
    emit_mount "$where"
done

exit 0
`

// configureSnapshotStateDirs installs the snapshot-boot state generator.
//
// It is btrfs-only for the same reason the rest of the snapshot machinery is (an
// ext4 install has no snapshot to boot), and it is not fatal: with no generator
// the system still boots its own live root, which is writable. It is called
// before the "after installation" snapshot on purpose - a snapshot boots the
// generators and units the snapshot contains, so anything a snapshot boot needs
// has to be inside it.
func configureSnapshotStateDirs(ctx *shared.InstallContext) error {
	if err := primitives.Run("mkdir", "-p", shared.MountRoot+systemdGeneratorDir).Run(); err != nil {
		return fmt.Errorf("failed to create %s: %w", systemdGeneratorDir, err)
	}
	if err := primitives.WriteFileMode(shared.MountRoot+snapshotStateGeneratorPath, snapshotStateGenerator, snapshotStateScriptMode); err != nil {
		return fmt.Errorf("failed to write %s: %w", snapshotStateGeneratorPath, err)
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("Snapshot boot state generator installed: a snapshot comes up at the greeter, not a text login")
	}
	return nil
}

// snapshotStateUnitNames returns the mount unit names the generator writes, for
// tests and for the fixture.
func snapshotStateUnitNames() []string {
	names := make([]string, 0, len(snapshotStateDirs))
	for _, where := range snapshotStateDirs {
		names = append(names, snapshotStateUnitName(where))
	}
	return names
}

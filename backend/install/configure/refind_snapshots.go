package configure

import (
	"fmt"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// Snapshot boot entries (rEFInd + snapper)
// ============================================================================
//
// rEFInd has no notion of a btrfs snapshot: every entry it shows is a menu
// stanza in refind.conf. The snapshots snapper takes for @ (see snapper.go) are
// therefore only bootable if something turns them into stanzas.
//
// Three pieces do that, all installed into the target by the configure step and
// all btrfs-only, because an ext4 install has no snapshot to list:
//
//   /usr/local/bin/aelum-refind-snapshots       writes the stanzas
//   /etc/systemd/system/aelum-refind-snapshots.service   runs it at boot
//   /usr/lib/snapper/plugins/10-aelum-…         runs it after every snapshot
//
// They write one file: this installation's snapshot menu fragment, which the
// installer creates empty (so refind.conf's include list covers it) and the
// installed system keeps current. The installer's own fragment is never touched.

const (
	// snapshotScriptPath is the generator, in the installed system. /usr/local
	// is the hierarchy no package writes to, so a snapper or rEFInd upgrade can
	// never remove it.
	snapshotScriptPath = "/usr/local/bin/aelum-refind-snapshots"

	// snapshotUnitName is the oneshot that runs the generator. It is enabled, so
	// the menu is rebuilt at every boot whatever else happened in between.
	snapshotUnitName = "aelum-refind-snapshots.service"
	snapshotUnitPath = "/etc/systemd/system/" + snapshotUnitName

	// snapperPluginDir + Path hook the generator into snapper itself. snapper(8)
	// runs every executable in this directory, passing the action it performed as
	// the first argument; the leading digit is snapper's requirement, not a
	// preference. snapper ships no plugins, so this directory is ours.
	snapperPluginDir  = "/usr/lib/snapper/plugins"
	snapperPluginPath = snapperPluginDir + "/10-aelum-refind-snapshots"

	// scriptMode: both shell files are executed, and mkarchiso-style copies
	// without exec bits are exactly the failure mode this project has hit before.
	scriptMode = 0o755
)

// configureSnapshotBootEntries installs the snapshot menu machinery into the
// target. Like the snapper configs it is best-effort in one direction only: a
// system without snapshot boot entries still installs and boots from its live
// subvolume, so a unit that cannot be enabled is a warning, while the files
// themselves are written through the normal error path (a half-installed
// generator is worse than none).
func configureSnapshotBootEntries(ctx *shared.InstallContext) error {
	if err := primitives.Run("mkdir", "-p", shared.MountRoot+snapperPluginDir).Run(); err != nil {
		return fmt.Errorf("failed to create %s: %w", snapperPluginDir, err)
	}
	if err := primitives.WriteFileMode(shared.MountRoot+snapshotScriptPath, snapshotMenuScript, scriptMode); err != nil {
		return fmt.Errorf("failed to write %s: %w", snapshotScriptPath, err)
	}
	if err := primitives.WriteFileMode(shared.MountRoot+snapperPluginPath, snapperSnapshotPlugin, scriptMode); err != nil {
		return fmt.Errorf("failed to write %s: %w", snapperPluginPath, err)
	}
	if err := primitives.WriteFile(shared.MountRoot+snapshotUnitPath, snapshotMenuUnit); err != nil {
		return fmt.Errorf("failed to write %s: %w", snapshotUnitPath, err)
	}

	if out, err := primitives.RunChroot("systemctl", "enable", snapshotUnitName); err != nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("Warning: failed to enable %s (non-fatal): %v: %s",
				snapshotUnitName, err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("Installed %s: snapper snapshots are selectable in the boot menu", snapshotUnitName)
	}
	return nil
}

// writeInitialSnapshotMenu runs the generator the installer just installed against
// the system that is still being installed. It is not a chroot: the script is run
// from the live system with --root pointing at the target, which is how it sees the
// target's ESP, snapper config and history through the mount table the installer
// already has open (a chroot would show it the live system's mounts instead).
//
// Why it belongs to the installer at all: rEFInd reads its configuration before any
// part of the installed system runs. A menu whose first write happens at boot is
// therefore one boot behind the menu that boot could have used - the install's own
// first boot shows no snapshot entry, and only the second one does. By the time
// this runs the layout has both snapshots it guarantees (#1, which is the system,
// and "after installation"), so the first boot has something to show.
//
// It is not fatal in either direction. The placeholder written above is a valid
// file, and the installed system rebuilds the menu at every boot, so a failed run
// here costs a boot menu, not an install.
func writeInitialSnapshotMenu(ctx *shared.InstallContext) error {
	script := shared.MountRoot + snapshotScriptPath
	out, err := primitives.Run(script, "--root", shared.MountRoot).CombinedOutput()
	if err != nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("Warning: the snapshot boot menu was not written here; the installed system writes it at its first boot: %v: %s",
				err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("Snapshot boot menu written: previous states are selectable at this install's first boot")
	}
	return nil
}

// refindSnapshotPlaceholder is the snapshot menu fragment as the installer writes
// it. It is empty on purpose: refind.conf can only include a file that is there,
// so the file has to exist before the first snapshot ever is, and it is what a
// boot falls back on if the generator has not run yet (see writeInitialSnapshotMenu
// for the normal case, where the install fills it before its own first boot).
func refindSnapshotPlaceholder() string {
	return `# Generated by the Aelum installer. Do not edit by hand.
# This file holds this installation's snapshot boot entries. The installer writes
# it empty (no snapshot exists yet); the installed system fills it from the
# snapshots snapper keeps for @, at every boot and after every snapshot:
#     systemctl start ` + snapshotUnitName + `
`
}

// ============================================================================
// The generator installed into the target
// ============================================================================

const snapshotMenuScript = `#!/bin/sh
# SPDX-License-Identifier: GPL-3.0
#
# aelum-refind-snapshots - put this system's snapshots in the Aelum boot menu.
#
# rEFInd shows exactly the menu stanzas its configuration contains, so a snapshot
# is only bootable once it has a stanza. Snapper keeps dozens of snapshots, and
# a row of icons that size is not a menu anyone can read, so this writes ONE
# entry into this installation's snapshot menu fragment
# (EFI/refind/aelum/<id>-snapshots.conf, which refind.conf already includes)
# and lists the snapshots inside it as submenuentries: rEFInd shows those on F2,
# Insert, Tab or +.
#
# It runs at every boot (aelum-refind-snapshots.service), after every snapshot
# snapper creates or deletes (its plugin in /usr/lib/snapper/plugins), and once
# while the system is being installed (aelum-refind-snapshots --root <mount>,
# see below) so that the first boot already has a menu. When it runs inside a
# snapshot boot it also writes that snapshot's description and date into the
# restore prompt's marker, so the prompt can say what the booted state was for
# (see "What this boot is").
#
# What an entry boots is this installation's current kernel and initramfs - they
# live on the ESP, outside every snapshot - with the snapshot as root. Snapper's
# snapshots are read-only subvolumes, so the system comes up with / read-only:
# enough to log in, compare and copy files out. Making one the system again is
# "snapper rollback" (or btrfs-assistant): the live system is the filesystem's
# default subvolume, and a rollback points that at a writable copy of the snapshot -
# which is why this installation's own entry names no subvolume and needs no change
# after one. /home, /srv and /var/* are separate subvolumes and stay exactly as they
# are.

set -u

# The submenu is a list the user asked for, so it can hold more than the main
# menu should: still only a fraction of what snapper keeps, and the states worth
# booting are the recent ones. The states someone had a reason to keep come first
# (see the ranking below), so this cap never becomes a wall of timestamps.
MAX_SNAPSHOTS=8

# The config snapper.go writes for @. Only a root subvolume is bootable, so the
# @home config is not this script's business.
SNAPPER_CONFIG="root"

# The icon that says "history", not "another system": the history glyph of the
# Aelum rEFInd theme, on the same cream tile as the Aelum mark.
SNAPSHOT_ICON="/` + shared.RefindSnapshotIconPath + `"

fail() { echo "aelum-refind-snapshots: $*" >&2; exit 1; }

# --- Which system this writes for -------------------------------------------
# The running one, unless told otherwise. The service and snapper's plugin run it
# from inside the installed system; the installer runs it against the system it is
# still installing:
#
#     aelum-refind-snapshots --root /mnt
#
# That call is what makes an install's own first boot show its snapshot menu.
# rEFInd reads its configuration before any part of the installed system runs, so
# a menu first written at boot is one boot behind the menu it could have booted.
# Everything below is resolved under this root: the ESP, the snapper config, the
# history. The one lookup that is not a path is the default subvolume, and btrfs
# answers that for any mount of the filesystem, which is why the same script reads
# the live system, a booted snapshot and a system that is still being installed.
root="/"
case "${1-}" in
    --root)
        [ "$#" -ge 2 ] || fail "--root needs a path"
        root=$2
        shift 2
        ;;
    --root=*)
        root=${1#--root=}
        shift
        ;;
esac
[ -n "$root" ] || root="/"
# Strip a trailing slash ("/mnt/" -> "/mnt") without ever turning "/" into "".
# That is the common case, not an edge case: the service and snapper's plugin run
# this with no argument at all, and an empty root makes "btrfs subvolume
# get-default" fail - which used to write stanzas naming subvolumes that do not
# exist, i.e. a boot menu whose entries cannot mount a root.
case $root in
    "/") ;;
    */) root=${root%/} ;;
esac

# --- Where the menu lives ---------------------------------------------------
# The ESP has to be mounted for anything to be written. The paths the installer's
# own fstab uses come first; the scan covers a user who moved the ESP. Mount
# targets containing spaces are not supported (they are not what fstab generates).
esp=""
for candidate in "$root/efi" "$root/boot"; do
    [ -d "$candidate/EFI/refind/aelum" ] && esp=$candidate && break
done
if [ -z "$esp" ]; then
    for candidate in $(findmnt -nrlo TARGET -t vfat 2>/dev/null); do
        [ -d "$candidate/EFI/refind/aelum" ] && esp=$candidate && break
    done
fi
[ -n "$esp" ] || fail "no mounted ESP with EFI/refind/aelum"

# The installation is named by the first eight hex characters of its root
# filesystem UUID - the same derivation the installer uses for every per-install
# file, including the menu entry whose options are copied below.
root_uuid=$(findmnt -no UUID "$root" 2>/dev/null | head -n 1)
if [ -z "$root_uuid" ]; then
    root_source=$(findmnt -no SOURCE "$root" 2>/dev/null | head -n 1)
    root_source=${root_source%%\[*}
    [ -n "$root_source" ] && root_uuid=$(blkid -s UUID -o value "$root_source" 2>/dev/null | head -n 1)
fi
[ -n "$root_uuid" ] || fail "cannot read the root filesystem UUID"
install_id=$(printf '%s' "$root_uuid" | tr 'A-Z' 'a-z' | tr -cd 'a-z0-9' | cut -c1-8)

base_entry="$esp/EFI/refind/aelum/$install_id.conf"
out="$esp/EFI/refind/aelum/$install_id-snapshots.conf"
[ -r "$base_entry" ] || fail "no menu entry to copy boot options from: $base_entry"
# The loader, initramfs and full kernel command line come from this installation's
# own menu entry. Copying them is what keeps LUKS (rd.luks.name=) and every
# future option correct without this script knowing anything about them; the only
# thing a snapshot entry changes is which subvolume is root.
base_loader=$(sed -n 's/^[[:space:]]*loader[[:space:]]\{1,\}\([^[:space:]]*\).*/\1/p' "$base_entry" | head -n 1)
base_initrd=$(sed -n 's/^[[:space:]]*initrd[[:space:]]\{1,\}\([^[:space:]]*\).*/\1/p' "$base_entry" | head -n 1)
base_options=$(sed -n 's/^[[:space:]]*options[[:space:]]\{1,\}"\(.*\)"[[:space:]]*$/\1/p' "$base_entry" | head -n 1)
[ -n "$base_loader" ] && [ -n "$base_options" ] || fail "menu entry has no loader/options: $base_entry"

# --- Where the snapshots live -----------------------------------------------
# Two spellings of the same place, and they are not interchangeable:
# snapshot_dir is where this running system can read the snapshots (a mount point
# under /), and subvol_base is the path rootflags= wants - from the top level of
# the filesystem. snapper's own config says which mount point it snapshots.
snapshot_dir=$(sed -n 's/^SUBVOLUME="\([^"]*\)".*/\1/p' "$root/etc/snapper/configs/$SNAPPER_CONFIG" 2>/dev/null | head -n 1)
[ -n "$snapshot_dir" ] || snapshot_dir=/
snapshot_dir="$root${snapshot_dir%/}/` + shared.BtrfsSnapshotsDirName + `"

# The history is a mount, not a directory inside the root. The live root is a
# snapshot, and a snapshot does not contain the subvolumes that were inside its
# parent, so the only .snapshots this system can see is the mounted one. If it is
# not mounted there is nothing correct to write here, and rewriting the fragment
# from an empty list would delete the entries that made this boot possible: leave the
# menu exactly as it is.
if ! findmnt -n -o SOURCE "$snapshot_dir" >/dev/null 2>&1; then
    echo "aelum-refind-snapshots: $snapshot_dir is not mounted; leaving the boot menu as it is" >&2
    exit 0
fi

# The path of the history from the top level of the filesystem. What the system is
# is named by the default subvolume - that is the pointer a rollback moves - so the
# container the history lives in is its parent. btrfs reports these paths from any
# mount of the filesystem, so this reads the same from the live system and from a
# snapshot.
default_path=$(btrfs subvolume get-default "$root" 2>/dev/null | sed -n 's/^ID [0-9]* gen [0-9]* top level [0-9]* path //p' | head -n 1)
case "$default_path" in
    # The live root is itself a snapshot: the history is in the container above it.
    */` + shared.BtrfsSnapshotsDirName + `/*) container=${default_path%/` + shared.BtrfsSnapshotsDirName + `/*} ;;
    # The history is at the top level of the filesystem and the default subvolume is
    # inside it: a hand-built disk, or a set-default on the tree root. The arm above
    # does not match, and falling through would make the container the snapshot
    # itself - a history named ".snapshots/1/snapshot/.snapshots", which is in no
    # tree.
    ` + shared.BtrfsSnapshotsDirName + `/*) container="" ;;
    # Nothing to derive from: btrfs did not answer. Without the default subvolume
    # there is no correct path to name a snapshot by, and a fragment that names a
    # subvolume that does not exist is worse than the one already on the ESP.
    "") echo "aelum-refind-snapshots: cannot read the default subvolume; leaving the boot menu as it is" >&2; exit 0 ;;
    # A plain subvolume is the default (a layout that was never rolled back).
    *) container=$default_path ;;
esac
if [ -n "$container" ]; then
    subvol_base="$container/` + shared.BtrfsSnapshotsDirName + `"
else
    subvol_base="` + shared.BtrfsSnapshotsDirName + `"
fi

# --- How a snapshot is named in the menu ------------------------------------
# A title is the snapshot's number, its date and its type - the columns of
# snapper's own list, in the same order, minus the cleanup column - and the
# date spelled the way that list spells it ("10/31/26 9:35 AM"), so a menu
# title and a snapper row are recognisably the same snapshot. That is a
# deliberately bounded shape, because rEFInd's text renderer neither wraps nor
# ellipsizes: a line it cannot fit is cut at the screen edge - or, for the
# selection text under the icons, dropped entirely. Measured at 1280x720, a
# 60-character title ("Aelum snapshot 3 - rollback backup of #1 (31 Oct 2026
# 11:07)") lost its selection text in the main menu and lost its date in the
# submenu header; this shape stays in its mid-40s even for the longest
# spelling. The number is what snapper and btrfs-assistant name a snapshot by
# anyway, the date is what a person scans a history for, and the type says
# which half of a transaction the state is; what a snapshot is *for* is said
# by the restore prompt - a desktop window, where any language and any length
# renders (see "What this boot is").
#
# rEFInd tokenizes its config and draws it with a bitmap font covering ASCII
# 32-126, so the text it is given must be one line of printable ASCII. The
# titles are authored here and meet that by construction; menu_text is the
# guard on the one title part that comes from elsewhere - the date read out
# of info.xml - and on '|' , this script's own field separator (see the
# collected list below), which no field may contain.
menu_text() {
    printf '%s' "$1" | tr -cd ' -~' | tr -d '|' | tr -s ' ' | sed -e 's/^ *//' -e 's/ *$//'
}

human_date() {
    # info.xml writes "YYYY-MM-DD HH:MM:SS". The menu spells the date the way
    # snapper's own list does - short date plus 12-hour clock with an unpadded
    # hour, "10/31/26 9:35 AM" - so a menu title and a snapper row name the
    # same state in the same words. Anything that is not the info.xml shape is
    # passed through rather than guessed at.
    case "$1" in
        [0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]*) ;;
        *) printf '%s' "$1"; return 0 ;;
    esac
    year=$(printf '%s' "$1" | cut -c3-4)
    month=$(printf '%s' "$1" | cut -c6-7)
    day=$(printf '%s' "$1" | cut -c9-10)
    clock=$(printf '%s' "$1" | cut -c12-16)
    # 24-hour to 12-hour. Stripping one leading zero keeps the arithmetic
    # decimal ("09" -> 9, "00" -> ""; "10" and "11" keep both digits); 0 and 12
    # both read as 12, and the half-day names the rest.
    hour=$(printf '%s' "$clock" | cut -c1-2 | sed -e 's/^0//')
    [ -n "$hour" ] || hour=0
    if [ "$hour" -ge 12 ]; then half=PM; else half=AM; fi
    hour=$((hour % 12))
    [ "$hour" -eq 0 ] && hour=12
    printf '%s/%s/%s %s:%s %s' "$month" "$day" "$year" "$hour" "$(printf '%s' "$clock" | cut -c4-5)" "$half"
}

# --- What this boot is ------------------------------------------------------
# The restore prompt (aelum-snapshot-prompt) asks its question out of the marker
# /run/aelum-snapshot-boot, which the aelum-snapshot-state generator writes with
# the number of the snapshot this root is on. The menu names a snapshot by its
# number, date and type - words short enough for rEFInd to draw on any screen -
# so what the state is *for* has to reach the person from here: the snapshot's own
# description, unaltered, or the words its type implies when it has none. This
# is the one place both halves exist: the history is mounted, and info.xml is
# readable. The prompt renders those words as they are, in any language.

# What a snapshot is *for*, in full and in whatever language it was written. A
# description written by a person or by btrfs-assistant is the best label there
# is, so it is used verbatim - the prompt is a desktop window, not a firmware
# menu, and it is not limited to ASCII or to one line's width. With none,
# snapper's own type says it: a pre snapshot is the state before a transaction,
# a post snapshot the state after one. "pacman" is this installer's own tag,
# not something a person would read, so it is replaced by the words the type
# implies. Only newlines are cut, because the marker is an EnvironmentFile.
snapshot_phrase() {
    phrase=$(printf '%s' "$2" | tr -d '\n\r' | sed -e 's/^ *//' -e 's/ *$//')
    case "$phrase" in pacman) phrase="" ;; esac
    if [ -z "$phrase" ]; then
        case "$1" in
            pre) phrase="Before system update" ;;
            post) phrase="After system update" ;;
        esac
    fi
    printf '%s' "$phrase"
}

# Only a run from inside this boot enriches it: with --root the history belongs
# to a system that is not the one running, and neither the live ISO nor the
# installer has a marker at all. An unreadable info.xml leaves the marker exactly
# as the generator wrote it - the number is the fact, the words are detail.
marker=${AELUM_SNAPSHOT_MARKER:-` + shared.SnapshotBootMarkerPath + `}
if [ "$root" = "/" ] && [ -r "$marker" ]; then
    boot_number=$(sed -n 's/^SNAPSHOT=//p' "$marker" | head -n 1)
    case "$boot_number" in ""|*[!0-9]*) boot_number="" ;; esac
    boot_info="$snapshot_dir/$boot_number/info.xml"
    if [ -n "$boot_number" ] && [ -r "$boot_info" ]; then
        boot_type=$(sed -n 's/.*<type>\([^<]*\)<\/type>.*/\1/p' "$boot_info" 2>/dev/null | head -n 1)
        boot_date=$(sed -n 's/.*<date>\([^<]*\)<\/date>.*/\1/p' "$boot_info" 2>/dev/null | head -n 1)
        boot_desc=$(sed -n 's/.*<description>\([^<]*\)<\/description>.*/\1/p' "$boot_info" 2>/dev/null | head -n 1)
        case "$boot_type" in pre|post) ;; *) boot_type="single" ;; esac
        boot_phrase=$(snapshot_phrase "$boot_type" "$boot_desc")
        boot_stamp=$(human_date "$boot_date")
        quote() { printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g'; }
        # The prompt unit reads the marker as an EnvironmentFile, so free text
        # goes in quoted, with the two characters that quoting would otherwise
        # eat. The whole file is rewritten, so a second run (snapper's plugin
        # starts this service after every snapshot change) cannot accumulate
        # duplicate lines.
        marker_tmp="$marker.tmp"
        {
            printf 'SNAPSHOT=%s\n' "$boot_number"
            if [ -n "$boot_phrase" ]; then printf 'SNAPSHOT_PHRASE="%s"\n' "$(quote "$boot_phrase")"; fi
            if [ -n "$boot_stamp" ]; then printf 'SNAPSHOT_DATE="%s"\n' "$(quote "$boot_stamp")"; fi
        } > "$marker_tmp" && mv -f "$marker_tmp" "$marker" || {
            rm -f "$marker_tmp"
            echo "aelum-refind-snapshots: could not enrich $marker; the prompt will ask with the number alone" >&2
        }
    fi
fi

# --- Which snapshots to offer -----------------------------------------------
# Snapshot numbers are directory names. info.xml is read directly rather than
# asking snapper, so this works with snapperd stopped and costs no filesystem
# walk beyond the snapshot directory.
#
# Each collected line is
#
#     rank|number|type|cleanup|important|submenu title|entry title
#
# Both titles name the same snapshot in the two shapes a menu needs: plain
# inside the submenu, qualified as the entry that boots it. They carry the
# number, the date and the type and no free text (see "How a snapshot is named
# in the menu"), so every field is authored here; '|' still separates them, and
# menu_text keeps the one field read from info.xml - the date - free of it.
#
# rank is what a snapshot is worth *listing*: 0 for a state a transaction or a
# person created (a pre/post pair, or one marked important=yes), 1 for the rest.
# Snapper's timeline timer takes a snapshot every hour and keeps a dozen of them,
# so a plain newest-N list is all timestamps and no decisions: the states someone
# had a reason to keep are collected first, and the automatic history fills
# whatever room is left.
#
# The snapshot that *is* this system is not offered at all. The default subvolume
# is what this install's own entry boots - that entry names no subvolume, so it
# follows the pointer - and a rollback moves it, so listing it would offer the
# state the user is already in as a state to go back to.
default_number=""
case "$default_path" in
    */` + shared.BtrfsSnapshotsDirName + `/*)
        default_number=${default_path##*/` + shared.BtrfsSnapshotsDirName + `/}
        default_number=${default_number%%/snapshot}
        ;;
    # The history at the filesystem tree root (see the container case above): the
    # same number, with no "/" to cut on.
    ` + shared.BtrfsSnapshotsDirName + `/*)
        default_number=${default_path##*` + shared.BtrfsSnapshotsDirName + `/}
        default_number=${default_number%%/snapshot}
        ;;
esac
case "$default_number" in ""|*[!0-9]*) default_number="" ;; esac

list=$(
    if [ -d "$snapshot_dir" ]; then
        for dir in "$snapshot_dir"/[0-9]*; do
            [ -d "$dir/snapshot" ] || continue
            number=${dir##*/}
            case "$number" in ""|*[!0-9]*) continue ;; esac
            if [ "$number" = "$default_number" ]; then
                continue
            fi

            info="$dir/info.xml"
            type=$(sed -n 's/.*<type>\([^<]*\)<\/type>.*/\1/p' "$info" 2>/dev/null | head -n 1)
            date=$(sed -n 's/.*<date>\([^<]*\)<\/date>.*/\1/p' "$info" 2>/dev/null | head -n 1)
            # The description is deliberately not read here. It is free text in
            # any language, and a menu title is not where it can be shown (see
            # "How a snapshot is named in the menu"); it reaches the person
            # through the restore prompt instead.
            # <cleanup> names the algorithm that owns a snapshot: "number" for
            # transaction pairs and rollbacks, "timeline" for the automatic
            # history. Userdata is a list of <key>/<value> pairs, which is what
            # snapper writes (measured on a real install's "after installation"
            # snapshot):
            #
            #   <userdata>
            #     <key>important</key>
            #     <value>yes</value>
            #   </userdata>
            #
            # so the pair is read from the flattened document - it spans two lines
            # in the file. The flat <important>yes</important> spelling is accepted
            # too, so a snapshot is not skipped over because of a version skew.
            cleanup=$(sed -n 's/.*<cleanup>\([^<]*\)<\/cleanup>.*/\1/p' "$info" 2>/dev/null | head -n 1)
            important=$(tr -d '\n\r' < "$info" 2>/dev/null | sed -nE \
                -e 's|.*<key>important</key>[[:space:]]*<value>([^<]*)</value>.*|\1|p' \
                -e 's|.*<important>([^<]*)</important>.*|\1|p' | head -n 1)
            case "$type" in pre|post) ;; *) type="single" ;; esac

            rank=1
            if [ "$type" = pre ] || [ "$type" = post ] || [ "$important" = yes ]; then
                rank=0
            fi

            # Number, date and type - snapper's list columns without the cleanup
            # one. menu_text guards the date field (the only title part not
            # authored here) against anything rEFInd cannot tokenize or draw; the
            # type is already limited to pre|post|single above.
            stamp=$(menu_text "$(human_date "$date")")

            title="Snapshot $number"
            entry_title="Aelum snapshot $number"
            if [ -n "$stamp" ]; then
                title="$title - $stamp"
                entry_title="$entry_title - $stamp"
            fi
            title="$title - $type"
            entry_title="$entry_title - $type"
            printf '%s|%s|%s|%s|%s|%s|%s\n' \
                "$rank" "$number" "$type" "$cleanup" "$important" \
                "$title" "$entry_title"
        done
    fi \
        | sort -t'|' -k1,1n -k2,2rn \
        | head -n "$MAX_SNAPSHOTS" \
        | cut -d'|' -f2- \
        | sort -t'|' -k1,1rn
)

# The entry itself boots the state worth booting, in this order:
#
#   1. the newest snapshot taken *before* a change - when an update breaks the
#      system that is the state to go back to, while the newest snapshot is
#      usually the one that already contains the change;
#   2. the newest one marked important=yes - a rollback backup, or the "after
#      installation" state of a system that has not been updated yet;
#   3. the newest snapshot left.
#
# A timeline snapshot is never the entry: it is a timestamp, not a decision, and a
# timer that happened to fire at 03:00 is not a state anyone chose to return to.
# It stays in the submenu, which is exactly what the history is for.
#
# The choice is made inside the list, because a snapshot the menu does not list is
# a snapshot this entry could not boot either; the list is in descending number
# order, so the first snapshot seen at a rank is the newest one at that rank.
target_rank=""; target_number=""; target_type=""; target_title=""; target_entry_title=""
first_number=""; first_type=""; first_title=""; first_entry_title=""
while IFS='|' read -r number type cleanup important title entry_title; do
    [ -n "$number" ] || continue
    [ -n "$first_number" ] || {
        first_number=$number; first_type=$type; first_title=$title; first_entry_title=$entry_title
    }
    case "$cleanup" in timeline) continue ;; esac

    rank=2
    if [ "$type" = pre ]; then
        rank=0
    elif [ "$important" = yes ]; then
        rank=1
    fi
    if [ -z "$target_rank" ] || [ "$rank" -lt "$target_rank" ]; then
        target_rank=$rank; target_number=$number; target_type=$type
        target_title=$title; target_entry_title=$entry_title
    fi
done <<EOF
$list
EOF
if [ -z "$target_number" ]; then
    target_number=$first_number; target_type=$first_type
    target_title=$first_title; target_entry_title=$first_entry_title
fi

# The install's own options with the snapshot substituted for the live subvolume -
# nothing else changes, so a snapshot that was made writable (btrfs property set
# ... ro false) still boots the way it was mounted.
snapshot_options() {
    case "$base_options" in
        *rootflags=*)
            printf '%s' "$base_options" | sed "s|rootflags=[^ ]*|rootflags=subvol=$subvol_base/$1/snapshot|g"
            ;;
        *)
            printf '%s rootflags=subvol=%s/%s/snapshot' "$base_options" "$subvol_base" "$1"
            ;;
    esac
}

# --- Write the fragment -----------------------------------------------------
# Written to a temporary name and renamed, so rEFInd can never read a fragment
# that is half-written (it is not a .conf file, so it is not an entry itself).
# With no snapshots there is no entry at all: an icon that boots nothing is
# worse than no icon.
tmp="$out.tmp"
{
    echo "# Generated by aelum-refind-snapshots. Do not edit by hand."
    echo "# One menu entry for the snapshots of @ (snapper config $SNAPPER_CONFIG)."
    echo "# Enter boots the snapshot named in the title; F2, Insert, Tab or + lists"
    echo "# every snapshot offered here. Each boots this install's current kernel"
    echo "# with that snapshot as root."
    echo "# Snapper's snapshots are read-only, so the system comes up with / read-only."
    echo "# To make one the system again: snapper rollback, then reboot into the"
    echo "# Aelum entry. The snapshot that is the current system is not listed."
    echo "# Rebuild this file with: systemctl start aelum-refind-snapshots.service"
    if [ -n "$target_number" ]; then
        echo ""
        echo "# The entry boots the newest snapshot taken before a change, else the"
        echo "# newest one marked important, else the newest one - never a timeline"
        echo "# snapshot, which is a timestamp rather than a state anyone chose."
        echo "# snapshot $target_number $target_type"
        echo "menuentry \"$target_entry_title\" {"
        echo "    icon $SNAPSHOT_ICON"
        echo "    loader $base_loader"
        [ -n "$base_initrd" ] && echo "    initrd $base_initrd"
        echo "    options \"$(snapshot_options "$target_number")\""
        echo ""
        while IFS='|' read -r number type cleanup important title entry_title; do
            # A submenuentry inherits loader, initrd and icon from the entry it
            # sits in, so it only has to say which subvolume becomes root. The
            # entry's own snapshot is listed too: rEFInd adds its own line for
            # the entry at the top of the submenu, and "Boot using default
            # options" does not say that it is a snapshot.
            echo "    # snapshot $number $type"
            echo "    submenuentry \"$title\" {"
            echo "        options \"$(snapshot_options "$number")\""
            echo "    }"
        done <<EOF
$list
EOF
        echo "}"
    fi
} > "$tmp" || { rm -f "$tmp"; fail "cannot write $tmp"; }
mv -f "$tmp" "$out" || { rm -f "$tmp"; fail "cannot replace $out"; }

exit 0
`

// ============================================================================
// The snapper plugin
// ============================================================================

// snapperSnapshotPlugin hooks the generator into snapper itself. snapper(8)
// runs every executable in /usr/lib/snapper/plugins with the action it performed
// as the first argument, so a snapshot that was just created or deleted is
// reflected in the boot menu without a poll.
const snapperSnapshotPlugin = `#!/bin/sh
# SPDX-License-Identifier: GPL-3.0
#
# snapper plugin: keep the Aelum boot menu in step with snapper.
#
# snapper(8), PLUGINS: every executable in this directory runs with the action
# snapper just performed as the first argument. Snapshot changes are the only
# events the boot menu cares about, and that is more than creating and deleting: a
# transactional-ambit rollback creates no snapshots at all, it only calls
# setDefault, and "snapper modify" rewrites the description, the cleanup tag and
# the important= marking a menu title is built from. Leaving those out keeps a menu
# naming a state the system no longer is until the next boot.
#
# The work is handed to aelum-refind-snapshots.service rather than done here,
# for two reasons: the ESP is mounted so that only root can write it, and a
# user allowed on the config by ALLOW_GROUPS is not root. --no-block keeps
# snapper's own operation from waiting on a boot menu.

case "$1" in
    create-snapshot-post|delete-snapshot-post|set-default-snapshot-post|modify-snapshot-post|set-read-only-post) ;;
    *) exit 0 ;;
esac

# $2 is the subvolume the config snapshots. Only @ can be booted; the @home
# config's snapshots are restored by hand, never selected at boot.
[ "$2" = "/" ] || exit 0

systemctl start --no-block aelum-refind-snapshots.service >/dev/null 2>&1 || true
exit 0
`

// ============================================================================
// The systemd unit
// ============================================================================

const snapshotMenuUnit = `# SPDX-License-Identifier: GPL-3.0
#
# Rebuild rEFInd's snapshot boot entries for this system's snapshots.
#
# Enabled, so it runs at every boot: that is what makes the menu correct even if
# a snapshot was deleted by hand, a cleanup timer pruned the timeline, or a
# snapper plugin run failed. It is also what snapper's plugin in
# ` + snapperPluginDir + ` starts after every snapshot create or delete.
#
# The script it runs rewrites one generated file on the ESP and changes nothing
# else; see /usr/local/bin/aelum-refind-snapshots.

[Unit]
Description=Aelum snapshot boot entries (rEFInd menu for snapper snapshots)
Documentation=man:snapper(8)
After=local-fs.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/aelum-refind-snapshots

[Install]
WantedBy=multi-user.target
`

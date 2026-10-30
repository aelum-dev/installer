package configure

import (
	"fmt"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// Snapper: snapshots for the btrfs layout
// ============================================================================

// snapperTargets are the subvolumes the installed system snapshots.
//
// Only @ and @home get a config. The layout already keeps the noisy directories
// off the system subvolume: @cache, @tmp and @log are separate subvolumes, so a
// snapshot of @ never recurses into them, and none of them is rollback material
// anyway — no restore wants an old package cache, scratch file or journal record.
// @srv is server data whose backup belongs to whatever runs there.
// qgroups the two configs aggregate their snapshots into. Level-1 ids are the
// ones snapper reserves for itself (level 0 is per-subvolume and automatic), so
// 1/0 and 1/1 are free on a filesystem the installer just made. One group per
// config, because SPACE_LIMIT is a property of a config: with a shared group the
// root's cleanup would delete system restore points because @home snapshots got
// big. FREE_LIMIT is the filesystem's own free space and covers the shared-disk
// case whatever the group holds.
const (
	snapperRootQGroup = "1/0"
	snapperHomeQGroup = "1/1"
)

type snapperTarget struct {
	// configName is the name snapper and `snapper -c <name>` use.
	configName string
	// subvolume is the Aelum subvolume this config snapshots.
	subvolume string
	// mountPoint is where the installed system mounts it.
	mountPoint string
	// qgroup is the level-1 qgroup written into the config as QGROUP. Space-aware
	// cleanup refuses to run without one (Snapper::prepare_quota throws "qgroup not
	// set", Cleaner::is_quota_aware turns that into "no space limit").
	qgroup string
	// retention is this config's cleanup policy.
	retention snapperRetention
}

var snapperTargets = []snapperTarget{
	{
		configName: snapperRootConfig,
		subvolume:  "@",
		mountPoint: "/",
		qgroup:     snapperRootQGroup,
		retention: snapperRetention{
			numberLimit:          "2-10",
			numberLimitImportant: "4-10",
			timelineCreate:       snapperTimelineCreate,
			hourly:               "1-6",
			daily:                "1-7",
			weekly:               "1-4",
			monthly:              "1-6",
			yearly:               "1-2",
		},
	},
	{
		configName: "home",
		subvolume:  "@home",
		mountPoint: "/home",
		qgroup:     snapperHomeQGroup,
		retention: snapperRetention{
			numberLimit:          "2-5",
			numberLimitImportant: "2-5",
			timelineCreate:       snapperTimelineCreate,
			hourly:               "1-3",
			daily:                "1-5",
			weekly:               "1-2",
			monthly:              "1-2",
			yearly:               "1-1",
		},
	},
}

// snapperRetention is one config's cleanup policy.
//
// Every limit is a range (min-max), never a single number, and that is the
// difference between a bounded history and an unbounded one: snapper skips its
// space-aware path entirely when the limits are degenerated
// (client/cleanup.cc Cleaner::is_quota_aware → Parameters::is_degenerated,
// client/utils/Range.h is_degenerated → min == max). A range means "keep at
// least min even if the space limits are exceeded, at most max even if there is
// room", which is the behaviour SUSE documents (NUMBER_LIMIT="2-10",
// NUMBER_LIMIT_IMPORTANT="4-10") and the reason it is worth paying for qgroup
// accounting at all.
//
// The two limits are separate quotas: an important snapshot is kept under
// NUMBER_LIMIT_IMPORTANT and does not compete with ordinary ones, which is what
// lets a run of small unrelated updates evict their own history instead of the
// kernel update a rollback would go back to.
type snapperRetention struct {
	numberLimit          string
	numberLimitImportant string
	// timelineCreate is "yes" only for a config whose history is worth taking on
	// a timer. See snapperTimelineCreate.
	timelineCreate                         string
	hourly, daily, weekly, monthly, yearly string
}

// snapperTimelineCreate is the decision about timeline snapshots, made once and
// written into every config: off.
//
// openSUSE's own default setup is "timeline snapshots are enabled, except for the
// root file system", and for the root there is no better reason than the one
// SUSE has: the history of a system is the states someone had a reason to keep —
// the pre/post pair every package transaction takes, the snapshots a rollback
// leaves behind, the "after installation" state — not a timer that fired. The boot
// menu already ranks timeline snapshots last and never makes one the recovery
// entry, so keeping them costs qgroup accounting, comparison work and root space
// for the least useful entries there are.
//
// It also removes a caveat that has no good answer: a snapshot boot runs the
// timers the snapshot contains, so booting a snapshot to *look* adds snapshots to
// the history it was booted from. With the timeline off, a snapshot boot writes
// nothing into that history, which is what "reboot back, nothing was lost" is
// supposed to mean.
//
// The limits below stay ranges and snapper-timeline.timer stays enabled, so a
// user who turns TIMELINE_CREATE on (btrfs-assistant has the switch) gets a
// working, space-bounded timeline rather than a config that cannot clean up.
const snapperTimelineCreate = "no"

const snapperMinAge = "1800"

// snapperUnits are the units that make the timeline real. snapperd is the D-Bus
// service btrfs-assistant talks to, and the two timers are what create and clean
// timeline snapshots on a system with no cron daemon (Arch ships none).
var snapperUnits = []string{"snapperd", "snapper-timeline.timer", "snapper-cleanup.timer"}

// ============================================================================
// Pacman hooks: a snapshot around every transaction
// ============================================================================

// snapperRootConfig is the name ConfigureSnapper gives the config that snapshots
// @, i.e. the only config whose snapshots are roots (see
// configureSnapshotBootEntries). The others are restored by hand, never booted.
const snapperRootConfig = "root"

// pacmanSnapshotHooks are the three hooks of one pre/post pair taken around every
// pacman transaction. They are numbered so the pre snapshot runs before the
// transaction and the post snapshots after every other post-transaction hook
// (mkinitcpio regenerates this install's kernel directory at 90-, and the post
// snapshot belongs after the transaction is really finished); pacman runs hooks in
// filename order, so the important marking is the last of them.
const (
	pacmanPreHookPath  = "/etc/pacman.d/hooks/90-aelum-snapper-pre.hook"
	pacmanPostHookPath = "/etc/pacman.d/hooks/91-aelum-snapper-post.hook"
	// pacmanImportantHookPath is the third hook of the same pair: it marks the
	// pair important when the transaction changed the system rather than a package.
	pacmanImportantHookPath = "/etc/pacman.d/hooks/92-aelum-snapper-important-post.hook"

	// pacmanPreHookState carries the pre snapshot's number to the post hook:
	// snapper pairs pre and post by number, and one hook cannot talk to the next.
	// pacmanPairState carries both numbers to the important hook. /run is tmpfs, so
	// neither survives a reboot, and the pre hook clears both first: a transaction
	// killed between its pre and post hooks would otherwise leave a number for the
	// next one to pair with, and the pair would span two transactions.
	pacmanPreHookState = "/run/aelum-snapper/pacman.pre"
	pacmanPairState    = "/run/aelum-snapper/pacman.pair"
)

// pacmanImportantTargetList is the Target line of the third hook: the packages
// whose transactions change the state a rollback is *for*. The kernel and its
// initramfs, the C library and init system everything else is built against, the
// tools that write the boot and the filesystem a snapshot is made of, and the
// display drivers - "before I changed the GPU driver" is the example every
// snapshot doc uses. snapper keeps snapshots marked important=yes in their own
// NUMBER_LIMIT_IMPORTANT quota, so without this list one kernel update's history
// is evicted by ten unrelated small updates, which is exactly the state a user
// wants to go back to. SUSE's snapper-zypp-plugin marks the same set.
//
// pacman matches Target against package names, so these are names, not globs.
const pacmanImportantTargetList = "linux linux-lts linux-zen linux-hardened linux-rt linux-rt-lts " +
	"linux-firmware mkinitcpio glibc systemd btrfs-progs snapper refind " +
	"mesa nvidia vulkan-radeon vulkan-intel vulkan-nouveau"

// pacmanImportantTargets is the same list as a slice, for tests that need to read
// it back rather than grep a hook file.
var pacmanImportantTargets = strings.Fields(pacmanImportantTargetList)

// pacmanHookTriggers is shared by both halves: a pair is only meaningful for a
// transaction that changes packages. Type is Package and never File: pacman allows
// exactly one Type per hook (declaring two warns and the last one wins), and
// Package is the type that covers every transaction - including a local
// pacman -U of a .pkg.tar.zst, whose package name is known before it is added.
const pacmanHookTriggers = `[Trigger]
Operation = Install
Operation = Upgrade
Operation = Remove
Type = Package
Target = *
`

// pacmanImportantHookTriggers is the same trigger narrowed to the packages whose
// transactions are worth keeping. A hook may declare exactly one Type (a second
// declaration warns and silently wins), so this is a third file rather than extra
// lines in the first two.
const pacmanImportantHookTriggers = `[Trigger]
Operation = Install
Operation = Upgrade
Operation = Remove
Type = Package
Target = ` + pacmanImportantTargetList + `
`

const pacmanPreHook = `# SPDX-License-Identifier: GPL-3.0
# Written by the Aelum installer: snapshot @ before every pacman transaction.
#
# The pre snapshot is what makes "undo that update" possible: snapper compares it
# with the post hook's snapshot to show what the transaction changed, and the boot
# menu can boot the state from before it (aelum-refind-snapshots).
#
# All three hooks end in || true: a PreTransaction hook that fails aborts the
# transaction, and a snapshot that cannot be taken is never a reason to refuse an
# update. The snapshots are tagged with the number cleanup algorithm, so an update
# history is bounded by the limits in /etc/snapper/configs/root instead of growing
# forever, and EMPTY_PRE_POST_CLEANUP drops the pairs whose transaction changed
# nothing.
#
# --no-dbus, on purpose: the default path asks snapperd to do the work, and if that
# service is masked, crashed, or D-Bus is unavailable, the call fails - which here
# means the undo path disappears with no message, no log, and an install that
# reported success. The hooks run as root, so snapper can do it itself; that is
# what --no-dbus is for, and it is what the installer's own snapshot call uses.
` + pacmanHookTriggers + `
[Action]
Depends = snapper
When = PreTransaction
Exec = ` + pacmanPreHookExec + `
`

const pacmanPostHook = `# SPDX-License-Identifier: GPL-3.0
# Written by the Aelum installer: the post half of the pair that
# 90-aelum-snapper-pre.hook starts. See that file for why.
#
# The pre snapshot's number is the pairing key, so it is read back from the file
# the pre hook left. With no number there is nothing to pair with, which is the
# normal state after a transaction whose pre hook did not run. snapper prints the
# number it created, so both halves are written to the pair file for the hook that
# runs after this one; the post number stays empty when the post snapshot failed,
# which leaves the pre snapshot markable on its own. Hook output is sent away: a
# successful transaction should not gain a line of its own.
` + pacmanHookTriggers + `
[Action]
Depends = snapper
When = PostTransaction
Exec = ` + pacmanPostHookExec + `
`

const pacmanImportantPostHook = `# SPDX-License-Identifier: GPL-3.0
# Written by the Aelum installer: the third hook of the pair that
# 90-aelum-snapper-pre.hook starts and 91-aelum-snapper-post.hook finishes. It
# runs after them, in filename order, and marks that pair important when the
# transaction touched the kernel, the init system, the boot or the display driver.
#
# Marking is a modify, not a second pair: the snapshots already exist and they are
# the right ones. snapper keeps important=yes snapshots in their own
# NUMBER_LIMIT_IMPORTANT quota instead of letting them compete for the ordinary
# count, which is the difference between "the state before the kernel update is
# still here" and "ten unrelated small updates evicted it".
` + pacmanImportantHookTriggers + `
[Action]
Depends = snapper
When = PostTransaction
Exec = ` + pacmanImportantHookExec + `
`

// The three Exec lines. Each one is a single line in the hook file - pacman parses
// Exec as one command, with no continuation - and is wrapped here only so this file
// stays readable. All three end in a truthy command: a PreTransaction hook that
// fails aborts the transaction, and a snapshot that could not be taken is never a
// reason to refuse an update.
//
// The command body carries no single quotes: pacman splits the Exec line on
// quoting, so a single quote nested inside the sh -c wrapper ends the quoting
// there and sh receives a truncated command (measured: an earlier
// printf '%s %s\n' made every transaction fail with "unexpected end of file
// from `if'"). Quote what the shell needs with double quotes - inert inside
// the outer single quotes, and the space they protect survives pacman's split.
const (
	pacmanPreHookExec = `/bin/sh -c 'mkdir -p /run/aelum-snapper && rm -f ` + pacmanPreHookState + ` ` + pacmanPairState +
		` && snapper --no-dbus -c ` + snapperRootConfig + ` create -t pre -p -c number -d pacman > ` +
		pacmanPreHookState + ` 2>/dev/null || true'`

	pacmanPostHookExec = `/bin/sh -c 'n=$(cat ` + pacmanPreHookState + ` 2>/dev/null); if [ -n "$n" ]; then ` +
		`m=$(snapper --no-dbus -c ` + snapperRootConfig + ` create -t post --pre-number "$n" -p -c number -d pacman 2>/dev/null); ` +
		`printf "%s %s\n" "$n" "$m" > ` + pacmanPairState + `; fi; true'`

	pacmanImportantHookExec = `/bin/sh -c 'set -- $(cat ` + pacmanPairState + ` 2>/dev/null); ` +
		`for n in "$1" "$2"; do case "$n" in ""|*[!0-9]*) continue ;; esac; ` +
		`snapper --no-dbus -c ` + snapperRootConfig + ` modify -u important=yes "$n" >/dev/null 2>&1 || true; done; true'`
)

// writePacmanSnapshotHooks installs the transaction snapshots as plain hook files
// in pacman's own hooks directory. The alternative is a package (snap-pac) whose
// numbering, descriptions and config names are not this installer's to choose.
func writePacmanSnapshotHooks() error {
	if err := primitives.WriteFile(pacmanPreHookPath, pacmanPreHook); err != nil {
		return fmt.Errorf("failed to write the pacman pre-transaction snapshot hook: %w", err)
	}
	if err := primitives.WriteFile(pacmanPostHookPath, pacmanPostHook); err != nil {
		return fmt.Errorf("failed to write the pacman post-transaction snapshot hook: %w", err)
	}
	if err := primitives.WriteFile(pacmanImportantHookPath, pacmanImportantPostHook); err != nil {
		return fmt.Errorf("failed to write the pacman post-transaction important-snapshot hook: %w", err)
	}
	return nil
}

// ConfigureSnapper configures snapper for the btrfs layout: a config per
// snapshotted subvolume, the .snapshots subvolume snapper requires inside each
// one, the config list in /etc/conf.d/snapper, btrfs quotas, the units that take
// the timeline snapshots, a snapshot pair around every pacman transaction, the
// machinery that puts the snapshots in the boot menu, the state mounts and the
// restore prompt a snapshot boot needs, and finally the "after installation"
// snapshot - finally because it is the finished system, and because everything
// written here has to be inside it.
//
// Nothing here aborts an otherwise complete install: a system without snapshots
// still installs and boots, so a command that fails is reported and the config it
// belongs to is skipped rather than left half-written (a snapper config whose
// .snapshots subvolume is missing makes every snapper command for it fail).
func ConfigureSnapper(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}
	// ext4 installs have no subvolumes to snapshot, and the installer does not
	// install snapper for them (see installbase.BtrfsOnlyBasePackages).
	if ctx.FileSystem != shared.FileSystemBtrfs {
		return nil
	}

	enableBtrfsQuotas(ctx)

	subvolumes := shared.GetBtrfsSubvolumes()
	configured := make([]string, 0, len(snapperTargets))
	rootConfigured := false
	for _, target := range snapperTargets {
		// The list above and the subvolume scheme must agree. If they ever drift,
		// skip the target instead of snapshotting something that is not there.
		subvolume := shared.FindSubvolumeByMountPoint(subvolumes, target.mountPoint)
		if subvolume == nil {
			if ctx.Logger != nil {
				ctx.Logger.Error("Warning: no subvolume mounted at %s; snapper config %s skipped",
					target.mountPoint, target.configName)
			}
			continue
		}

		if err := ensureSnapshotSubvolume(ctx, target, subvolume); err != nil {
			if ctx.Logger != nil {
				ctx.Logger.Error("Warning: snapper config %s skipped: %v", target.configName, err)
			}
			continue
		}

		if err := primitives.WriteFile(snapperConfigPath(target.configName), snapperConfigContent(target, ensureSnapshotQGroup(ctx, target))); err != nil {
			return err
		}
		configured = append(configured, target.configName)
		if target.configName == snapperRootConfig {
			rootConfigured = true
		}
		if ctx.Logger != nil {
			ctx.Logger.Info("Snapper config %s: snapshots for %s (%s)",
				target.configName, target.subvolume, target.mountPoint)
		}
	}

	if len(configured) == 0 {
		return nil
	}
	if err := primitives.WriteFile(snapperSysconfigPath, snapperSysconfigContent(configured)); err != nil {
		return err
	}

	enableSnapperUnits(ctx)

	if err := writePacmanSnapshotHooks(); err != nil {
		return err
	}
	// Only the root config's snapshots are bootable, so the boot menu follows that
	// config alone: with no root config there is nothing for it to list.
	if !rootConfigured {
		return nil
	}
	if err := configureSnapshotBootEntries(ctx); err != nil {
		return err
	}
	// Before the snapshot, on purpose: a snapshot boots the unit files the snapshot
	// contains, so the mounts a snapshot boot needs have to be inside it. The
	// "after installation" snapshot is taken last for the same reason (configure.go).
	if err := configureSnapshotStateDirs(ctx); err != nil {
		return err
	}
	// The restore path - the helper, its polkit action and grant, the prompt that
	// asks - is not written here: it is source in the live ISO and the SetupUsers
	// step copied it into this tree (install/isocopy, btrfs-only). That step runs
	// before this one, which is what keeps the property this step has to keep: a
	// snapshot boots the binaries, rules and prompt it contains, so they had to be
	// here before the factory snapshot was taken.
	if err := createInstallationSnapshot(ctx); err != nil {
		return err
	}
	// The menu is written after the snapshot it lists exists, and by the script the
	// install itself will run from now on - not by a second copy of its logic here.
	return writeInitialSnapshotMenu(ctx)
}

// ensureSnapshotSubvolume makes sure a config has a .snapshots subvolume to write
// into. Snapper needs it to be a subvolume, not a directory: it refuses to use a
// plain .snapshots, and as a subvolume it stays out of the snapshots of its own
// parent.
//
// For the subvolume that holds the system the history is part of the layout and is
// mounted at /.snapshots (see format.root_snapshot and the fstab): the live root is
// itself a snapshot, and a snapshot cannot contain a subvolume, so this step could
// not create the history inside it even if it wanted to. What it can check is that
// the mount is there - without it snapper would write its history inside its own
// snapshots.
//
// Every other config's history is created here, inside its own subvolume.
func ensureSnapshotSubvolume(ctx *shared.InstallContext, target snapperTarget, subvolume *shared.BtrfsSubvolume) error {
	if subvolume.SnapshotRoot {
		mountedAt := shared.MountRoot + shared.BtrfsSnapshotsMountPoint
		out, err := primitives.Run("findmnt", "-n", "-o", "SOURCE", mountedAt).Output()
		if err != nil || strings.TrimSpace(string(out)) == "" {
			return fmt.Errorf("%s is not mounted: the history of %s would be written inside its own snapshots",
				shared.BtrfsSnapshotsMountPoint, subvolume.Name)
		}
		return nil
	}

	// The mount point is the subvolume as the installed system sees it, so the
	// host path is the same path under MountRoot ("/home" -> "/mnt/home"). Trim the
	// trailing slash so "/" does not become "/mnt//".
	mountedAt := strings.TrimSuffix(shared.MountRoot+target.mountPoint, "/")
	path := mountedAt + "/" + shared.BtrfsSnapshotsDirName

	out, err := primitives.Run("btrfs", "subvolume", "create", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("btrfs subvolume create %s: %w: %s", path, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// createInstallationSnapshot takes the snapshot "restore to the state the installer
// produced" is measured against, and marks it so no cleanup algorithm ever removes
// it. SUSE ships the same snapshot in every rollback-capable install and its own
// documentation lists it as "after installation, important=yes"; without it the
// oldest state a user can go back to is whatever the first transaction happened to
// leave behind.
//
// It is not fatal. A system whose first snapshot failed still installs, still boots
// and still snapshots every transaction after it.
func createInstallationSnapshot(ctx *shared.InstallContext) error {
	out, err := primitives.RunChroot("snapper", "--no-dbus", "-c", snapperRootConfig,
		"create", "-t", "single", "-d", "after installation", "-c", "number", "-u", "important=yes")
	if err != nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("Warning: the \"after installation\" snapshot failed (non-fatal): %v: %s",
				err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("Snapshot \"after installation\": the state this installer produced is restorable")
	}
	return nil
}

// enableBtrfsQuotas turns on btrfs qgroup accounting, which snapper needs to
// report the space a snapshot uses and to bound its cleanup by space. It is not
// fatal: without quotas snapper still creates, compares and cleans snapshots by
// count, and quotas can be enabled later with `btrfs quota enable /`.
func enableBtrfsQuotas(ctx *shared.InstallContext) {
	out, err := primitives.Run("btrfs", "quota", "enable", shared.MountRoot).CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(out))
		if ctx.Logger != nil {
			// Re-running the installer over an existing filesystem is the common
			// case here, and btrfs reports it as an error.
			if strings.Contains(text, "already enabled") {
				ctx.Logger.Info("btrfs quotas already enabled")
			} else {
				ctx.Logger.Error("Warning: btrfs quotas (snapper space limits need them): %v: %s", err, text)
			}
		}
		return
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("btrfs quotas enabled for snapshot space accounting")
	}
}

// ensureSnapshotQGroup creates the level-1 qgroup a config's QGROUP names and
// returns the value to write, or "" when the group could not be created.
//
// Enabling quotas is only half of it. snapper measures a config's snapshots by
// attaching each snapshot's own 0/<subvolid> group to the aggregate named in
// QGROUP (Snapper::prepareQuota), and refuses to do any of it when the config
// names none ("qgroup not set") — which is why SPACE_LIMIT / FREE_LIMIT do nothing
// on a config that has quotas but no group. Creating it here is what
// `snapper -c root setup-quota` does on first use: quota_enable, pick a free
// level-1 id, qgroup_create, write it into the config. Doing it at install time
// means the limits are live from the first cleanup instead of from the first user
// who happens to run that command.
//
// Not fatal either way: with no group snapper falls back to count-only cleanup,
// which is the state every other Arch snapper setup is in. Writing QGROUP="" for
// that case is deliberate — a config naming a group that does not exist makes every
// cleanup report "quota not working" for no gain.
//
// The id is level-1 id 0 (1/1 for @home): free on a filesystem the installer just
// made. A user who later defines their own level-1 (user or group) quotas picks a
// different one, or points both configs at a shared group on purpose.
func ensureSnapshotQGroup(ctx *shared.InstallContext, target snapperTarget) string {
	if target.qgroup == "" {
		return ""
	}

	out, err := primitives.Run("btrfs", "qgroup", "create", target.qgroup, shared.MountRoot).CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(out))
		// Re-running the installer over a filesystem that already has the group is
		// the common case, and btrfs reports it as an error.
		if strings.Contains(strings.ToLower(text), "already") {
			return target.qgroup
		}
		if ctx.Logger != nil {
			ctx.Logger.Error("Warning: btrfs qgroup %s for snapper config %s (space limits stay off): %v: %s",
				target.qgroup, target.configName, err, text)
		}
		return ""
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("btrfs qgroup %s created: %s snapshots are measured, not just counted", target.qgroup, target.configName)
	}
	return target.qgroup
}

// enableSnapperUnits enables snapperd and the timeline timers. Each unit is
// enabled on its own so one missing unit cannot drop the others; none of them is
// required for a bootable system, so failures are warnings.
func enableSnapperUnits(ctx *shared.InstallContext) {
	for _, unit := range snapperUnits {
		out, err := primitives.RunChroot("systemctl", "enable", unit)
		if err != nil && ctx.Logger != nil {
			ctx.Logger.Error("Warning: failed to enable %q (non-fatal): %v: %s", unit, err, strings.TrimSpace(string(out)))
		}
	}
}

// snapperConfigPath is the config file snapper reads for one config name.
func snapperConfigPath(configName string) string {
	return "/etc/snapper/configs/" + configName
}

// snapperConfigContent renders one snapper config. SUBVOLUME holds the mount
// point of the subvolume to snapshot: snapper resolves the subvolume itself from
// the mount table at runtime, so the config stays correct whatever device, UUID
// or subvolume id the mount uses. qgroup is the group ensureSnapshotQGroup
// created, or "" when there is none, which is snapper's own spelling for "no space
// accounting".
func snapperConfigContent(target snapperTarget, qgroup string) string {
	return fmt.Sprintf(`# SPDX-License-Identifier: GPL-3.0
# Aelum snapper configuration for %s (mounted at %s), written by the installer.
#
# Snapshots are the rollback path for a bad update, so they must never become the
# thing that fills the disk - a root that is full is not a degraded system, it is
# an unbootable one.
#
# Every limit below is a range (min-max), never a single number, because snapper
# skips its space-aware path entirely when the limits are degenerated, i.e. when
# min == max (client/cleanup.cc Cleaner::is_quota_aware and is_free_aware,
# client/utils/Range.h). A range says: keep at least the min even if the space
# limits are exceeded, keep at most the max even when there is room. QGROUP is the
# aggregate the snapshots are measured against, and the installer creates it
# (btrfs qgroup create %s); FREE_LIMIT is the filesystem's own free space and needs
# no group, so it is the backstop when two configs share one partition.
#
# Quotas and qgroup accounting are what make that work, and they are not free: a
# filesystem with quotas charges every write to them, more as the snapshot count
# grows. SUSE pays the same price for the same reason and asks for a 16 GB root or
# more; the limits below are what keep the cost bounded.
SUBVOLUME=%q
FSTYPE="btrfs"

QGROUP=%q
SPACE_LIMIT="0.5"
FREE_LIMIT="0.2"

# The Aelum user is in wheel (see install/users), so snapper works without sudo.
ALLOW_GROUPS="wheel"

BACKGROUND_COMPARISON="yes"
COMPRESSION="gzip"

# Pre/post pairs - the undo path for one transaction - and rollback backups.
# important=yes snapshots (the kernel, glibc, systemd, mkinitcpio, the boot and the
# display-driver transactions, and the "after installation" state) are kept in their
# own quota, so a run of small unrelated updates cannot evict the state a rollback
# would go back to.
NUMBER_CLEANUP="yes"
NUMBER_MIN_AGE=%q
NUMBER_LIMIT=%q
NUMBER_LIMIT_IMPORTANT=%q

# Timeline snapshots are off, as they are for / in openSUSE's own default setup: the
# history of a system is the states someone had a reason to keep, and a timer that
# fired is not one of them. It also keeps a snapshot boot from adding to the history
# it booted from. The limits stay ranges and snapper-timeline.timer stays enabled,
# so turning this on (btrfs-assistant has the switch) works and stays bounded.
TIMELINE_CREATE=%q
TIMELINE_CLEANUP="yes"
TIMELINE_MIN_AGE=%q
TIMELINE_LIMIT_HOURLY=%q
TIMELINE_LIMIT_DAILY=%q
TIMELINE_LIMIT_WEEKLY=%q
TIMELINE_LIMIT_MONTHLY=%q
TIMELINE_LIMIT_YEARLY=%q

EMPTY_PRE_POST_CLEANUP="yes"
EMPTY_PRE_POST_MIN_AGE=%q
`, target.subvolume, target.mountPoint, target.qgroup, target.mountPoint, qgroup,
		snapperMinAge, target.retention.numberLimit, target.retention.numberLimitImportant,
		target.retention.timelineCreate, snapperMinAge,
		target.retention.hourly, target.retention.daily, target.retention.weekly,
		target.retention.monthly, target.retention.yearly,
		snapperMinAge)
}

// snapperSysconfigPath is the sysconfig file snapper keeps its config list in.
const snapperSysconfigPath = "/etc/conf.d/snapper"

// snapperSysconfigContent lists the configs this install has, the same list
// `snapper create-config` maintains.
func snapperSysconfigContent(configNames []string) string {
	return fmt.Sprintf(`# SPDX-License-Identifier: GPL-3.0
# Written by the Aelum installer: the snapper configs of this installation
# (one per snapshotted subvolume, in /etc/snapper/configs/).
SNAPPER_CONFIGS="%s"
`, strings.Join(configNames, " "))
}

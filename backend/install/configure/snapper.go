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
type snapperTarget struct {
	// configName is the name snapper and `snapper -c <name>` use.
	configName string
	// subvolume is the Aelum subvolume this config snapshots.
	subvolume string
	// mountPoint is where the installed system mounts it.
	mountPoint string
}

var snapperTargets = []snapperTarget{
	{configName: snapperRootConfig, subvolume: "@", mountPoint: "/"},
	{configName: "home", subvolume: "@home", mountPoint: "/home"},
}

// Snapshot retention. The timeline is the "when did this break?" history: recent
// changes in detail, older ones in coarse steps. Pre/post pairs (snapper
// --command, btrfs-assistant rollbacks) are kept by the number algorithm.
const (
	snapperMinAge         = "1800"
	snapperNumberLimit    = "10"
	snapperNumberLimitImp = "10"

	snapperLimitHourly  = "6"
	snapperLimitDaily   = "7"
	snapperLimitWeekly  = "4"
	snapperLimitMonthly = "6"
	snapperLimitYearly  = "2"
)

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

// pacmanSnapshotHooks are the two halves of one pre/post pair taken around every
// pacman transaction. They are numbered so the pre snapshot runs before the
// transaction and the post snapshot after every other post-transaction hook
// (mkinitcpio regenerates this install's kernel directory at 90-, and the post
// snapshot belongs after the transaction is really finished).
const (
	pacmanPreHookPath  = "/etc/pacman.d/hooks/90-aelum-snapper-pre.hook"
	pacmanPostHookPath = "/etc/pacman.d/hooks/91-aelum-snapper-post.hook"

	// pacmanPreHookState carries the pre snapshot's number to the post hook:
	// snapper pairs pre and post by number, and one hook cannot talk to the next.
	// /run is tmpfs, so a half pair cannot survive a reboot.
	pacmanPreHookState = "/run/aelum-snapper/pacman.pre"
)

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

const pacmanPreHook = `# SPDX-License-Identifier: GPL-3.0
# Written by the Aelum installer: snapshot @ before every pacman transaction.
#
# The pre snapshot is what makes "undo that update" possible: snapper compares it
# with the post hook's snapshot to show what the transaction changed, and the boot
# menu can boot the state from before it (aelum-refind-snapshots).
#
# Both hooks end in || true: a PreTransaction hook that fails aborts the
# transaction, and a snapshot that cannot be taken is never a reason to refuse an
# update. The snapshots are tagged with the number cleanup algorithm, so an update
# history is bounded by NUMBER_LIMIT in /etc/snapper/configs/root instead of
# growing forever, and EMPTY_PRE_POST_CLEANUP drops the pairs whose transaction
# changed nothing.
` + pacmanHookTriggers + `
[Action]
Depends = snapper
When = PreTransaction
Exec = /bin/sh -c 'mkdir -p /run/aelum-snapper && snapper -c ` + snapperRootConfig + ` create -t pre -p -c number -d pacman > ` + pacmanPreHookState + ` 2>/dev/null || true'
`

const pacmanPostHook = `# SPDX-License-Identifier: GPL-3.0
# Written by the Aelum installer: the post half of the pair that
# 90-aelum-snapper-pre.hook starts. See that file for why.
#
# The pre snapshot's number is the pairing key, so it is read back from the file
# the pre hook left. With no number there is nothing to pair with, which is the
# normal state after a transaction whose pre hook did not run. snapper prints the
# number it created and pacman forwards hook output, so that is sent away: a
# successful transaction should not gain a line of its own.
` + pacmanHookTriggers + `
[Action]
Depends = snapper
When = PostTransaction
Exec = /bin/sh -c 'n=$(cat ` + pacmanPreHookState + ` 2>/dev/null); [ -n "$n" ] && snapper -c ` + snapperRootConfig + ` create -t post --pre-number "$n" -p -c number -d pacman >/dev/null || true; rm -f ` + pacmanPreHookState + `'
`

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
	return nil
}

// ConfigureSnapper configures snapper for the btrfs layout: a config per
// snapshotted subvolume, the .snapshots subvolume snapper requires inside each
// one, the config list in /etc/conf.d/snapper, btrfs quotas, the units that take
// the timeline snapshots, a snapshot pair around every pacman transaction, the
// machinery that puts the snapshots in the boot menu, the state mounts a snapshot
// boot needs, and finally the "after installation" snapshot - finally because it
// is the finished system, and because everything written here has to be inside it.
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

		if err := primitives.WriteFile(snapperConfigPath(target.configName), snapperConfigContent(target)); err != nil {
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
// or subvolume id the mount uses.
func snapperConfigContent(target snapperTarget) string {
	return fmt.Sprintf(`# SPDX-License-Identifier: GPL-3.0
# Aelum snapper configuration for %s (mounted at %s), written by the installer.
#
# Snapshots are the rollback path for a bad update, so they must never become the
# thing that fills the disk: the counts below stay small and SPACE_LIMIT /
# FREE_LIMIT bound them. Quotas are enabled by the installer; run
# "snapper -c %s setup-quota" (or btrfs-assistant) to bind the limits to a quota
# group, which also gives every snapshot a reported size.
SUBVOLUME=%q
FSTYPE="btrfs"

SPACE_LIMIT="0.5"
FREE_LIMIT="0.2"

# The Aelum user is in wheel (see install/users), so snapper works without sudo.
ALLOW_GROUPS="wheel"

BACKGROUND_COMPARISON="yes"
COMPRESSION="gzip"

NUMBER_CLEANUP="yes"
NUMBER_MIN_AGE=%q
NUMBER_LIMIT=%q
NUMBER_LIMIT_IMPORTANT=%q

TIMELINE_CREATE="yes"
TIMELINE_CLEANUP="yes"
TIMELINE_MIN_AGE=%q
TIMELINE_LIMIT_HOURLY=%q
TIMELINE_LIMIT_DAILY=%q
TIMELINE_LIMIT_WEEKLY=%q
TIMELINE_LIMIT_MONTHLY=%q
TIMELINE_LIMIT_YEARLY=%q

EMPTY_PRE_POST_CLEANUP="yes"
EMPTY_PRE_POST_MIN_AGE=%q
`, target.subvolume, target.mountPoint, target.configName, target.mountPoint,
		snapperMinAge, snapperNumberLimit, snapperNumberLimitImp,
		snapperMinAge,
		snapperLimitHourly, snapperLimitDaily, snapperLimitWeekly, snapperLimitMonthly, snapperLimitYearly,
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

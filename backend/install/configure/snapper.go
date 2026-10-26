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
	{configName: "root", subvolume: "@", mountPoint: "/"},
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

// ConfigureSnapper configures snapper for the btrfs layout: a config per
// snapshotted subvolume, the .snapshots subvolume snapper requires inside each
// one, the config list in /etc/conf.d/snapper, btrfs quotas, and the units that
// take the timeline snapshots.
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
	for _, target := range snapperTargets {
		// The list above and the subvolume scheme must agree. If they ever drift,
		// skip the target instead of snapshotting something that is not there.
		if shared.FindSubvolumeByMountPoint(subvolumes, target.mountPoint) == nil {
			if ctx.Logger != nil {
				ctx.Logger.Error("Warning: no subvolume mounted at %s; snapper config %s skipped",
					target.mountPoint, target.configName)
			}
			continue
		}

		if err := createSnapshotSubvolume(target); err != nil {
			if ctx.Logger != nil {
				ctx.Logger.Error("Warning: snapper config %s skipped: %v", target.configName, err)
			}
			continue
		}

		if err := primitives.WriteFile(snapperConfigPath(target.configName), snapperConfigContent(target)); err != nil {
			return err
		}
		configured = append(configured, target.configName)
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
	return nil
}

// createSnapshotSubvolume creates the .snapshots subvolume inside the mounted
// subvolume. Snapper needs it to be a subvolume, not a directory: it refuses to
// use a plain .snapshots, and as a subvolume it stays out of the snapshots of
// its own parent.
func createSnapshotSubvolume(target snapperTarget) error {
	// The mount point is the subvolume as the installed system sees it, so the
	// host path is the same path under MountRoot ("/" -> "/mnt", "/home" ->
	// "/mnt/home"). Trim the trailing slash so "/" does not become "/mnt//".
	mountedAt := strings.TrimSuffix(shared.MountRoot+target.mountPoint, "/")
	path := mountedAt + "/.snapshots"

	out, err := primitives.Run("btrfs", "subvolume", "create", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("btrfs subvolume create %s: %w: %s", path, err, strings.TrimSpace(string(out)))
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

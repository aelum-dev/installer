// Package partition implements manual partitioning.
//
// Unlike the automatic format step (which wipes the whole disk and creates an
// EFI + root layout), the manual partition step only touches the two
// partitions the user designates (one Boot, one Root). Both are always
// formatted. Every other partition on the disk is left exactly as-is, so
// another operating system can keep its own partitions and its own ESP.
package partition

import (
	"fmt"
	"sort"
	"strconv"

	"aelum-installer/backend/install/format"
	"aelum-installer/backend/install/lvm"
	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ComputeLayout returns a ManualLayout describing a disk's current partitions
// (all preset to Preserve) plus the derived free space. It reuses the same
// lsblk path as shared.GetDiskPartitions / shared.GetDisks.
func ComputeLayout(diskID string) (shared.ManualLayout, error) {
	if diskID == "" {
		return shared.ManualLayout{}, fmt.Errorf("disk ID not set")
	}

	// Disk size.
	disks, err := shared.GetDisks()
	if err != nil {
		return shared.ManualLayout{}, err
	}
	var diskSizeBytes int64
	found := false
	for _, d := range disks {
		if d.Path == diskID {
			diskSizeBytes = d.Size
			found = true
			break
		}
	}
	if !found {
		return shared.ManualLayout{}, fmt.Errorf("disk %s not found", diskID)
	}

	// Existing partitions.
	partitions, err := shared.GetDiskPartitions(diskID)
	if err != nil {
		return shared.ManualLayout{}, err
	}

	var layout shared.ManualLayout
	layout.Disk = diskID
	layout.DiskSizeMB = bytesToMB(diskSizeBytes)
	// Carry the exact byte/sector geometry so downstream free-space math can stay
	// truncation-free instead of relying on the rounded-down MiB values below.
	layout.DiskSizeBytes = diskSizeBytes
	// Record the disk's logical sector size so all downstream sector math uses
	// the same unit sgdisk reports in (512e vs 4Kn).
	layout.SectorSizeBytes = shared.LogicalSectorSize(diskID)
	// Always return a non-nil slice: an empty disk (no partitions) must marshal
	// as "partitions": [] rather than null. The frontend iterates this list
	// directly, and a null payload crashes its free-space math (leaving the
	// manual page stuck on "Scanning partitions...").
	layout.Partitions = make([]shared.ManualPartition, 0, len(partitions))
	for i, p := range partitions {
		layout.Partitions = append(layout.Partitions, shared.ManualPartition{
			Index:          i,
			Path:           p.Path,
			SizeMB:         bytesToMB(p.Size),
			SizeBytes:      p.Size,
			ExistingFstype: p.Fstype,
			// Role is empty → untouched (not formatted, mounted, or added to fstab).
		})
	}

	return layout, nil
}

// Apply formats and prepares the Boot and Root partitions described by layout,
// then populates ctx.Partitions so the subsequent mount/install/configure steps
// work unchanged. Preserve partitions are never touched.
//
// This replaces format.Run on the manual install path.
func Apply(ctx *shared.InstallContext, layout *shared.ManualLayout) (retErr error) {
	diskID := layout.Disk
	if diskID == "" {
		diskID = ctx.DiskID
	}
	if diskID == "" {
		return fmt.Errorf("manual layout has no disk")
	}
	layout.Disk = diskID
	// A layout from the CLI may omit the sector size; read it so the placement
	// math uses the same sector unit sgdisk reports in (512e vs 4Kn).
	ensureSectorSize(layout, diskID)

	reconcileLvm(ctx, layout)
	reconcileLuks(ctx, layout)

	if err := Validate(*layout); err != nil {
		return err
	}

	// Preflight exact placement against the post-delete geometry before any
	// destructive sgdisk -d. Validate is totals-only; this mirrors
	// ValidateFeasible so a caller that skipped the GUI/CLI preflight cannot lose
	// partitions to a fragmented plan that buildActions will reject later.
	if err := preflightPlacement(diskID, layout); err != nil {
		return err
	}

	defer func() {
		if retErr != nil {
			if ctx.Logger != nil {
				ctx.Logger.Error("manual partition apply failed, cleaning up: %v", retErr)
			}
			_ = primitives.CleanupMounts(ctx.Logger) //nolint:errcheck // cleanup best-effort on error path
		}
	}()

	// Unmount anything left behind by a prior attempt.
	_ = primitives.CleanupMounts(ctx.Logger)

	// A previous LVM install leaves the `aelum` volume group active on the
	// target. Its device-mapper holders keep the System partition busy, so
	// mkfs and cryptsetup — which open with O_EXCL — fail with EBUSY. Remove the
	// group before touching the partition table (lvm.Apply does the same before
	// recreating it on the LVM path).
	lvm.Teardown(shared.LvmVolumeGroup, nil)

	// Remove any partitions the user deleted from the layout before reading the
	// disk geometry or creating new partitions. Deleting first frees their space
	// for new partitions and keeps the numbers of the surviving partitions valid.
	if err := deletePartitions(diskID, layout.DeletedPartitions); err != nil {
		return err
	}

	// Build the ordered list of format actions for Boot + Root only.
	sectors, err := partitionSectors(diskID)
	if err != nil {
		return err
	}
	actions, err := buildActions(diskID, layout, ctx.FileSystem, sectors)
	if err != nil {
		return err
	}

	// Resolve the actual device paths for the ESP and System partitions. For
	// brand-new partitions the layout rows carry an empty Path; buildActions
	// computed the real device path (e.g. /dev/vda1) in each formatAction, and
	// that is what format/blkid/mount must use downstream.
	var espPath, sysPath string
	for _, a := range actions {
		switch a.pType {
		case shared.TypeESP:
			espPath = a.path
		case shared.TypeSystem:
			sysPath = a.path
		}
	}

	// Create brand-new partitions and stamp the correct GPT type on the
	// existing partitions kept in place.
	if err := applyPartitionTable(diskID, actions); err != nil {
		return err
	}

	// Refresh the kernel partition table.
	probeOut, probeErr := primitives.Run("partprobe", diskID).CombinedOutput()
	if probeErr != nil {
		return fmt.Errorf("partprobe: %w: %s", probeErr, string(probeOut))
	}

	// Format each Boot/Root partition. When LUKS or LVM is enabled the System
	// partition is not formatted directly (EncryptRoot / SetupRootFilesystem
	// below create the filesystem, and for LVM the PV/VG/LVs). The Boot
	// partition is always formatted, new or existing: it belongs to this
	// install, so it must never carry another OS's boot loaders.
	for _, a := range actions {
		if a.pType == shared.TypeSystem && (ctx.Luks || ctx.Lvm) {
			continue
		}
		label := "aelum"
		if a.fstype == shared.FileSystemVfat {
			label = "EFI"
		}
		if err := format.FormatPartition(a.path, a.fstype, label); err != nil {
			return err
		}
	}

	var setup format.RootSetup
	switch {
	case ctx.Luks:
		if len(ctx.LuksPassphrase) == 0 {
			return fmt.Errorf("LUKS enabled but no passphrase set")
		}
		setup, err = format.EncryptRoot(ctx, sysPath, string(ctx.LuksPassphrase), ctx.FileSystem)
		if err != nil {
			return err
		}
	case ctx.Lvm:
		setup, err = format.SetupRootFilesystem(ctx, sysPath, ctx.FileSystem)
		if err != nil {
			return err
		}
	default:
		// Plain root (ext4 or btrfs): the System partition was formatted above.
		// For btrfs create the subvolume scheme here, then read the filesystem
		// UUID for fstab/boot.
		if ctx.FileSystem == shared.FileSystemBtrfs {
			if err := format.SetupBtrfsSubvolumes(ctx, sysPath, shared.GetBtrfsSubvolumes()); err != nil {
				return err
			}
		}
		uuid, err := format.ReadPartitionUUID(sysPath)
		if err != nil {
			return fmt.Errorf("blkid %s: %w", sysPath, err)
		}
		setup = format.RootSetup{RootDevice: sysPath, RootUUID: uuid}
	}

	// Build PartitionInfo from the resolved EFI and System paths, applying the
	// LUKS/LVM/plain root mapping.
	partitions, err := format.NewPartitionInfo(ctx, diskID, espPath, sysPath, setup, ctx.FileSystem)
	if err != nil {
		return err
	}
	ctx.Partitions = partitions

	return nil
}

// applyPartitionTable creates every brand-new partition, then sets the expected
// GPT type GUID on each existing ESP/System action. Existing partitions are
// never recreated or renumbered: they keep their slot and are formatted in
// place, while their GPT type is adjusted so firmware and bootloader discovery
// see the role the user assigned.
func applyPartitionTable(diskID string, actions []formatAction) error {
	for _, a := range actions {
		if !a.isNew {
			continue
		}
		if err := createPartition(diskID, a); err != nil {
			return err
		}
	}
	for _, a := range actions {
		if a.isNew {
			continue
		}
		out, err := primitives.Run("sgdisk", "-t", fmt.Sprintf("%d:%s", a.num, a.guid), diskID).CombinedOutput()
		if err != nil {
			return fmt.Errorf("sgdisk set type for existing partition %d: %w: %s", a.num, err, string(out))
		}
	}
	return nil
}

// preflightPlacement simulates the exact post-delete placement before Apply
// removes anything. It is a no-op when the plan neither creates nor deletes a
// partition; otherwise it reads the current geometry and runs the same
// buildActions simulation ValidateFeasible uses.
func preflightPlacement(diskID string, layout *shared.ManualLayout) error {
	if len(layout.DeletedPartitions) == 0 && !hasNewPartitions(*layout) {
		return nil
	}
	sectors, err := partitionSectors(diskID)
	if err != nil {
		return err
	}
	return feasible(*layout, sectors)
}

// reconcileLvm reconciles the LVM decision, which reaches the manual path two
// ways: the --lvm toggle (ctx.Lvm, from the frontend/CLI) and the layout's own
// flag (layout.Lvm, which drives the System partition's GPT type in
// buildActions). LVM is on if either says so.
func reconcileLvm(ctx *shared.InstallContext, layout *shared.ManualLayout) {
	if ctx.Lvm || layout.Lvm {
		ctx.Lvm = true
		layout.Lvm = true
	}
}

// reconcileLuks reconciles the LUKS decision the same way: either the --luks
// toggle (ctx.Luks) or the layout's validation hint can enable it. Validate uses
// the flag to include the LUKS header reserve in the System-partition floor.
func reconcileLuks(ctx *shared.InstallContext, layout *shared.ManualLayout) {
	if ctx.Luks || layout.Luks {
		ctx.Luks = true
		layout.Luks = true
	}
}

// createPartition creates a new GPT partition with the correct size and type
// GUID using sgdisk. The disk path is passed as sgdisk's final argument — it is
// required: without it sgdisk succeeds silently and creates no partition.
func createPartition(diskID string, a formatAction) error {
	args := append([]string{"sgdisk"}, []string{
		"-n", fmt.Sprintf("%d:%d:+%dM", a.num, a.startSector, a.sizeMB),
		"-t", fmt.Sprintf("%d:%s", a.num, a.guid),
		"-c", fmt.Sprintf("%d:%s", a.num, roleLabel(a.fstype)),
		diskID,
	}...)
	out, err := primitives.Run(args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("sgdisk create partition %d: %w: %s", a.num, err, string(out))
	}
	return nil
}

// deletePartitions removes the given partitions from the disk with `sgdisk -d`,
// highest partition number first so the numbers of the surviving partitions are
// left untouched, then refreshes the kernel's view of the partition table.
func deletePartitions(diskID string, deleted []shared.ManualPartition) error {
	nums := deleteOrder(deleted)
	for _, num := range nums {
		out, err := primitives.Run("sgdisk", "-d", strconv.Itoa(num), diskID).CombinedOutput()
		if err != nil {
			return fmt.Errorf("sgdisk delete partition %d: %w: %s", num, err, string(out))
		}
	}

	probeOut, probeErr := primitives.Run("partprobe", diskID).CombinedOutput()
	if probeErr != nil {
		return fmt.Errorf("partprobe: %w: %s", probeErr, string(probeOut))
	}
	return nil
}

// deleteOrder extracts the partition numbers to delete and returns them
// highest-first. Deleting in that order leaves the numbers of the surviving
// partitions unchanged (sgdisk does not renumber after a deletion, but going
// high-to-low is still the safe, deterministic order).
func deleteOrder(deleted []shared.ManualPartition) []int {
	nums := make([]int, 0, len(deleted))
	for _, p := range deleted {
		if n := parsePartitionNum(p.Path); n > 0 {
			nums = append(nums, n)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(nums)))
	return nums
}

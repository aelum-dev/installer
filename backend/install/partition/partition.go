// Package partition implements dualboot-safe manual partitioning.
//
// Unlike the automatic format step (which wipes the whole disk and creates an
// EFI + root layout), the manual partition step only touches the two
// partitions the user designates (one Boot, one Root). Every other partition
// on the disk is left exactly as-is, which is what makes a dualboot install
// safe.
package partition

import (
	"fmt"
	"sort"
	"strconv"

	"aelum-installer/backend/install/format"
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
	var usedBytes int64
	for i, p := range partitions {
		sizeBytes := p.Size
		usedBytes += sizeBytes
		layout.Partitions = append(layout.Partitions, shared.ManualPartition{
			Index:          i,
			Path:           p.Path,
			SizeMB:         bytesToMB(sizeBytes),
			SizeBytes:      sizeBytes,
			ExistingFstype: p.Fstype,
			// Role is empty → untouched (dualboot-safe).
		})
	}

	freeBytes := diskSizeBytes - usedBytes
	if freeBytes < 0 {
		freeBytes = 0
	}
	layout.FreeMB = bytesToMB(freeBytes)
	layout.FreeSectors = freeBytes / layout.SectorSizeBytes
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
	// A layout from the CLI may omit the sector size; read it so the placement
	// math uses the same sector unit sgdisk reports in (512e vs 4Kn).
	ensureSectorSize(layout, diskID)

	reconcileLvm(ctx, layout)

	if err := Validate(*layout); err != nil {
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

	// Create any brand-new partitions first.
	for _, a := range actions {
		if !a.isNew {
			continue
		}
		if err := createPartition(diskID, a); err != nil {
			return err
		}
	}

	// Ensure existing ESP/System partitions carry the correct GPT type GUID.
	for _, a := range actions {
		if a.isNew {
			continue
		}
		out, err := primitives.Run("sgdisk", "-t", fmt.Sprintf("%d:%s", a.num, a.guid), diskID).CombinedOutput()
		if err != nil {
			return fmt.Errorf("sgdisk set type for existing partition %d: %w: %s", a.num, err, string(out))
		}
	}

	// Refresh the kernel partition table.
	probeOut, probeErr := primitives.Run("partprobe", diskID).CombinedOutput()
	if probeErr != nil {
		return fmt.Errorf("partprobe: %w: %s", probeErr, string(probeOut))
	}

	// Format each Boot/Root partition. When LUKS or LVM is enabled the System
	// partition is not formatted directly (EncryptRoot / SetupRootFilesystem
	// below create the filesystem, and for LVM the PV/VG/LVs), so only the ESP
	// is formatted here.
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

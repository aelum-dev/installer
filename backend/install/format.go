package install

import (
	"fmt"
	"os/exec"
	"strings"

	"aestheticos-installer/backend/shared"
)

// Error wrapping convention: use %w when wrapping an existing error so the
// caller can inspect it with errors.Is/As. Use plain %s/%d for new errors
// that do not wrap an underlying error (validation, missing config, etc.).

// FormatDisk partitions and formats the target disk, then stores partition info
// in ctx.Partitions. On failure, tears down any mounts left behind.
func FormatDisk(ctx *InstallContext) (retErr error) {
	diskID := ctx.DiskID
	if diskID == "" {
		return fmt.Errorf("disk ID not set. Provide a disk path (e.g., --format-disk /dev/vda)")
	}

	defer func() {
		if retErr != nil {
			if ctx.Logger != nil {
				ctx.Logger.Error("FormatDisk failed, cleaning up: %v", retErr)
			}
			_ = CleanupMounts(ctx.Logger, diskID) //nolint:errcheck // cleanup best-effort on error path
		}
	}()

	fsType := ctx.FileSystem
	if fsType == "" {
		fsType = shared.DefaultFileSystem
	}
	if fsType != shared.FileSystemExt4 && fsType != shared.FileSystemBtrfs {
		return fmt.Errorf("unsupported filesystem type: %s (supported: ext4, btrfs)", fsType)
	}

	partitions := []struct {
		Name       string
		SizeMB     int64
		Fstype     string
		MountPoint string
	}{
		{Name: shared.PartitionLayoutEFI, SizeMB: shared.EFIPartitionSizeMB, Fstype: shared.FileSystemVfat, MountPoint: shared.BootMountPoint},
		{Name: shared.PartitionLayoutRoot, SizeMB: 0, Fstype: fsType, MountPoint: shared.MountRoot},
	}

	// Unmount anything previously mounted (from a failed prior attempt)
	// CleanupMounts handles: sync, fuser -k, umount -R, partprobe, dmsetup
	if err := CleanupMounts(ctx.Logger, diskID); err != nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("Pre-format cleanup failed: %v", err)
		}
		// Don't fail here — sgdisk -Z below will handle disk cleanup
		// If /mnt is still mounted, format operations will fail with a clear error
	}

	// Clear partition table (GPT backup, etc.) for a clean slate
	clearOut, err := Run("sgdisk", "-Z", diskID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("sgdisk -Z failed: %w: %s", err, string(clearOut))
	}

	// Create partitions
	var currentSector int64 = shared.GPTPartitionStartSector
	var cmd *exec.Cmd
	var out []byte
	for i, p := range partitions {
		num := i + 1

		var args []string
		if p.SizeMB > 0 {
			args = []string{"-n", fmt.Sprintf("%d:%d:+%dm", num, currentSector, p.SizeMB)}
			currentSector += (p.SizeMB * 1024 * 1024) / 512
		} else {
			args = []string{"-n", fmt.Sprintf("%d:%d:0", num, currentSector)}
		}

		switch p.Fstype {
		case shared.FileSystemVfat:
			args = append(args, "-t", fmt.Sprintf("%d:%s", num, shared.GPTTypeGUIDs.ESP))
		case shared.FileSystemExt4, shared.FileSystemBtrfs:
			args = append(args, "-t", fmt.Sprintf("%d:%s", num, shared.GPTTypeGUIDs.LinuxRootX86_64))
		default:
			args = append(args, "-t", fmt.Sprintf("%d:%s", num, shared.GPTTypeGUIDs.LinuxFilesystemGeneric))
		}
		args = append(args, "-c", fmt.Sprintf("%d:%s", num, p.Name))

		cmd = Run("sgdisk")
		cmd.Args = append(cmd.Args, args...)
		cmd.Args = append(cmd.Args, diskID)
		out, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("sgdisk partition %d failed: %w: %s", num, err, string(out))
		}
	}

	// Format EFI
	if err := formatPartition(diskID, 1, partitions[0]); err != nil {
		return err
	}

	// Format root
	if err := formatPartition(diskID, 2, partitions[1]); err != nil {
		return err
	}

	// Refresh kernel partition table
	cmd = Run("partprobe", diskID)
	probeOut, probeErr := cmd.CombinedOutput()
	if probeErr != nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("partprobe failed: %v (%s)", probeErr, string(probeOut))
		}
		return fmt.Errorf("partprobe: %w", probeErr)
	}

	var subvolumes []shared.BtrfsSubvolume
	if fsType == "btrfs" {
		subvolumes = shared.GetBtrfsSubvolumes()
		if err := setupBtrfsSubvolumes(ctx, diskID, subvolumes); err != nil {
			return err
		}
	}

	// Discover UUIDs
	diskPartitions, err := shared.GetPartitions(diskID)
	if err != nil {
		return fmt.Errorf("get partitions: %w", err)
	}
	for i := range diskPartitions {
		uuid, err := readPartitionUUID(diskPartitions[i].Path)
		if err != nil {
			return fmt.Errorf("blkid %s: %w", diskPartitions[i].Path, err)
		}
		diskPartitions[i].UUID = uuid
	}

	efi, root := findPartitionsByFstype(diskPartitions)
	if efi == nil || root == nil {
		efi, root = findPartitionsByIndex(diskPartitions)
	}
	if efi == nil {
		return fmt.Errorf("EFI partition not found (got %d partitions)", len(diskPartitions))
	}
	if root == nil {
		return fmt.Errorf("root partition not found (got %d partitions)", len(diskPartitions))
	}

	ctx.Partitions = &shared.PartitionInfo{
		Disk:        diskID,
		EFIPart:     efi.Path,
		EFIUUID:     efi.UUID,
		RootPart:    root.Path,
		RootUUID:    root.UUID,
		MountPoints: make(map[string]string),
	}
	if fsType == "btrfs" {
		for _, sv := range subvolumes {
			if sv.MountPoint != "/" {
				ctx.Partitions.MountPoints[sv.MountPoint] = root.Path
			}
		}
	}

	return nil
}

func formatPartition(diskID string, num int, p struct {
	Name       string
	SizeMB     int64
	Fstype     string
	MountPoint string
}) error {
	path := derivePartitionPath(diskID, num)

	// Wipe filesystem signatures before formatting
	wipeOut, err := Run("wipefs", "--all", "-f", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("wipefs %s failed: %w: %s", path, err, string(wipeOut))
	}

	// Format with appropriate tool
	var cmd *exec.Cmd
	switch p.Fstype {
	case shared.FileSystemVfat:
		cmd = Run("mkfs.fat", "-F", "32", "-n", "EFI", path)
	case shared.FileSystemExt4:
		cmd = Run("mkfs.ext4", "-F", "-E", "lazy_itable_init=0", "-L", "aestheticos", path)
	case shared.FileSystemBtrfs:
		cmd = Run("mkfs.btrfs", "-f", "-L", "aestheticos", path)
	default:
		return fmt.Errorf("unsupported filesystem: %s", p.Fstype)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("format %s failed: %w: %s", path, err, string(out))
	}
	return nil
}

func setupBtrfsSubvolumes(ctx *InstallContext, diskID string, subvolumes []shared.BtrfsSubvolume) error {
	rootPath := derivePartitionPath(diskID, 2)
	tmp := shared.BtrfsSetupTmpMount

	_ = Run("mkdir", "-p", tmp).Run()
	cmd := Run("mount", "-o", fmt.Sprintf("subvolid=%d", shared.BtrfsTopLevelSubvolumeID), rootPath, tmp)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mount btrfs top-level: %w", err)
	}
	defer func() { _ = Run("umount", tmp).Run() }()

	for _, sv := range subvolumes {
		cmd = Run("btrfs", "subvolume", "create", "-p", tmp+"/"+sv.Name)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("create subvolume %s: %w: %s", sv.Name, err, string(out))
		}
	}

	cmd = Run("btrfs", "subvolume", "list", tmp)
	listOut, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("list subvolumes: %w", err)
	}
	var targetID string
	for _, line := range strings.Split(strings.TrimSpace(string(listOut)), "\n") {
		prefix := "path @\n"
		if strings.HasSuffix(line+"\n", prefix) {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				targetID = fields[1]
				break
			}
		}
	}
	if targetID == "" {
		return fmt.Errorf("subvolume @ not found in %s", tmp)
	}
	cmd = Run("btrfs", "subvolume", "set-default", targetID, tmp)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("set default subvolume: %w: %s", err, string(out))
	}

	for _, sv := range subvolumes {
		if sv.Compression == "" {
			continue
		}
		cmd = Run("btrfs", "property", "set", tmp+"/"+sv.Name, "compression", sv.Compression)
		if _, err := cmd.CombinedOutput(); err != nil {
			if ctx.Logger != nil {
				ctx.Logger.Error("Warning: compression on %s: %v", sv.Name, err)
			}
		}
	}

	_ = Run("rmdir", tmp).Run()
	return nil
}

func derivePartitionPath(diskID string, num int) string {
	return shared.DerivePartitionPath(diskID, num)
}

func findPartitionsByFstype(parts []shared.PartitionEntry) (*shared.PartitionEntry, *shared.PartitionEntry) {
	var efi, root *shared.PartitionEntry
	for i := range parts {
		switch parts[i].Fstype {
		case shared.FileSystemVfat:
			if efi == nil {
				efi = &parts[i]
			}
		case shared.FileSystemExt4, shared.FileSystemBtrfs:
			if root == nil {
				root = &parts[i]
			}
		}
	}
	return efi, root
}

func findPartitionsByIndex(parts []shared.PartitionEntry) (*shared.PartitionEntry, *shared.PartitionEntry) {
	var efi, root *shared.PartitionEntry
	for i := range parts {
		switch i {
		case 0:
			efi = &parts[i]
		case 1:
			root = &parts[i]
			return efi, root
		}
	}
	return efi, root
}

func readPartitionUUID(partition string) (string, error) {
	cmd := Run("blkid", "-s", "UUID", "-o", "value", partition)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("blkid %s: %w", partition, err)
	}
	return strings.TrimSpace(string(out)), nil
}

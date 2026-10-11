package format

import (
	"fmt"
	"os/exec"
	"strings"

	"aelum-installer/backend/install/luks"
	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// Run partitions and formats the target disk, then stores partition info
// in ctx.Partitions. On failure, tears down any mounts left behind.
func Run(ctx *shared.InstallContext) (retErr error) {
	diskID := ctx.DiskID
	if diskID == "" {
		return fmt.Errorf("disk ID not set. Provide a disk path (e.g., --format-disk /dev/vda)")
	}

	defer func() {
		if retErr != nil {
			if ctx.Logger != nil {
				ctx.Logger.Error("FormatDisk failed, cleaning up: %v", retErr)
			}
			_ = primitives.CleanupMounts(ctx.Logger) //nolint:errcheck // cleanup best-effort on error path
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
	// primitives.CleanupMounts handles: sync, fuser -k, umount -R, partprobe, dmsetup
	if err := primitives.CleanupMounts(ctx.Logger); err != nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("Pre-format cleanup failed: %v", err)
		}
		// Don't fail here — sgdisk -Z below will handle disk cleanup
		// If /mnt is still mounted, format operations will fail with a clear error
	}

	// Clear partition table (GPT backup, etc.) for a clean slate
	clearOut, err := primitives.Run("sgdisk", "-Z", diskID).CombinedOutput()
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

		cmd = primitives.Run("sgdisk")
		cmd.Args = append(cmd.Args, args...)
		cmd.Args = append(cmd.Args, diskID)
		out, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("sgdisk partition %d failed: %w: %s", num, err, string(out))
		}
	}

	// Format EFI
	if err := FormatPartition(shared.DerivePartitionPath(diskID, 1), shared.FileSystemVfat, "EFI"); err != nil {
		return err
	}

	// Refresh kernel partition table
	cmd = primitives.Run("partprobe", diskID)
	probeOut, probeErr := cmd.CombinedOutput()
	if probeErr != nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("partprobe failed: %v (%s)", probeErr, string(probeOut))
		}
		return fmt.Errorf("partprobe: %w", probeErr)
	}

	part2 := shared.DerivePartitionPath(diskID, 2)

	// Format the root partition. When LUKS is enabled the root is encrypted: a
	// LUKS2 header is written, the mapper device is opened, the filesystem (and
	// btrfs subvolumes) are created on the mapper, then the mapper is closed
	// again (mount reopens it). The EFI partition is always plaintext.
	var mapperPath, mapperUUID, luksUUID string
	if ctx.Luks {
		if len(ctx.LuksPassphrase) == 0 {
			return fmt.Errorf("LUKS enabled but no passphrase set")
		}
		mapperPath, mapperUUID, luksUUID, err = EncryptRoot(ctx, part2, string(ctx.LuksPassphrase), fsType)
		if err != nil {
			return err
		}
	} else {
		// Unencrypted root: the filesystem lives directly on the partition, so the
		// partition path and its filesystem UUID are the "mapper" values the mount,
		// install, and configure steps consume (btrfs subvolumes included).
		mapperPath, mapperUUID, err = formatUnencryptedRoot(ctx, part2, fsType)
		if err != nil {
			return err
		}
	}

	// Discover the EFI UUID (plaintext) and build the partition info.
	diskPartitions, err := shared.GetPartitions(diskID)
	if err != nil {
		return fmt.Errorf("get partitions: %w", err)
	}
	for i := range diskPartitions {
		uuid, err := readPartitionUUID(diskPartitions[i].Path)
		if err != nil {
			// A LUKS-encrypted root reports its header UUID, not a filesystem
			// UUID — that is expected and harmless here.
			if !ctx.Luks || diskPartitions[i].Path != part2 {
				return fmt.Errorf("blkid %s: %w", diskPartitions[i].Path, err)
			}
		}
		diskPartitions[i].UUID = uuid
	}

	efi, _ := findPartitionsByFstype(diskPartitions)
	if efi == nil {
		efi, _ = findPartitionsByIndex(diskPartitions)
	}
	if efi == nil {
		types := partitionTypes(diskPartitions)
		return fmt.Errorf("EFI (vfat) partition not found — found partitions: %s", types)
	}

	info := &shared.PartitionInfo{
		Disk:        diskID,
		EFIPart:     efi.Path,
		EFIUUID:     efi.UUID,
		RootPart:    part2,
		RootUUID:    mapperUUID,
		MountPoints: make(map[string]string),
	}
	if ctx.Luks {
		info.RootMapper = mapperPath
		info.LuksUUID = luksUUID
	}
	if fsType == "btrfs" {
		for _, sv := range shared.GetBtrfsSubvolumes() {
			if sv.MountPoint != "/" {
				// Subvolumes live on the mapper device when LUKS is enabled.
				info.MountPoints[sv.MountPoint] = mapperPath
			}
		}
	}
	ctx.Partitions = info

	return nil
}

// EncryptRoot runs the full LUKS sequence on the raw root partition:
//
//  1. luksFormat writes a LUKS2 header (keyed by passphrase).
//  2. luksOpen creates the decrypted mapper device.
//  3. mkfs (ext4/btrfs) is created on the mapper device.
//  4. btrfs subvolumes are created on the mapper.
//  5. The mapper's filesystem UUID and the LUKS header UUID are read.
//  6. The mapper is closed again; mount reopens it during install.
//
// It returns the mapper path, the mapper filesystem UUID, and the LUKS header
// UUID (used for the boot cmdline).
func EncryptRoot(ctx *shared.InstallContext, rawPart, passphrase, fsType string) (mapperPath, mapperUUID, luksUUID string, err error) {
	mapperPath = shared.LuksMapperPath

	if err := luks.LuksFormat(ctx.Logger, rawPart, passphrase); err != nil {
		return "", "", "", err
	}
	if err := luks.LuksOpen(rawPart, shared.LuksMapperName, passphrase); err != nil {
		return "", "", "", fmt.Errorf("opening LUKS device (validates passphrase): %w", err)
	}
	// Ensure the mapper is closed even if a later step fails, so the device
	// doesn't leak between install retries.
	defer func() {
		if cerr := luks.LuksClose(shared.LuksMapperName); cerr != nil && ctx.Logger != nil {
			ctx.Logger.Info("Warning: closing LUKS mapper: %v", cerr)
		}
	}()

	// Create the filesystem on the decrypted mapper, not the raw partition.
	if err := FormatPartition(mapperPath, fsType, "aelum"); err != nil {
		return "", "", "", err
	}

	// btrfs subvolumes are created on the mapper too.
	if fsType == shared.FileSystemBtrfs {
		if err := SetupBtrfsSubvolumes(ctx, mapperPath, shared.GetBtrfsSubvolumes()); err != nil {
			return "", "", "", err
		}
	}

	// Read the mapper's filesystem UUID while it is still open.
	mapperUUID, err = readPartitionUUID(mapperPath)
	if err != nil {
		return "", "", "", fmt.Errorf("blkid %s: %w", mapperPath, err)
	}
	// Read the LUKS header UUID (from the raw partition) for the boot cmdline.
	luksUUID, err = luks.LuksUUID(rawPart)
	if err != nil {
		return "", "", "", err
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Encrypted root: mapper=%s fsUUID=%s luksUUID=%s", mapperPath, mapperUUID, luksUUID)
	}
	return mapperPath, mapperUUID, luksUUID, nil
}

// formatUnencryptedRoot formats the raw root partition in place (no LUKS) and
// returns the device path and filesystem UUID the mount, install, and configure
// steps consume. For btrfs it also creates the standard subvolume layout, so the
// plain path mirrors what EncryptRoot sets up on the decrypted mapper.
func formatUnencryptedRoot(ctx *shared.InstallContext, part2, fsType string) (mapperPath, mapperUUID string, err error) {
	if err := FormatPartition(part2, fsType, "aelum"); err != nil {
		return "", "", err
	}
	uuid, err := readPartitionUUID(part2)
	if err != nil {
		return "", "", fmt.Errorf("blkid %s: %w", part2, err)
	}
	if fsType == shared.FileSystemBtrfs {
		if err := SetupBtrfsSubvolumes(ctx, part2, shared.GetBtrfsSubvolumes()); err != nil {
			return "", "", err
		}
	}
	return part2, uuid, nil
}

// BuildPartitionInfo reads UUIDs for the EFI and root partitions and returns
// a PartitionInfo suitable for the mount/install steps. For btrfs it also
// populates the subvolume mount points map.
func BuildPartitionInfo(diskID, efiPath, rootPath, fsType string) (*shared.PartitionInfo, error) {
	efiUUID, err := readPartitionUUID(efiPath)
	if err != nil {
		return nil, fmt.Errorf("blkid %s: %w", efiPath, err)
	}
	rootUUID, err := readPartitionUUID(rootPath)
	if err != nil {
		return nil, fmt.Errorf("blkid %s: %w", rootPath, err)
	}

	partitions := &shared.PartitionInfo{
		Disk:        diskID,
		EFIPart:     efiPath,
		EFIUUID:     efiUUID,
		RootPart:    rootPath,
		RootUUID:    rootUUID,
		MountPoints: make(map[string]string),
	}
	if fsType == shared.FileSystemBtrfs {
		for _, sv := range shared.GetBtrfsSubvolumes() {
			if sv.MountPoint != "/" {
				partitions.MountPoints[sv.MountPoint] = rootPath
			}
		}
	}
	return partitions, nil
}

// FormatPartition wipes any existing filesystem signatures on path and formats
// it with the requested filesystem and volume label. Shared by the automatic
// format step and the manual partition step.
func FormatPartition(path, fstype, label string) error {
	// Wipe filesystem signatures before formatting
	wipeOut, err := primitives.Run("wipefs", "--all", "-f", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("wipefs %s failed: %w: %s", path, err, string(wipeOut))
	}

	// Format with appropriate tool
	var cmd *exec.Cmd
	switch fstype {
	case shared.FileSystemVfat:
		cmd = primitives.Run("mkfs.fat", "-F", "32", "-n", label, path)
	case shared.FileSystemExt4:
		cmd = primitives.Run("mkfs.ext4", "-F", "-E", "lazy_itable_init=0", "-L", label, path)
	case shared.FileSystemBtrfs:
		cmd = primitives.Run("mkfs.btrfs", "-f", "-L", label, path)
	default:
		return fmt.Errorf("unsupported filesystem: %s", fstype)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("format %s failed: %w: %s", path, err, string(out))
	}
	return nil
}

// SetupBtrfsSubvolumes creates the standard btrfs subvolume scheme on the
// mounted root partition and sets the default subvolume. Shared by the
// automatic format step and the manual partition step.
func SetupBtrfsSubvolumes(ctx *shared.InstallContext, rootPath string, subvolumes []shared.BtrfsSubvolume) error {
	tmp := shared.BtrfsSetupTmpMount

	_ = primitives.Run("mkdir", "-p", tmp).Run()
	cmd := primitives.Run("mount", "-o", fmt.Sprintf("subvolid=%d", shared.BtrfsTopLevelSubvolumeID), rootPath, tmp)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mount btrfs top-level: %w", err)
	}
	defer func() { _ = primitives.Run("umount", tmp).Run() }()

	for _, sv := range subvolumes {
		cmd = primitives.Run("btrfs", "subvolume", "create", "-p", tmp+"/"+sv.Name)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("create subvolume %s: %w: %s", sv.Name, err, string(out))
		}
	}

	cmd = primitives.Run("btrfs", "subvolume", "list", tmp)
	listOut, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("list subvolumes: %w", err)
	}
	var targetID string
	for _, line := range strings.Split(strings.TrimSpace(string(listOut)), "\n") {
		// btrfs subvolume list emits lines like "ID 258 gen ... top level 5 path @".
		// Match the subvolume whose path is exactly "@" and take its ID (field 1).
		if strings.HasSuffix(strings.TrimSpace(line), "path @") {
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
	cmd = primitives.Run("btrfs", "subvolume", "set-default", targetID, tmp)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("set default subvolume: %w: %s", err, string(out))
	}

	for _, sv := range subvolumes {
		if sv.Compression == "" {
			continue
		}
		cmd = primitives.Run("btrfs", "property", "set", tmp+"/"+sv.Name, "compression", sv.Compression)
		if _, err := cmd.CombinedOutput(); err != nil {
			if ctx.Logger != nil {
				ctx.Logger.Error("Warning: compression on %s: %v", sv.Name, err)
			}
		}
	}

	_ = primitives.Run("rmdir", tmp).Run()
	return nil
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
	cmd := primitives.Run("blkid", "-s", "UUID", "-o", "value", partition)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("blkid %s: %w", partition, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// partitionTypes returns a human-readable list of partition types found on the disk.
// Used in error messages to help the user understand why a required partition was not found.
func partitionTypes(parts []shared.PartitionEntry) string {
	if len(parts) == 0 {
		return "none"
	}
	seen := map[string]bool{}
	var types []string
	for _, p := range parts {
		t := p.Fstype
		if t == "" {
			t = p.Type
		}
		if t == "" {
			t = "unknown"
		}
		if !seen[t] {
			seen[t] = true
			types = append(types, t)
		}
	}
	return strings.Join(types, ", ")
}

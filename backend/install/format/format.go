package format

import (
	"fmt"
	"strings"

	"aelum-installer/backend/install/luks"
	"aelum-installer/backend/install/lvm"
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
		{Name: shared.PartitionLayoutEFI, SizeMB: shared.EFIPartitionSizeMB, Fstype: shared.FileSystemVfat, MountPoint: shared.ESPMountPoint},
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

	// A previous LVM install leaves the `aelum` volume group active on the
	// target. Its device-mapper holders keep the partitions busy, so mkfs and
	// cryptsetup — which open with O_EXCL — fail with EBUSY. Remove the group
	// before partitioning (the manual path does the same); its PV label is
	// wiped by FormatPartition's wipefs on the new partitions.
	lvm.Teardown(shared.LvmVolumeGroup, nil)

	// Clear partition table (GPT backup, etc.) for a clean slate
	clearOut, err := primitives.Run("sgdisk", "-Z", diskID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("sgdisk -Z failed: %w: %s", err, string(clearOut))
	}

	// Create partitions. Start/end are left to sgdisk's defaults (0): it picks
	// the first 1 MiB-aligned usable sector, then the start/end of the largest
	// free block. This keeps the layout correct regardless of the disk's logical
	// sector size (512e vs 4Kn) with no sector arithmetic on our side.
	var cmd *primitives.Command
	var out []byte
	for i, p := range partitions {
		num := i + 1

		var args []string
		if p.SizeMB > 0 {
			args = []string{"-n", fmt.Sprintf("%d:0:+%dM", num, p.SizeMB)}
		} else {
			args = []string{"-n", fmt.Sprintf("%d:0:0", num)}
		}

		switch p.Fstype {
		case shared.FileSystemVfat:
			args = append(args, "-t", fmt.Sprintf("%d:%s", num, shared.GPTTypeGUIDs.ESP))
		case shared.FileSystemExt4, shared.FileSystemBtrfs:
			// The root partition hosts a filesystem directly, or an LVM physical
			// volume when LVM is enabled (its type GUID is Linux LVM).
			guid := shared.GPTTypeGUIDs.LinuxRootX86_64
			if p.Name == shared.PartitionLayoutRoot && ctx.Lvm {
				guid = shared.GPTTypeGUIDs.LinuxLVM
			}
			args = append(args, "-t", fmt.Sprintf("%d:%s", num, guid))
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

	// Refresh the kernel partition table before touching any partition node.
	// sgdisk issues BLKRRPART, but the /dev entries are created asynchronously
	// by udev; partprobe waits for the kernel/udev to expose the new partitions.
	// Formatting /dev/vda1 before that can fail with "No such file or directory".
	cmd = primitives.Run("partprobe", diskID)
	probeOut, probeErr := cmd.CombinedOutput()
	if probeErr != nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("partprobe failed: %v (%s)", probeErr, string(probeOut))
		}
		return fmt.Errorf("partprobe: %w", probeErr)
	}

	// Format EFI
	if err := FormatPartition(shared.DerivePartitionPath(diskID, 1), shared.FileSystemVfat, "EFI"); err != nil {
		return err
	}

	part2 := shared.DerivePartitionPath(diskID, 2)

	// Create the root filesystem. When LUKS is enabled a LUKS2 header is written
	// and the filesystem (and btrfs subvolumes) is created on the decrypted
	// mapper, then the mapper is closed again (mount reopens it). When LVM is
	// enabled the filesystem lives on logical volumes in the `aelum` volume
	// group instead of directly on the partition/mapper. The EFI partition is
	// always plaintext. SetupRootFilesystem/EncryptRoot return the device that
	// will be mounted as root (the root LV when LVM, else the mapper/partition).
	var setup RootSetup
	if ctx.Luks {
		if len(ctx.LuksPassphrase) == 0 {
			return fmt.Errorf("LUKS enabled but no passphrase set")
		}
		setup, err = EncryptRoot(ctx, part2, string(ctx.LuksPassphrase), fsType)
	} else {
		setup, err = SetupRootFilesystem(ctx, part2, fsType)
	}
	if err != nil {
		return err
	}

	// Find the EFI partition and build the partition info. The root device, its
	// UUID, and the subvolume mount points come from the setup above.
	diskPartitions, err := shared.GetPartitions(diskID)
	if err != nil {
		return fmt.Errorf("get partitions: %w", err)
	}
	efi, _ := findPartitionsByFstype(diskPartitions)
	if efi == nil {
		efi, _ = findPartitionsByIndex(diskPartitions)
	}
	if efi == nil {
		types := partitionTypes(diskPartitions)
		return fmt.Errorf("EFI (vfat) partition not found — found partitions: %s", types)
	}

	info, err := NewPartitionInfo(ctx, diskID, efi.Path, part2, setup, fsType)
	if err != nil {
		return err
	}
	ctx.Partitions = info

	return nil
}

// RootSetup describes the root filesystem produced by the format step.
type RootSetup struct {
	// RootDevice is the device mounted as root: the root LV when LVM is on,
	// otherwise the decrypted LUKS mapper or the raw partition.
	RootDevice string
	// RootUUID is the filesystem UUID of RootDevice (used by fstab).
	RootUUID string
	// HomeLV / HomeUUID are the LVM home logical volume and its filesystem
	// UUID. Empty when LVM is disabled.
	HomeLV   string
	HomeUUID string
	// LuksUUID is the LUKS header UUID of the raw partition. Empty when LUKS is
	// disabled.
	LuksUUID string
}

// EncryptRoot runs the full LUKS (and, when enabled, LVM) sequence on the raw
// root partition:
//
//  1. luksFormat writes a LUKS2 header (keyed by passphrase).
//  2. luksOpen creates the decrypted mapper device.
//  3. The filesystem (and, when LVM is enabled, the volume group with root +
//     home LVs) is created on the mapper.
//  4. The mapper's filesystem UUID and the LUKS header UUID are read.
//  5. The mapper is closed again; mount reopens it during install.
//
// It returns the RootSetup describing the mounted root device, its UUID, the
// LVM home LV (when enabled), and the LUKS header UUID (for the boot cmdline).
func EncryptRoot(ctx *shared.InstallContext, rawPart, passphrase, fsType string) (setup RootSetup, err error) {
	mapperPath := shared.LuksMapperPath

	if err := luks.LuksFormat(ctx.Logger, rawPart, passphrase); err != nil {
		return RootSetup{}, err
	}
	if err := luks.LuksOpen(rawPart, shared.LuksMapperName, passphrase); err != nil {
		return RootSetup{}, fmt.Errorf("opening LUKS device (validates passphrase): %w", err)
	}
	// Ensure the mapper is closed even if a later step fails, so the device
	// doesn't leak between install retries.
	defer func() {
		if cerr := luks.LuksClose(shared.LuksMapperName); cerr != nil && ctx.Logger != nil {
			ctx.Logger.Info("Warning: closing LUKS mapper: %v", cerr)
		}
	}()

	// Create the filesystem (and LVM volume group) on the decrypted mapper, not
	// the raw partition.
	setup, err = SetupRootFilesystem(ctx, mapperPath, fsType)
	if err != nil {
		return RootSetup{}, err
	}

	// Read the LUKS header UUID (from the raw partition) for the boot cmdline.
	luksUUID, err := luks.LuksUUID(rawPart)
	if err != nil {
		return RootSetup{}, err
	}
	setup.LuksUUID = luksUUID

	if ctx.Logger != nil {
		ctx.Logger.Info("Encrypted root: device=%s fsUUID=%s luksUUID=%s", setup.RootDevice, setup.RootUUID, setup.LuksUUID)
	}
	return setup, nil
}

// SetupRootFilesystem formats device as the root filesystem and returns the
// RootSetup the mount/install/configure steps consume. When LVM is enabled it
// creates the `aelum` volume group on device with root + home LVs; otherwise it
// formats device in place. The btrfs paths create the appropriate subvolume
// scheme (for LVM, the root LV carries every subvolume except @home, which the
// home LV carries).
func SetupRootFilesystem(ctx *shared.InstallContext, device, fsType string) (RootSetup, error) {
	if ctx.Lvm {
		// The root and home LVs use the selected filesystem: btrfs (the root LV
		// carries the subvolume scheme and the home LV carries @home) or ext4.
		return applyLvm(ctx, device, fsType)
	}
	if err := FormatPartition(device, fsType, "aelum"); err != nil {
		return RootSetup{}, err
	}
	uuid, err := ReadPartitionUUID(device)
	if err != nil {
		return RootSetup{}, fmt.Errorf("blkid %s: %w", device, err)
	}
	if fsType == shared.FileSystemBtrfs {
		if err := SetupBtrfsSubvolumes(ctx, device, shared.GetBtrfsSubvolumes()); err != nil {
			return RootSetup{}, err
		}
	}
	return RootSetup{RootDevice: device, RootUUID: uuid}, nil
}

// applyLvm creates the LVM volume group on device and formats the root and home
// logical volumes. The formatter closes over the format helpers so lvm never
// imports format (format imports lvm — a direct call would be an import cycle).
func applyLvm(ctx *shared.InstallContext, device, fsType string) (RootSetup, error) {
	formatter := func(dev, fs string, subvols []shared.BtrfsSubvolume) error {
		if err := FormatPartition(dev, fs, "aelum"); err != nil {
			return err
		}
		if fs == shared.FileSystemBtrfs && len(subvols) > 0 {
			return SetupBtrfsSubvolumes(ctx, dev, subvols)
		}
		return nil
	}
	lvs, err := lvm.Apply(device, lvm.Params{FileSystem: fsType}, formatter)
	if err != nil {
		return RootSetup{}, err
	}
	if len(lvs) < 2 {
		return RootSetup{}, fmt.Errorf("lvm: expected root and home LVs, got %d", len(lvs))
	}
	rootLV, homeLV := lvs[0].Path, lvs[1].Path

	rootUUID, err := ReadPartitionUUID(rootLV)
	if err != nil {
		return RootSetup{}, fmt.Errorf("blkid %s: %w", rootLV, err)
	}
	homeUUID, err := ReadPartitionUUID(homeLV)
	if err != nil {
		return RootSetup{}, fmt.Errorf("blkid %s: %w", homeLV, err)
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("LVM root: rootLV=%s homeLV=%s", rootLV, homeLV)
	}
	return RootSetup{RootDevice: rootLV, RootUUID: rootUUID, HomeLV: homeLV, HomeUUID: homeUUID}, nil
}

// applyRootSetup writes the root device mapping (LUKS/LVM/plain) and the
// subvolume mount-point map into info, given the RootSetup produced by
// SetupRootFilesystem or EncryptRoot. Shared by the automatic and manual paths.
func applyRootSetup(info *shared.PartitionInfo, ctx *shared.InstallContext, setup RootSetup, fsType string) {
	info.RootUUID = setup.RootUUID
	if ctx.Luks {
		info.RootMapper = shared.LuksMapperPath
		info.LuksUUID = setup.LuksUUID
	}
	if ctx.Lvm {
		// Root LV carries every subvolume except @home; the home LV carries
		// @home. The mount step reads the per-mount-point device from this map.
		// An ext4 LVM layout has no subvolumes: root is the root LV and /home is
		// the home LV, both mounted directly by the mount step (no map entries).
		info.RootLV = setup.RootDevice
		info.HomeLV = setup.HomeLV
		info.HomeUUID = setup.HomeUUID
		if fsType == shared.FileSystemBtrfs {
			for _, sv := range shared.GetLvmBtrfsSubvolumes() {
				if sv.MountPoint != "/" {
					info.MountPoints[sv.MountPoint] = setup.RootDevice
				}
			}
			for _, sv := range shared.GetLvmHomeSubvolumes() {
				if sv.MountPoint != "/" {
					info.MountPoints[sv.MountPoint] = setup.HomeLV
				}
			}
		}
		return
	}
	if fsType == shared.FileSystemBtrfs {
		for _, sv := range shared.GetBtrfsSubvolumes() {
			if sv.MountPoint != "/" {
				// Subvolumes live on the mapper device when LUKS is enabled.
				info.MountPoints[sv.MountPoint] = setup.RootDevice
			}
		}
	}
}

// NewPartitionInfo assembles the installed system's PartitionInfo from the
// discovered EFI partition, the raw root partition, and the RootSetup produced
// by SetupRootFilesystem or EncryptRoot. It reads the ESP UUID and applies the
// LUKS/LVM/plain root mapping plus the subvolume mount-point map.
func NewPartitionInfo(ctx *shared.InstallContext, diskID, efiPath, rootPart string, setup RootSetup, fsType string) (*shared.PartitionInfo, error) {
	efiUUID, err := ReadPartitionUUID(efiPath)
	if err != nil {
		return nil, fmt.Errorf("blkid %s: %w", efiPath, err)
	}
	info := &shared.PartitionInfo{
		Disk:        diskID,
		EFIPart:     efiPath,
		EFIUUID:     efiUUID,
		RootPart:    rootPart,
		MountPoints: make(map[string]string),
	}
	applyRootSetup(info, ctx, setup, fsType)
	return info, nil
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
	var cmd *primitives.Command
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
	// Prefer the subvolume mounted at "/" (usually @) as the default; fall back
	// to the first one for sets without a root subvolume (e.g. the LVM home LV,
	// which only carries @home).
	defaultName := ""
	for _, sv := range subvolumes {
		if sv.MountPoint == "/" {
			defaultName = sv.Name
			break
		}
	}
	if defaultName == "" && len(subvolumes) > 0 {
		defaultName = subvolumes[0].Name
	}
	var targetID string
	for _, line := range strings.Split(strings.TrimSpace(string(listOut)), "\n") {
		// btrfs subvolume list emits lines like "ID 258 gen ... top level 5 path @".
		// Match the subvolume whose path is exactly the default name and take its
		// ID (field 1).
		if strings.HasSuffix(strings.TrimSpace(line), "path "+defaultName) {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				targetID = fields[1]
				break
			}
		}
	}
	if targetID == "" {
		return fmt.Errorf("subvolume %s not found in %s", defaultName, tmp)
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

	// Unmount explicitly so a failure is reported (a leaked temp mount would
	// keep the device busy for the mount/cleanup steps). The deferred unmount
	// above remains as a safety net for the early-error returns.
	if out, err := primitives.Run("umount", tmp).CombinedOutput(); err != nil {
		if !strings.Contains(strings.ToLower(string(out)), "not mounted") {
			return fmt.Errorf("unmount btrfs top-level %s: %w: %s", tmp, err, strings.TrimSpace(string(out)))
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

// ReadPartitionUUID returns the filesystem UUID of a device (via blkid),
// trimmed of surrounding whitespace. An empty UUID is treated as an error: a
// device without one cannot be referenced by fstab or the boot entry, and
// silently returning "" would defer the failure to a much later step.
func ReadPartitionUUID(partition string) (string, error) {
	cmd := primitives.Run("blkid", "-s", "UUID", "-o", "value", partition)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("blkid %s: %w", partition, err)
	}
	uuid := strings.TrimSpace(string(out))
	if uuid == "" {
		return "", fmt.Errorf("blkid %s returned no filesystem UUID (format may have failed)", partition)
	}
	return uuid, nil
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

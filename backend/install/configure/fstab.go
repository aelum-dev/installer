package configure

import (
	"fmt"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// fstab generation
// ============================================================================

// GenerateFstab generates the fstab file for the installed system.
func GenerateFstab(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}
	if ctx.DiskID == "" {
		return fmt.Errorf("disk ID not set. Run --format-disk <disk> first")
	}
	if ctx.Partitions == nil {
		return fmt.Errorf("partitions not initialized. Run --format-disk <disk> first")
	}
	if ctx.FileSystem == "" {
		return fmt.Errorf("filesystem not set. Run --format-disk <disk> first")
	}
	if ctx.Partitions.EFIUUID == "" || ctx.Partitions.RootUUID == "" {
		return fmt.Errorf("partition UUIDs not set. FormatDisk may have failed")
	}

	p := ctx.Partitions

	if ctx.Lvm {
		return writeLvmFstab(ctx)
	}
	if ctx.FileSystem == shared.FileSystemBtrfs {
		return writeBtrfsFstab(ctx)
	}

	fstabContent := generateExt4Fstab(ctx)
	if err := primitives.WriteFile("/etc/fstab", fstabContent); err != nil {
		return err
	}
	return verifyFstab(p.RootUUID, p.EFIUUID, shared.InstallID(p.RootUUID))
}

func generateBtrfsFstab(ctx *shared.InstallContext) string {
	p := ctx.Partitions
	subvolumes := shared.GetBtrfsSubvolumes()

	var lines []string
	lines = append(lines, "# <file system>                             <mount point>  <type>  <options>               <dump>  <pass>")
	lines = append(lines, efiFstabLines(p, shared.InstallID(p.RootUUID))...)

	for _, sv := range subvolumes {
		opts := primitives.BuildMountOpts(&sv)
		lines = append(lines, fmt.Sprintf("UUID=%s  %s  btrfs   %s  0 0", p.RootUUID, sv.MountPoint, opts))
	}

	return strings.Join(lines, "\n") + "\n"
}

func generateExt4Fstab(ctx *shared.InstallContext) string {
	p := ctx.Partitions
	rootFstype := "ext4"
	if ctx.FileSystem != "" {
		rootFstype = ctx.FileSystem
	}
	lines := []string{
		"# <file system>                             <mount point>  <type>  <options>       <dump>  <pass>",
		fmt.Sprintf("UUID=%s  /              %s    %s        0  1", p.RootUUID, rootFstype, shared.DefaultMountOptions),
	}
	lines = append(lines, efiFstabLines(p, shared.InstallID(p.RootUUID))...)
	return strings.Join(lines, "\n") + "\n"
}

// efiFstabLines returns the fstab entries that give the installation its own
// boot directory on the ESP: the ESP is mounted at /efi, and this
// installation's subdirectory is bind-mounted at /boot. Keeping /boot a bind
// mount means pacman and mkinitcpio keep writing to /boot unchanged, while the
// files physically live in aelum/<id>/ and stay isolated from another install.
// x-systemd.requires-mounts-for makes the bind wait for the ESP mount.
func efiFstabLines(p *shared.PartitionInfo, id string) []string {
	src := shared.InstalledESPMountPoint + "/" + shared.InstallBootDir(id)
	return []string{
		fmt.Sprintf("UUID=%s  %s  vfat  %s  0 2", p.EFIUUID, shared.InstalledESPMountPoint, shared.EfiPartitionUmask),
		fmt.Sprintf("%s  /boot  none  bind,x-systemd.requires-mounts-for=%s  0 0", src, shared.InstalledESPMountPoint),
	}
}

func writeBtrfsFstab(ctx *shared.InstallContext) error {
	fstabContent := generateBtrfsFstab(ctx)
	if err := primitives.WriteFile("/etc/fstab", fstabContent); err != nil {
		return err
	}
	return verifyFstab(ctx.Partitions.RootUUID, ctx.Partitions.EFIUUID, shared.InstallID(ctx.Partitions.RootUUID))
}

// generateLvmFstab builds the fstab for an LVM layout. For btrfs it is the
// subvolume layout with /home split off onto the home logical volume: every
// subvolume except @home lives on the root LV (RootUUID), @home lives on the
// home LV (HomeUUID). For ext4 the root and home LVs are mounted directly at /
// and /home.
func generateLvmFstab(ctx *shared.InstallContext) string {
	if ctx.FileSystem != shared.FileSystemBtrfs {
		return generateLvmExt4Fstab(ctx)
	}

	p := ctx.Partitions

	var lines []string
	lines = append(lines, "# <file system>                             <mount point>  <type>  <options>               <dump>  <pass>")
	lines = append(lines, efiFstabLines(p, shared.InstallID(p.RootUUID))...)

	for _, sv := range shared.GetLvmBtrfsSubvolumes() {
		opts := primitives.BuildMountOpts(&sv)
		lines = append(lines, fmt.Sprintf("UUID=%s  %s  btrfs   %s  0 0", p.RootUUID, sv.MountPoint, opts))
	}
	for _, sv := range shared.GetLvmHomeSubvolumes() {
		opts := primitives.BuildMountOpts(&sv)
		lines = append(lines, fmt.Sprintf("UUID=%s  %s  btrfs   %s  0 0", p.HomeUUID, sv.MountPoint, opts))
	}

	return strings.Join(lines, "\n") + "\n"
}

// generateLvmExt4Fstab builds the fstab for an ext4 LVM layout: the root LV is
// mounted at / and the home LV at /home, both ext4.
func generateLvmExt4Fstab(ctx *shared.InstallContext) string {
	p := ctx.Partitions
	lines := []string{
		"# <file system>                             <mount point>  <type>  <options>       <dump>  <pass>",
		fmt.Sprintf("UUID=%s  /              ext4    %s        0  1", p.RootUUID, shared.DefaultMountOptions),
		fmt.Sprintf("UUID=%s  /home          ext4    %s        0  2", p.HomeUUID, shared.DefaultMountOptions),
	}
	lines = append(lines, efiFstabLines(p, shared.InstallID(p.RootUUID))...)
	return strings.Join(lines, "\n") + "\n"
}

func writeLvmFstab(ctx *shared.InstallContext) error {
	fstabContent := generateLvmFstab(ctx)
	if err := primitives.WriteFile("/etc/fstab", fstabContent); err != nil {
		return err
	}
	return verifyFstab(ctx.Partitions.RootUUID, ctx.Partitions.EFIUUID, shared.InstallID(ctx.Partitions.RootUUID))
}

func verifyFstab(rootUUID, efiUUID, installID string) error {
	if rootUUID == "" {
		return fmt.Errorf("root UUID is empty — cannot verify fstab")
	}
	if efiUUID == "" {
		return fmt.Errorf("EFI UUID is empty — cannot verify fstab")
	}

	data, err := primitives.ReadFile(shared.MountRoot + "/etc/fstab")
	if err != nil {
		return fmt.Errorf("failed to read fstab: %w", err)
	}
	content := string(data)
	if len(content) == 0 || len(content) < shared.MinFstabContentLengthBytes {
		return fmt.Errorf("fstab is empty or too short (%d bytes)", len(content))
	}
	if !strings.Contains(content, rootUUID) {
		return fmt.Errorf("fstab missing root UUID %s (content length: %d bytes)", rootUUID, len(content))
	}
	if !strings.Contains(content, efiUUID) {
		return fmt.Errorf("fstab missing EFI UUID %s (content length: %d bytes)", efiUUID, len(content))
	}
	if !strings.Contains(content, "/ ") && !strings.Contains(content, "/\t") {
		return fmt.Errorf("fstab missing / mount point")
	}
	bootSrc := shared.InstalledESPMountPoint + "/" + shared.InstallBootDir(installID)
	if installID != "" && !strings.Contains(content, bootSrc) {
		return fmt.Errorf("fstab missing this installation's boot directory %s", bootSrc)
	}
	if !strings.Contains(content, "bind") {
		return fmt.Errorf("fstab missing the /boot bind mount")
	}
	return nil
}

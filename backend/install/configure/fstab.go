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

	if ctx.FileSystem == shared.FileSystemBtrfs {
		return writeBtrfsFstab(ctx)
	}

	fstabContent := generateExt4Fstab(ctx)
	if err := primitives.WriteFile("/etc/fstab", fstabContent); err != nil {
		return err
	}
	return verifyFstab(p.RootUUID, p.EFIUUID, shared.InstallID(p.RootUUID), p.HomeUUID)
}

func generateBtrfsFstab(ctx *shared.InstallContext) string {
	p := ctx.Partitions
	subvolumes := shared.GetBtrfsSubvolumes()

	var lines []string
	lines = append(lines, "# <file system>                             <mount point>  <type>  <options>               <dump>  <pass>")
	lines = append(lines, efiFstabLines(p, shared.InstallID(p.RootUUID))...)

	for i := range subvolumes {
		sv := &subvolumes[i]
		opts := primitives.BuildFstabMountOpts(sv)
		lines = append(lines, fmt.Sprintf("UUID=%s  %s  btrfs   %s  0 0", p.RootUUID, sv.MountPoint, opts))
		// The history of the subvolume that holds the system gets its own mount.
		// A snapshot never contains the subvolumes inside its parent, so once a
		// rollback has run the new live root has an empty .snapshots of its own:
		// this line is what makes the history reachable again, and it names the
		// container (@/.snapshots), which no rollback ever moves.
		if sv.SnapshotRoot {
			lines = append(lines, fmt.Sprintf("UUID=%s  %s  btrfs   %s  0 0", p.RootUUID,
				shared.BtrfsSnapshotsMountPoint, primitives.BuildSnapshotsMountOpts(sv)))
		}
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
	// LVM on ext4 keeps /home on its own logical volume, so a full root does not
	// take user data with it.
	if p.HomeUUID != "" {
		lines = append(lines, fmt.Sprintf("UUID=%s  /home          %s    %s        0  2", p.HomeUUID, rootFstype, shared.DefaultMountOptions))
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
	if err := verifyFstab(ctx.Partitions.RootUUID, ctx.Partitions.EFIUUID, shared.InstallID(ctx.Partitions.RootUUID), ""); err != nil {
		return err
	}
	// The root mount names no subvolume, so the only thing that decides which state
	// comes back after a rollback is the filesystem's default subvolume. If the
	// history is not mounted, the restored system cannot see it and snapper stops
	// working in it, so a fstab without that line is a broken install, not a
	// cosmetic one.
	data, err := primitives.ReadFile(shared.MountRoot + "/etc/fstab")
	if err != nil {
		return err
	}
	if !strings.Contains(string(data), shared.BtrfsSnapshotsMountPoint) {
		return fmt.Errorf("fstab is missing the %s mount (snapper's history would not be reachable after a rollback)", shared.BtrfsSnapshotsMountPoint)
	}
	return nil
}

func verifyFstab(rootUUID, efiUUID, installID, homeUUID string) error {
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
	if homeUUID != "" && !strings.Contains(content, homeUUID) {
		return fmt.Errorf("fstab missing home UUID %s (content length: %d bytes)", homeUUID, len(content))
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

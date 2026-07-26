package install

import (
	"fmt"
	"os/exec"
	"strings"

	"aestheticos-installer/backend/internal/shared"
)

// GenerateFstab generates the fstab file for the installed system using sudo.
// For btrfs subvolume layouts, manually writes fstab with subvolume mount options.
// This must be done after pacstrap and before chroot operations.
func GenerateFstab(ctx *InstallContext) error {
	if err := validateMountedSystem(ctx); err != nil {
		return err
	}

	// Validate preconditions
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

	// For btrfs, write custom fstab with subvolume entries
	if ctx.FileSystem == shared.FileSystemBtrfs {
		if ctx.Logger != nil {
			ctx.Logger.Info("Generating custom fstab for btrfs subvolumes")
		}
		return writeBtrfsFstab(ctx)
	}

	// Standard fstab generation for ext4 and other filesystems
	if ctx.Logger != nil {
		ctx.Logger.Info("Generating fstab for %s", p.RootPart)
	}

	// Write fstab manually (genfstab requires chroot, which we don't have here)
	// Determine root filesystem type from partition
	rootFstype := "ext4"
	if ctx.FileSystem != "" {
		rootFstype = ctx.FileSystem
	}
	fstabContent := fmt.Sprintf(`# <file system>                             <mount point>  <type>  <options>       <dump>  <pass>
UUID=%s  /              %s    %s        0  1
UUID=%s  /boot          vfat    %s 0 2
`, p.RootUUID, rootFstype, DefaultMountOptions, p.EFIUUID, EfiPartitionUmask)

	if err := writeFile(ctx, "/etc/fstab", fstabContent); err != nil {
		return err
	}

	// Verify fstab was written correctly
	if err := verifyFstab(ctx, p.RootUUID, p.EFIUUID); err != nil {
		return err
	}

	return nil
}

// writeBtrfsFstab writes a custom fstab for btrfs with subvolumes.
// All subvolumes share the root partition UUID, with subvol=<name> in mount options.
func writeBtrfsFstab(ctx *InstallContext) error {
	p := ctx.Partitions

	// Get subvolumes from shared definition
	subvolumes := shared.GetBtrfsSubvolumes()

	// Build fstab: one line per subvolume + EFI
	var lines []string
	lines = append(lines, "# <file system>                             <mount point>  <type>  <options>               <dump>  <pass>")

	for _, sv := range subvolumes {
		opts := fmt.Sprintf("%s%s,%s,%s", BtrfsSubvolumeMountPrefix, sv.Name, DefaultMountOptions, BtrfsAutodefragOption)
		if sv.Compression != "" {
			opts += ",compress=" + sv.Compression
		}
		lines = append(lines, fmt.Sprintf("UUID=%s  %s  btrfs   %s  0 0", p.RootUUID, sv.MountPoint, opts))
	}

	// EFI partition
	lines = append(lines, fmt.Sprintf("UUID=%s  /boot  vfat    %s  0  2", p.EFIUUID, EfiPartitionUmask))

	fstabContent := strings.Join(lines, "\n") + "\n"

	// Write fstab
	if err := writeFile(ctx, "/etc/fstab", fstabContent); err != nil {
		return err
	}

	// Verify fstab was written correctly
	if err := verifyFstab(ctx, p.RootUUID, p.EFIUUID); err != nil {
		return err
	}

	return nil
}

// verifyFstab reads /mnt/etc/fstab and verifies it contains the expected UUIDs.
// Returns an error if the fstab is empty or missing required entries.
func verifyFstab(ctx *InstallContext, rootUUID, efiUUID string) error {
	if ctx.Logger != nil {
		ctx.Logger.Info("Verifying fstab")
	}

	// Validate UUIDs are not empty before checking
	if rootUUID == "" {
		return fmt.Errorf("root UUID is empty — cannot verify fstab")
	}
	if efiUUID == "" {
		return fmt.Errorf("EFI UUID is empty — cannot verify fstab")
	}

	// Read fstab content
	cmd := exec.Command("sudo", ctx.Helper, "run", "cat", "/mnt/etc/fstab")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to read fstab: %w", err)
	}

	content := string(output)
	if len(content) == 0 || len(content) < shared.MinFstabContentLengthBytes {
		return fmt.Errorf("fstab is empty or too short (%d bytes)", len(content))
	}

	// Check for root UUID
	if !strings.Contains(content, rootUUID) {
		return fmt.Errorf("fstab missing root UUID %s (content length: %d bytes)", rootUUID, len(content))
	}

	// Check for EFI UUID
	if !strings.Contains(content, efiUUID) {
		return fmt.Errorf("fstab missing EFI UUID %s (content length: %d bytes)", efiUUID, len(content))
	}

	// Check for / mount point
	if !strings.Contains(content, "/ ") && !strings.Contains(content, "/\t") {
		return fmt.Errorf("fstab missing / mount point")
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Fstab verification passed")
	}

	return nil
}

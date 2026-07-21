package steps

import (
	"fmt"
	"os/exec"
)

// GenerateFstab generates the fstab file for the installed system using sudo.
// For btrfs subvolume layouts, manually writes fstab with subvolume mount options.
// This must be done after pacstrap and before chroot operations.
func GenerateFstab(ctx *InstallContext) error {
	if err := validateMountedSystem(ctx); err != nil {
		return err
	}

	if ctx.Partitions == nil {
		return fmt.Errorf("partition info not available. Run --format-disk <disk> first")
	}

	p := ctx.Partitions

	// For btrfs subvolume layouts, write custom fstab
	if p.LayoutName == "btrfs-subvolumes" {
		if ctx.Logger != nil {
			ctx.Logger.Info("Generating custom fstab for btrfs subvolumes")
		}
		return writeBtrfsFstab(ctx)
	}

	// Standard fstab generation for ext4 and other filesystems
	if ctx.Logger != nil {
		ctx.Logger.Info("Generating fstab for /mnt")
	}

	// Run genfstab -U /mnt and redirect output to /mnt/etc/fstab
	cmd := exec.Command("sudo", ctx.Helper, "run", "bash", "-c", "genfstab -U /mnt > /mnt/etc/fstab")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("genfstab failed: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	return nil
}

// writeBtrfsFstab writes a custom fstab for btrfs subvolume layouts.
// Uses UUIDs and subvolume mount options.
func writeBtrfsFstab(ctx *InstallContext) error {
	p := ctx.Partitions

	// Build fstab content
	fstabContent := fmt.Sprintf(`# <file system>                             <mount point>  <type>  <options>       <dump>  <pass>
UUID=%s  /              btrfs   defaults,subvol=@       0  0
UUID=%s  /boot          vfat    defaults,noatime        0  2
UUID=%s  /home          btrfs   defaults,subvol=@home   0  0
`, p.RootUUID, p.EFIUUID, p.RootUUID)

	// Write fstab
	cmd := exec.Command("sudo", ctx.Helper, "run", "bash", "-c", fmt.Sprintf("cat > /mnt/etc/fstab << 'EOF'\n%s\nEOF", fstabContent))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to write fstab: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	return nil
}

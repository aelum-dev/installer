package steps

import (
	"fmt"
	"os/exec"
	"strings"
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
		ctx.Logger.Info("Generating fstab for %s", p.RootPart)
	}

	// Write fstab manually (genfstab requires chroot, which we don't have here)
	// Determine root filesystem type from partition
	rootFstype := "ext4"
	if ctx.FileSystem != "" {
		rootFstype = ctx.FileSystem
	}
	fstabContent := fmt.Sprintf(`# <file system>                             <mount point>  <type>  <options>       <dump>  <pass>
UUID=%s  /              %s    defaults,noatime        0  1
UUID=%s  /boot          vfat    umask=0077 0 2
`, p.RootUUID, rootFstype, p.EFIUUID)

	cmd := exec.Command("sudo", ctx.Helper, "run", "bash", "-c", fmt.Sprintf("cat > /mnt/etc/fstab << 'EOF'\n%s\nEOF", fstabContent))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to write fstab: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	// Verify fstab was written correctly
	if err := verifyFstab(ctx, p.RootUUID, p.EFIUUID); err != nil {
		return err
	}

	return nil
}

// writeBtrfsFstab writes a custom fstab for btrfs subvolume layouts.
// Uses UUIDs and subvolume mount options.
//
// Layout:
//   - Root (@ subvolume): mounted with subvol=@, compress=zstd, autodefrag
//   - ESP (/boot): mounted with umask=0077 for security
//   - @home subvolume: mounted with subvol=@home, compress=zstd, autodefrag
//
// The @ subvolume was set as default in FormatDisk, but subvol=@ is specified
// explicitly in fstab for clarity and resilience (if default changes, boot still works).
func writeBtrfsFstab(ctx *InstallContext) error {
	p := ctx.Partitions

	// Build fstab content
	fstabContent := fmt.Sprintf(`# <file system>                             <mount point>  <type>  <options>               <dump>  <pass>
UUID=%s  /              btrfs   subvol=@,defaults,noatime,autodefrag,compress=zstd 0 0
UUID=%s  /boot          vfat    umask=0077              0  2
UUID=%s  /home          btrfs   subvol=@home,defaults,noatime,autodefrag,compress=zstd 0 0
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
	if len(content) == 0 || len(content) < 50 {
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

package install

import (
	"fmt"
	"os/exec"

	"aestheticos-installer/backend/internal/shared"
)

// SwapFilePermissions are the required permissions for swap files (kernel requirement).
const SwapFilePermissions = 0600

// CreateSwap creates a swap file on the installed system.
// For btrfs: creates /swap subvolume with nodatacow, then swap file inside it.
// For ext4: creates file with fallocate + chmod + mkswap.
func CreateSwap(ctx *InstallContext) error {
	if err := validateMountedSystem(ctx); err != nil {
		return err
	}

	if ctx.Logger != nil {
	}

	var swapFilePath string
	switch ctx.FileSystem {
	case shared.FileSystemBtrfs:
		swapFilePath = "/swap/swapfile"
		if err := createBtrfsSwapFile(ctx, swapFilePath); err != nil {
			return err
		}
	default:
		swapFilePath = "/swapfile"
		if err := createExt4SwapFile(ctx, swapFilePath); err != nil {
			return err
		}
	}

	// Enable swap
	if err := enableSwap(ctx, swapFilePath); err != nil {
		return err
	}

	if ctx.Logger != nil {
	}

	return nil
}

// createBtrfsSwapFile creates the swap file inside the already-mounted @swap subvolume.
// The subvolume is created, configured with nodatacow, and mounted by MountFilesystem.
func createBtrfsSwapFile(ctx *InstallContext, path string) error {
	if ctx.Logger != nil {
		ctx.Logger.Info("Creating btrfs swap file: %s (4G)", path)
	}

	// @swap subvolume is already mounted at /mnt/swap by MountFilesystem
	swapMountPoint := shared.MountRoot + path
	cmd := exec.Command("sudo", ctx.Helper, HelperCmdBtrfsMkswapfile, swapMountPoint, "4G")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create btrfs swap file: %w: %s", err, string(output))
	}

	return nil
}

// createExt4SwapFile creates a swap file using fallocate (fast) with dd fallback.
// Sets 0600 permissions and formats with mkswap (btrfs handles these via mkswapfile).
func createExt4SwapFile(ctx *InstallContext, path string) error {
	if ctx.Logger != nil {
	}

	// Fast path: fallocate
	cmd := exec.Command("sudo", ctx.Helper, HelperCmdFallocate, "-l", "4096M", shared.MountRoot+path)
	if err := cmd.Run(); err == nil {
		if ctx.Logger != nil {
		}
	} else {
		// Fallback: dd (works on all filesystems)
		if ctx.Logger != nil {
		}

		cmd = exec.Command("sudo", ctx.Helper, HelperCmdDd,
			"if=/dev/zero",
			"of="+shared.MountRoot+path,
			"bs=1M",
			"count=4096",
			"status=progress",
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to create swap file with dd: %w: %s", err, string(output))
		}

		if ctx.Logger != nil {
		}
	}

	// Set permissions (btrfs mkswapfile handles this automatically)
	if err := setSwapFilePermissions(ctx, path); err != nil {
		return err
	}

	// Format as swap (btrfs mkswapfile handles this automatically)
	if err := formatSwap(ctx, path); err != nil {
		return err
	}

	return nil
}

// setSwapFilePermissions sets the required permissions on the swap file.
func setSwapFilePermissions(ctx *InstallContext, path string) error {
	if ctx.Logger != nil {
	}

	cmd := exec.Command("sudo", ctx.Helper, HelperCmdChmod, "0600", shared.MountRoot+path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set swap file permissions: %w: %s", err, string(output))
	}

	return nil
}

// formatSwap formats the swap file with mkswap.
func formatSwap(ctx *InstallContext, path string) error {
	if ctx.Logger != nil {
	}

	cmd := exec.Command("sudo", ctx.Helper, HelperCmdMkswap, shared.MountRoot+path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to format swap file: %w: %s", err, string(output))
	}

	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}
	return nil
}

// enableSwap enables the swap file with swapon.
func enableSwap(ctx *InstallContext, path string) error {
	if ctx.Logger != nil {
	}

	cmd := exec.Command("sudo", ctx.Helper, HelperCmdSwapon, shared.MountRoot+path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to enable swap file: %w: %s", err, string(output))
	}

	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}
	return nil
}

// GetSwapFstabEntry returns the fstab line for the swap file.
// For btrfs: swap file is in /swap subvolume (nodatacow property avoids CoW),
// referenced via UUID with subvol=@swap mount option.
// For ext4: swap file is directly on root filesystem.
func GetSwapFstabEntry(ctx *InstallContext) string {
	if ctx.FileSystem == shared.FileSystemBtrfs {
		// btrfs: mount the @swap subvolume which contains the swap file
		return fmt.Sprintf("UUID=%s  /swap  btrfs  subvol=@swap,defaults,autodefrag  0  0", ctx.Partitions.RootUUID)
	}
	return "/swapfile  none  swap  defaults  0  0"
}

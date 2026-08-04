package install

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"aestheticos-installer/backend/internal/shared"
)

// SwapFilePermissions are the required permissions for swap files (kernel requirement).
const SwapFilePermissions = 0600

// Swap size constants (in bytes).
const (
	SwapMaxBytes = 16 * 1024 * 1024 * 1024 // 16 GiB cap
)

// meminfoKBRegex extracts the numeric value from a /proc/meminfo line (e.g. "MemTotal:       16384 kB").
var meminfoKBRegex = regexp.MustCompile(`^MemTotal:\s+(\d+)\s+kB$`)

// GetTotalRAMKB reads /proc/meminfo on the live ISO and returns total RAM in kB.
func GetTotalRAMKB() (int64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, fmt.Errorf("failed to read /proc/meminfo: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		matches := meminfoKBRegex.FindStringSubmatch(line)
		if matches != nil {
			kb, err := strconv.ParseInt(matches[1], 10, 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse meminfo value: %w", err)
			}
			return kb, nil
		}
	}
	return 0, fmt.Errorf("MemTotal not found in /proc/meminfo")
}

// CalculateSwapSize returns the recommended swap size in bytes.
// Rule: min(RAM/2, 16GiB).
func CalculateSwapSize(totalRAMKB int64) int64 {
	totalRAMBytes := totalRAMKB * 1024
	halfRAM := totalRAMBytes / 2
	if halfRAM > SwapMaxBytes {
		return SwapMaxBytes
	}
	return halfRAM
}

// formatSwapSize returns a human-readable swap size string for commands
// (e.g. "8G", "4096M"). Uses GiB when >= 1GiB, otherwise MiB.
func formatSwapSize(bytes int64) string {
	giB := int(bytes / (1024 * 1024 * 1024))
	if giB >= 1 {
		return fmt.Sprintf("%dG", giB)
	}
	miB := int(bytes / (1024 * 1024))
	return fmt.Sprintf("%dM", miB)
}

// parseSwapSize converts a size string (e.g. "8G", "4096M") back to bytes.
func parseSwapSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "G") {
		v, err := strconv.ParseInt(strings.TrimSuffix(s, "G"), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid size %q: %w", s, err)
		}
		return v * 1024 * 1024 * 1024, nil
	}
	if strings.HasSuffix(s, "M") {
		v, err := strconv.ParseInt(strings.TrimSuffix(s, "M"), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid size %q: %w", s, err)
		}
		return v * 1024 * 1024, nil
	}
	return 0, fmt.Errorf("unsupported size format %q (expected G or M suffix)", s)
}

// CreateSwap creates a swap file on the installed system.
// For btrfs: creates /swap subvolume with nodatacow, then swap file inside it.
// For ext4: creates file with fallocate + chmod + mkswap.
func CreateSwap(ctx *InstallContext) error {
	if err := validateMountedSystem(ctx); err != nil {
		return err
	}


	// Detect RAM and calculate swap size
	totalRAMKB, err := GetTotalRAMKB()
	if err != nil {
		return fmt.Errorf("failed to detect RAM: %w", err)
	}
	swapSizeBytes := CalculateSwapSize(totalRAMKB)
	swapSizeStr := formatSwapSize(swapSizeBytes)
	if ctx.Logger != nil {
		ctx.Logger.Info("Detected %d kB RAM, using %s swap", totalRAMKB, swapSizeStr)
	}

	var swapFilePath string
	switch ctx.FileSystem {
	case shared.FileSystemBtrfs:
		swapFilePath = "/swap/swapfile"
		if err := createBtrfsSwapFile(ctx, swapFilePath, swapSizeStr); err != nil {
			return err
		}
	default:
		swapFilePath = "/swapfile"
		if err := createExt4SwapFile(ctx, swapFilePath, swapSizeStr); err != nil {
			return err
		}
	}

	// Enable swap
	if err := enableSwap(ctx, swapFilePath); err != nil {
		return err
	}


	return nil
}

// createBtrfsSwapFile creates the swap file inside the already-mounted @swap subvolume.
// The subvolume is created, configured with nodatacow, and mounted by MountFilesystem.
func createBtrfsSwapFile(ctx *InstallContext, path, size string) error {
	if ctx.Logger != nil {
		ctx.Logger.Info("Creating btrfs swap file: %s (%s)", path, size)
	}

	// @swap subvolume is already mounted at /mnt/swap by MountFilesystem
	swapMountPoint := shared.MountRoot + path
	cmd := ctx.run(HelperCmdBtrfsMkswapfile, swapMountPoint, size)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create btrfs swap file: %w: %s", err, string(output))
	}

	return nil
}

// createExt4SwapFile creates a swap file using fallocate (fast) with dd fallback.
// Sets 0600 permissions and formats with mkswap (btrfs handles these via mkswapfile).
func createExt4SwapFile(ctx *InstallContext, path, size string) error {
	// Parse size string back to bytes for fallocate/dd
	sizeBytes, err := parseSwapSize(size)
	if err != nil {
		return fmt.Errorf("failed to parse swap size %q: %w", size, err)
	}

	// Fast path: fallocate
	cmd := ctx.run(HelperCmdRun, "fallocate", "-l", size, shared.MountRoot+path)
	if err := cmd.Run(); err == nil {
	} else {
		// Fallback: dd (works on all filesystems)
		miB := int(sizeBytes / (1024 * 1024))
		cmd = ctx.run(HelperCmdRun,
			"if=/dev/zero",
			"of="+shared.MountRoot+path,
			"bs=1M",
			"count="+strconv.Itoa(miB),
			"status=progress",
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to create swap file with dd: %w: %s", err, string(output))
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

	cmd := ctx.run(HelperCmdRun, "chmod", "0600", shared.MountRoot+path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set swap file permissions: %w: %s", err, string(output))
	}

	return nil
}

// formatSwap formats the swap file with mkswap.
func formatSwap(ctx *InstallContext, path string) error {

	cmd := ctx.run(HelperCmdRun, "mkswap", shared.MountRoot+path)
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

	cmd := ctx.run(HelperCmdRun, "swapon", shared.MountRoot+path)
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
// For btrfs: returns the swap file entry (the @swap mount is handled separately in fstab).
// For ext4: swap file is directly on root filesystem.
func GetSwapFstabEntry(ctx *InstallContext) string {
	return "/swapfile  none  swap  defaults  0  0"
}

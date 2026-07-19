package steps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ConfigureBootloader sets up systemd-boot bootloader using sudo.
func ConfigureBootloader(diskID, helper, username, password, language string) error {
	if diskID == "" {
		diskID = "/dev/sda"
	}

	esp := "/boot/efi"

	// Install systemd-boot to ESP
	cmd := exec.Command("sudo", helper, "bootctl-install", esp)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bootctl install failed: %w: %s", err, string(output))
	}

	// Create bootloader entry directory
	entryDir := filepath.Join(esp, "boot", "loader", "entries")
	if err := os.MkdirAll(entryDir, 0755); err != nil {
		return fmt.Errorf("failed to create boot entry directory: %w", err)
	}

	// Find the root partition UUID on the selected disk
	rootUUID, err := findRootPartitionUUID(diskID)
	if err != nil {
		return fmt.Errorf("failed to find root partition UUID: %w", err)
	}

	// Create the boot entry configuration
	entryContent := fmt.Sprintf(`title   Aesthetic OS
linux   /vmlinuz-linux
initrd  /initramfs-linux.img
options root=UUID=%s rw
`, rootUUID)

	entryPath := filepath.Join(entryDir, "aesthetic-os.conf")
	if err := os.WriteFile(entryPath, []byte(entryContent), 0644); err != nil {
		return fmt.Errorf("failed to write boot entry: %w", err)
	}

	return nil
}

// findRootPartitionUUID finds the UUID of the root partition on the selected disk.
// It assumes the second partition (e.g., /dev/sda2) is the root partition.
func findRootPartitionUUID(diskID string) (string, error) {
	// Get partition list for the disk
	cmd := exec.Command("lsblk", "-d", "-o", "NAME,TYPE,FSTYPE", diskID)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list partitions: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return "", fmt.Errorf("no partitions found on %s", diskID)
	}

	// Parse the second partition (index 1, skipping header)
	partition := lines[1]
	fields := strings.Fields(partition)
	if len(fields) < 1 {
		return "", fmt.Errorf("failed to parse partition info")
	}

	partName := fields[0]
	partPath := "/dev/" + partName

	// Get UUID of the partition
	cmd = exec.Command("blkid", "-s", "UUID", "-o", "value", partPath)
	uuid, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get UUID for %s: %w", partPath, err)
	}

	return strings.TrimSpace(string(uuid)), nil
}

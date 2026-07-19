package steps

import (
	"fmt"
	"os/exec"
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

	// Find the root partition UUID on the selected disk
	rootUUID, err := findRootPartitionUUID(diskID)
	if err != nil {
		return fmt.Errorf("failed to find root partition UUID: %w", err)
	}

	// Create the boot entry configuration via chroot
	entryContent := fmt.Sprintf(`title   Aesthetic OS
linux   /vmlinuz-linux
initrd  /initramfs-linux.img
options root=UUID=%s rw
`, rootUUID)

	cmd = exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf("mkdir -p /boot/efi/boot/loader/entries && cat > /boot/efi/boot/loader/entries/aesthetic-os.conf << 'BOOTEOF'\n%s\nBOOTEOF", entryContent))
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to write boot entry: %w: %s", err, string(output))
	}

	// Set default boot entry
	cmd = exec.Command("sudo", helper, "bootctl-set-default", "/boot/efi")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set default boot entry: %w: %s", err, string(output))
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

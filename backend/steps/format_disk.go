package steps

import (
	"fmt"
	"os/exec"
)

// FormatDisk formats the target disk with ext4 filesystem.
func FormatDisk(diskID string) error {
	if diskID == "" {
		diskID = "/dev/sda"
	}
	cmd := exec.Command("mkfs.ext4", "-F", "-L", "aesthetic-os", diskID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("format disk failed: %w: %s", err, string(output))
	}
	return nil
}

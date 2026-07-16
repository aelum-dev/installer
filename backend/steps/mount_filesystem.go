package steps

import (
	"fmt"
	"os/exec"
)

// MountFilesystem mounts the formatted disk.
func MountFilesystem(diskID string) error {
	if diskID == "" {
		diskID = "/dev/sda"
	}
	cmd := exec.Command("mount", diskID, "/mnt")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount failed: %w: %s", err, string(output))
	}
	return nil
}

package steps

import (
	"fmt"
	"os/exec"
)

// InstallBase installs the Arch Linux base system.
func InstallBase(diskID string) error {
	cmd := exec.Command("pacstrap", "/mnt", "base", "linux", "linux-firmware", "base-devel")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pacstrap failed: %w: %s", err, string(output))
	}
	return nil
}

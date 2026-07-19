package steps

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// FormatDisk formats the target disk with ext4 filesystem using pkexec.
func FormatDisk(diskID string, username, password, language string) error {
	if diskID == "" {
		diskID = "/dev/sda"
	}

	// Get the path to the root-helper binary (next to the main binary)
	exe, err := exec.LookPath("aesthetic-os-installer")
	if err != nil {
		exe = "aesthetic-os-installer"
	}
	dir := filepath.Dir(exe)
	helper := filepath.Join(dir, "root-helper")

	cmd := exec.Command("pkexec", helper, "format", diskID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("format disk failed: %w: %s", err, string(output))
	}
	return nil
}

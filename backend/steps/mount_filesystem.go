package steps

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// MountFilesystem mounts the formatted disk using pkexec.
func MountFilesystem(diskID string, username, password, language string) error {
	if diskID == "" {
		diskID = "/dev/sda"
	}

	exe, err := exec.LookPath("aesthetic-os-installer")
	if err != nil {
		exe = "aesthetic-os-installer"
	}
	dir := filepath.Dir(exe)
	helper := filepath.Join(dir, "root-helper")

	cmd := exec.Command("pkexec", helper, "mount", diskID, "/mnt")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount failed: %w: %s", err, string(output))
	}
	return nil
}

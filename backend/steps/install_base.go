package steps

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// InstallBase installs the Arch Linux base system using pkexec.
func InstallBase(diskID string, username, password, language string) error {
	exe, err := exec.LookPath("aesthetic-os-installer")
	if err != nil {
		exe = "aesthetic-os-installer"
	}
	dir := filepath.Dir(exe)
	helper := filepath.Join(dir, "root-helper")

	cmd := exec.Command("pkexec", helper, "pacstrap", "/mnt", "base", "linux", "linux-firmware", "base-devel")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pacstrap failed: %w: %s", err, string(output))
	}
	return nil
}

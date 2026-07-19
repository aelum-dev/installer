package steps

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// InstallPackages installs additional packages using pkexec.
func InstallPackages(diskID string, username, password, language string) error {
	// Default packages to install
	packages := []string{"networkmanager", "sudo", "vim", "nano", "wget", "curl"}

	exe, err := exec.LookPath("aesthetic-os-installer")
	if err != nil {
		exe = "aesthetic-os-installer"
	}
	dir := filepath.Dir(exe)
	helper := filepath.Join(dir, "root-helper")

	args := []string{"/mnt"}
	args = append(args, packages...)
	cmd := exec.Command("pkexec", append([]string{helper, "pacstrap"}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pacstrap failed: %w: %s", err, string(output))
	}
	return nil
}

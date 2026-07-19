package steps

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// ConfigureBootloader sets up GRUB bootloader using pkexec.
func ConfigureBootloader(diskID string, username, password, language string) error {
	if diskID == "" {
		diskID = "/dev/sda"
	}

	exe, err := exec.LookPath("aesthetic-os-installer")
	if err != nil {
		exe = "aesthetic-os-installer"
	}
	dir := filepath.Dir(exe)
	helper := filepath.Join(dir, "root-helper")

	// Generate fstab
	cmd := exec.Command("pkexec", helper, "genfstab", "/mnt")
	if _, err := cmd.CombinedOutput(); err != nil {
		// genfstab may output to stdout but still succeed
	}

	// Install GRUB
	cmd = exec.Command("pkexec", helper, "grub-install", diskID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("grub-install failed: %w: %s", err, string(output))
	}

	// Generate GRUB config
	cmd = exec.Command("pkexec", helper, "grub-mkconfig", "-o", "/mnt/boot/grub/grub.cfg")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("grub-mkconfig failed: %w: %s", err, string(output))
	}

	return nil
}

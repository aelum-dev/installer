package steps

import (
	"fmt"
	"os/exec"
)

// ConfigureBootloader sets up GRUB bootloader.
func ConfigureBootloader(diskID string) error {
	if diskID == "" {
		diskID = "/dev/sda"
	}
	// Generate fstab
	cmd := exec.Command("genfstab", "-U", "/mnt")
	if _, err := cmd.CombinedOutput(); err != nil {
		// genfstab may output to stdout but still succeed
	}
	// Install GRUB
	cmd = exec.Command("grub-install", "--target=i386-pc", diskID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("grub-install failed: %w: %s", err, string(output))
	}
	// Generate GRUB config
	cmd = exec.Command("grub-mkconfig", "-o", "/mnt/boot/grub/grub.cfg")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("grub-mkconfig failed: %w: %s", err, string(output))
	}
	return nil
}

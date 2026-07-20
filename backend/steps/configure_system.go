package steps

import (
	"fmt"
	"os/exec"
)

// ConfigureSystem sets up system configuration in the installed system via chroot.
// Configures: hostname, timezone, locale, keymap, and enables services.
func ConfigureSystem(ctx *InstallContext) error {
	if err := validateMountedSystem(ctx); err != nil {
		return err
	}

	if ctx.Username == "" {
		return fmt.Errorf("username not set")
	}

	helper := ctx.Helper

	// Step 1: Set hostname
	hostname := "aesthetic-os"
	cmd := exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf("echo '%s' > /mnt/etc/hostname", hostname))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set hostname: %w: %s", err, string(output))
	}

	// Step 2: Set timezone (UTC as default)
	cmd = exec.Command("sudo", helper, "run", "bash", "-c", "ln -sf /usr/share/zoneinfo/Etc/UTC /mnt/etc/localtime")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set timezone: %w: %s", err, string(output))
	}

	// Step 3: Configure locale
	// Uncomment en_US.UTF-8 in locale.gen
	cmd = exec.Command("sudo", helper, "run", "bash", "-c", "sed -i 's/^#en_US.UTF-8/en_US.UTF-8/' /mnt/etc/locale.gen")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to configure locale: %w: %s", err, string(output))
	}

	// Generate locale
	cmd = exec.Command("sudo", helper, "run", "chroot", "/mnt", "locale-gen")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to generate locale: %w: %s", err, string(output))
	}

	// Write locale.conf
	cmd = exec.Command("sudo", helper, "run", "bash", "-c", "echo 'LANG=en_US.UTF-8' > /mnt/etc/locale.conf")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to write locale.conf: %w: %s", err, string(output))
	}

	// Step 4: Set console keymap (us as default)
	cmd = exec.Command("sudo", helper, "run", "bash", "-c", "echo 'KEYMAP=us' > /mnt/etc/vconsole.conf")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set keymap: %w: %s", err, string(output))
	}

	return nil
}

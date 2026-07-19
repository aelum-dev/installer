package steps

import (
	"fmt"
	"os/exec"
)

// ConfigureSystem sets up system configuration in the installed system via chroot.
// Configures: hostname, timezone, locale, keymap, and enables services.
func ConfigureSystem(diskID, helper, username, password, language string) error {
	if username == "" {
		return fmt.Errorf("username not set")
	}

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
	cmd = exec.Command("sudo", helper, "run", "bash", "-c", "chroot /mnt locale-gen")
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

	// Step 5: Enable NetworkManager service
	cmd = exec.Command("sudo", helper, "run", "bash", "-c", "chroot /mnt systemctl enable NetworkManager")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to enable NetworkManager: %w: %s", err, string(output))
	}

	// Step 6: Enable sudo group wheel membership (already done by useradd -G in some variants)
	// Ensure user is in wheel group for sudo access
	cmd = exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf("chroot /mnt usermod -aG wheel %s", username))
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add user to wheel group: %w: %s", err, string(output))
	}

	// Step 7: Configure sudoers for passwordless sudo for wheel group
	cmd = exec.Command("sudo", helper, "run", "bash", "-c", "echo '%wheel ALL=(ALL:ALL) ALL' > /mnt/etc/sudoers.d/wheel")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to configure sudoers: %w: %s", err, string(output))
	}
	// Set proper permissions on sudoers file
	cmd = exec.Command("sudo", helper, "run", "bash", "-c", "chmod 0440 /mnt/etc/sudoers.d/wheel")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set sudoers permissions: %w: %s", err, string(output))
	}

	return nil
}

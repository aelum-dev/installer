package steps

import (
	"fmt"
	"os/exec"
)

// SetupUsers creates user accounts using sudo.
func SetupUsers(ctx *InstallContext) error {
	// Skip user setup if username is not provided (CLI mode)
	if ctx.Username == "" {
		return nil
	}

	helper := ctx.Helper

	// Create user inside chroot (skip if already exists)
	cmd := exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf(
		"chroot /mnt bash -c 'if ! id %s >/dev/null 2>&1; then useradd -m -G wheel %s; fi'",
		ctx.Username, ctx.Username,
	))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to setup user: %w: %s", err, string(output))
	}

	// Set password inside chroot
	if ctx.Password != "" {
		cmd = exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf(
			"chroot /mnt bash -c \"echo '%s:%s' | chpasswd\"",
			ctx.Username, ctx.Password,
		))
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("passwd failed: %w: %s", err, string(output))
		}
	}

	// Configure sudoers: wheel group requires password for sudo
	cmd = exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf(
		"chroot /mnt bash -c 'mkdir -p /etc/sudoers.d && echo \"%%wheel ALL=(ALL:ALL) ALL\" > /etc/sudoers.d/wheel && chmod 0440 /etc/sudoers.d/wheel'",
	))
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to configure sudoers: %w: %s", err, string(output))
	}

	return nil
}

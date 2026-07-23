package steps

import (
	"fmt"
	"os/exec"
	"strings"
)

// SetupUsers creates user accounts using sudo.
func SetupUsers(ctx *InstallContext) error {
	if err := validateMountedSystem(ctx); err != nil {
		return err
	}

	// Skip user setup if username is not provided (CLI mode)
	if ctx.Username == "" {
		return nil
	}

	helper := ctx.Helper

	if ctx.Logger != nil {
		ctx.Logger.Info("Creating user account '%s' with wheel group", ctx.Username)
	}

	// Create user inside chroot (skip if already exists)
	cmd := exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf(
		"chroot /mnt bash -c 'if ! id %s >/dev/null 2>&1; then useradd -m -G wheel %s; fi'",
		ctx.Username, ctx.Username,
	))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to setup user: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	// Set password inside chroot
	if ctx.Password != "" {
		if ctx.Logger != nil {
			ctx.Logger.Info("Setting password for user '%s'", ctx.Username)
		}
		cmd = exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf(
			"chroot /mnt bash -c \"echo '%s:%s' | chpasswd\"",
			ctx.Username, ctx.Password,
		))
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("passwd failed: %w: %s", err, string(output))
		}

		// Also set root password (same as user password for recovery)
		// Generate hash on host using openssl, then update /mnt/etc/shadow directly
		if ctx.Logger != nil {
			ctx.Logger.Info("Setting root password")
		}
		// Generate SHA-512 hash of password
		hashCmd := exec.Command("openssl", "passwd", "-6", ctx.Password)
		hashOutput, err := hashCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to generate password hash: %w", err)
		}
		hash := strings.TrimSpace(string(hashOutput))

		// Update root entry in /mnt/etc/shadow (use | as delimiter since hash contains /)
		cmd = exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf(
			"sed -i 's|^root:.*|root:%s|' /mnt/etc/shadow",
			hash,
		))
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to set root password: %w: %s", err, string(output))
		}
	}

	// Configure sudoers: wheel group requires password for sudo
	if ctx.Logger != nil {
		ctx.Logger.Info("Configuring sudoers for wheel group")
	}
	cmd = exec.Command("sudo", helper, HelperCmdRun, "bash", "-c", fmt.Sprintf(
		"chroot %s bash -c 'mkdir -p /etc/sudoers.d && echo \"%%wheel ALL=(ALL:ALL) ALL\" > /etc/sudoers.d/wheel && chmod %o /etc/sudoers.d/wheel'",
		MountRoot, SudoersFileMode,
	))
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to configure sudoers: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	return nil
}

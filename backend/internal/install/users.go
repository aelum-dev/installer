package install

import (
	"fmt"
	"os/exec"
	"strings"

	"aestheticos-installer/backend/internal/shared"
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

	var cmd *exec.Cmd
	var output []byte
	var err error


	// Copy ISO files (skel, dconf, wallpaper, environment, hypr.lua)
	if err := copyISOFiles(ctx); err != nil {
		return err
	}

	// Compile dconf database in installed system
	cmd = ctx.run(HelperCmdChrootRun, shared.MountRoot, "dconf", "update")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to compile dconf: %w: %s", err, string(output))
	}

	// Create user inside chroot (skip if already exists)
	cmd = ctx.run(HelperCmdChrootRun, shared.MountRoot, "useradd", "-m", "-G", "wheel", ctx.Username)
	output, err = cmd.CombinedOutput()
	if err != nil {
		// useradd fails if user already exists — that's ok
		if !strings.Contains(string(output), "already exists") {
			return fmt.Errorf("failed to setup user: %w: %s", err, string(output))
		}
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	// Set password inside chroot (via stdin to avoid shell interpolation of special chars)
	if len(ctx.Password) > 0 {
		cmd = ctx.run(HelperCmdRunChpasswd, ctx.Username)
		cmd.Stdin = strings.NewReader(string(ctx.Password) + "\n")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("passwd failed: %w: %s", err, string(output))
		}
		// Zero the password from the context after use.
		for k := range ctx.Password {
			ctx.Password[k] = 0
		}
	}

	// Configure sudoers: wheel group requires password for sudo
	// Write directly to /mnt/etc/sudoers.d/wheel (the mounted installed system),
	// NOT /etc/sudoers.d/wheel which is on the live ISO.
	cmd = ctx.run(HelperCmdRun, "mkdir", "-p", shared.MountRoot+"/etc/sudoers.d")
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create sudoers.d dir: %w", err)
	}
	if err := writeFile(ctx, "/etc/sudoers.d/wheel", "%wheel ALL=(ALL:ALL) ALL\n"); err != nil {
		return err
	}
	cmd = ctx.run(HelperCmdRun, "chmod", "0440", shared.MountRoot+"/etc/sudoers.d/wheel")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to chmod sudoers: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	return nil
}

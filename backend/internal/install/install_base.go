package install

import (
	"fmt"
	"os/exec"
)

// basePackages are the essential system packages installed during the base
// system step. Kept minimal so this step finishes quickly and the progress
// bar moves smoothly.
var basePackages = []string{
	// Base system
	"base",
	"linux-lts",
	"linux-firmware",
	"mkinitcpio",
	"arch-install-scripts",
	"mkinitcpio-archiso",
	"qemu-guest-agent",
	"edk2-shell",
	"memtest86+-efi",

	// Utilities
	"util-linux",
	"gptfdisk",
	"parted",
	"dosfstools",
	"e2fsprogs",
	"btrfs-progs",
	"pv",

	// System
	"sudo",
	"networkmanager",

	// Editor
	"vim",
	"nano",
}

// Install installs the Arch Linux base system via pacstrap.
func Install(ctx *InstallContext) error {
	return installPackages(ctx, basePackages, "base system")
}

// installPackages runs pacstrap with the given package list.
func installPackages(ctx *InstallContext, packages []string, description string) error {
	if err := validateMountedSystem(ctx); err != nil {
		return err
	}

	if len(packages) == 0 {
		return fmt.Errorf("no packages to install")
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Installing %d %s packages via pacstrap", len(packages), description)
	}

	args := []string{PacstrapMountPoint}
	args = append(args, packages...)
	cmd := exec.Command("sudo", append([]string{ctx.Helper, HelperCmdRun, "pacstrap"}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pacstrap failed: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}
	return nil
}

package steps

import (
	"fmt"
	"os/exec"
)

// RegenerateInitramfs loads the vfat kernel module and regenerates the initramfs.
// This ensures the vfat driver is included in the initramfs for mounting /boot at boot time.
//
// The autodetect mkinitcpio hook only includes modules that are currently loaded.
// Since vfat is not persistently loaded on the live ISO, it gets excluded from the
// initramfs, causing "Unknown filesystem vfat" errors at boot.
//
// This step must run after pacstrap (which triggers an initial mkinitcpio run) and
// before the system is rebooted.
func RegenerateInitramfs(ctx *InstallContext) error {
	if err := validateMountedSystem(ctx); err != nil {
		return err
	}

	helper := ctx.Helper

	if ctx.Logger != nil {
		ctx.Logger.Info("Loading vfat module and regenerating initramfs")
	}

	// Load vfat module inside the installed system
	// This ensures autodetect (or explicit inclusion) picks it up
	cmd := exec.Command("sudo", helper, "run", "chroot", "/mnt", "modprobe", "vfat")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Log warning but continue — modprobe might succeed silently or module might already be loaded
		if ctx.Logger != nil {
			ctx.Logger.Error("Warning: failed to load vfat module: %v", err)
		}
	} else if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	// Regenerate initramfs with vfat included
	// Using the linux-lts preset (matches the kernel package installed)
	cmd = exec.Command("sudo", helper, "run", "chroot", "/mnt", "mkinitcpio", "-p", "linux-lts")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to regenerate initramfs: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Initramfs regenerated successfully with vfat support")
	}

	return nil
}

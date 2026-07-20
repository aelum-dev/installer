package steps

import (
	"fmt"
	"os/exec"
)

// GenerateFstab generates the fstab file for the installed system using sudo.
// This must be done after pacstrap and before chroot operations.
func GenerateFstab(ctx *InstallContext) error {
	if err := validateMountedSystem(ctx); err != nil {
		return err
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Generating fstab for /mnt")
	}

	// Run genfstab -U /mnt and redirect output to /mnt/etc/fstab
	cmd := exec.Command("sudo", ctx.Helper, "run", "bash", "-c", "genfstab -U /mnt > /mnt/etc/fstab")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("genfstab failed: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	return nil
}

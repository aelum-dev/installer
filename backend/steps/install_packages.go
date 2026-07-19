package steps

import (
	"fmt"
	"os/exec"
)

// InstallPackages installs additional packages via arch-chroot using sudo.
// This runs AFTER pacstrap, so packages are installed into the already-mounted system.
func InstallPackages(diskID, helper, username, password, language string) error {
	// Additional packages to install via arch-chroot (after base system is set up)
	packages := []string{
		"htop", "neofetch", "git", "fish",
	}

	if len(packages) == 0 {
		return nil
	}

	args := []string{"arch-chroot", "/mnt", "pacman", "-S", "--noconfirm"}
	args = append(args, packages...)
	cmd := exec.Command("sudo", append([]string{helper, "run"}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("arch-chroot pacman failed: %w: %s", err, string(output))
	}
	return nil
}

package steps

import (
	"fmt"
	"os/exec"
)

// InstallBase installs the Arch Linux base system + additional packages using sudo.
// pacstrap runs only once with all packages.
func InstallBase(diskID, helper, username, password, language string) error {
	// All packages to install via pacstrap (runs only once)
	packages := []string{
		"base", "linux", "linux-firmware", "base-devel",
		"networkmanager", "sudo", "vim", "nano", "wget", "curl",
	}

	args := []string{"/mnt"}
	args = append(args, packages...)
	cmd := exec.Command("sudo", append([]string{helper, "pacstrap"}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pacstrap failed: %w: %s", err, string(output))
	}
	return nil
}

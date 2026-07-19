package steps

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// SetupUsers creates user accounts using pkexec.
func SetupUsers(diskID string, username, password, language string) error {
	if username == "" {
		return fmt.Errorf("username not set")
	}

	exe, err := exec.LookPath("aesthetic-os-installer")
	if err != nil {
		exe = "aesthetic-os-installer"
	}
	dir := filepath.Dir(exe)
	helper := filepath.Join(dir, "root-helper")

	// Create user
	cmd := exec.Command("pkexec", helper, "useradd", username)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("useradd failed: %w: %s", err, string(output))
	}

	// Set password (using chroot to set password in the installed system)
	if password != "" {
		cmd = exec.Command("pkexec", helper, "run", "bash", "-c", fmt.Sprintf("echo '%s:%s' | chroot /mnt chpasswd", username, password))
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("passwd failed: %w: %s", err, string(output))
		}
	}

	return nil
}

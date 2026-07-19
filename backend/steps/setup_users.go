package steps

import (
	"fmt"
	"os/exec"
)

// SetupUsers creates user accounts using sudo.
func SetupUsers(diskID, helper, username, password, language string) error {
	if username == "" {
		return fmt.Errorf("username not set")
	}

	// Create user
	cmd := exec.Command("sudo", helper, "useradd", username)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("useradd failed: %w: %s", err, string(output))
	}

	// Set password (using chroot to set password in the installed system)
	if password != "" {
		cmd = exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf("echo '%s:%s' | chroot /mnt chpasswd", username, password))
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("passwd failed: %w: %s", err, string(output))
		}
	}

	return nil
}

package install

import (
	"fmt"
	"os/exec"
	"strings"

	"aestheticos-installer/backend/internal/shared"
)

// runChroot executes a command inside the installed system via arch-chroot.
// Wraps: sudo <helper> run arch-chroot /mnt <args...>
func runChroot(ctx *InstallContext, args ...string) ([]byte, error) {
	cmdArgs := append([]string{ctx.Helper, HelperCmdRun, ChrootCommand, shared.MountRoot}, args...)
	cmd := exec.Command("sudo", cmdArgs...)
	return cmd.CombinedOutput()
}

// writeFile writes content to a file inside the installed system using tee.
// Avoids bash heredocs by using tee with stdin redirection.
// targetPath is relative to the system root (e.g., "/etc/hostname").
func writeFile(ctx *InstallContext, targetPath string, content string) error {
	cmd := exec.Command("sudo", ctx.Helper, HelperCmdRun, ChrootCommand, shared.MountRoot,
		"tee", targetPath)
	cmd.Stdin = strings.NewReader(content)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to write %s: %w: %s", targetPath, err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}
	return nil
}

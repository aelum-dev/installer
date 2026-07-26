package install

import (
	"fmt"
	"os/exec"
	"path/filepath"
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
// targetPath should be absolute path inside the chroot (e.g., "/etc/fstab").
// Creates parent directories if they don't exist.
func writeFile(ctx *InstallContext, targetPath string, content string) error {
	// Ensure parent directory exists (strip MountRoot since we're inside chroot)
	dir := filepath.Dir(targetPath)
	if dir != "/" && dir != "." {
		mkdirCmd := exec.Command("sudo", ctx.Helper, HelperCmdRun, ChrootCommand, shared.MountRoot,
			"mkdir", "-p", dir)
		if _, err := mkdirCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Writing %d bytes to %s", len(content), targetPath)
	}

	// targetPath is relative to chroot root, so pass it as-is (arch-chroot handles the root)
	cmd := exec.Command("sudo", ctx.Helper, HelperCmdRun, ChrootCommand, shared.MountRoot,
		"tee", targetPath)
	cmd.Stdin = strings.NewReader(content)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Log command output for debugging, but don't embed in error to avoid duplication
		if ctx.Logger != nil && len(output) > 0 {
			ctx.Logger.Command(string(output))
		}
		return fmt.Errorf("failed to write %s: %w", targetPath, err)
	}
	return nil
}

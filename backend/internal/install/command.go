package install

import (
	"fmt"
	"os/exec"
	"strings"

	"aestheticos-installer/backend/internal/shared"
)

// runChroot executes a command inside the installed system via arch-chroot.
// Wraps: sudo <helper> chroot-run /mnt <args...>
func runChroot(ctx *InstallContext, args ...string) ([]byte, error) {
	cmdArgs := append([]string{ctx.Helper, HelperCmdChrootRun, shared.MountRoot}, args...)
	cmd := exec.Command("sudo", cmdArgs...)
	return cmd.CombinedOutput()
}

// writeFile writes content to a file inside the installed system.
// targetPath should be absolute path inside the chroot (e.g., "/etc/fstab").
// If the path already starts with MountRoot (e.g., ESP paths), it is used as-is.
func writeFile(ctx *InstallContext, targetPath string, content string) error {
	if ctx != nil && ctx.Logger != nil {
	}

	// Prepend mount root for chroot-relative paths, but leave host paths (like ESP) as-is
	hostPath := targetPath
	if !strings.HasPrefix(targetPath, shared.MountRoot) {
		hostPath = shared.MountRoot + targetPath
	}
	cmd := exec.Command("sudo", ctx.Helper, HelperCmdWriteFile, hostPath)
	cmd.Stdin = strings.NewReader(content)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx != nil && ctx.Logger != nil && len(output) > 0 {
			ctx.Logger.Command(string(output))
		}
		return fmt.Errorf("failed to write %s: %w", targetPath, err)
	}
	return nil
}

// appendFile appends content to a file inside the installed system.
// targetPath should be absolute path inside the chroot (e.g., "/etc/vconsole.conf").
// If the path already starts with MountRoot, it is used as-is.
func appendFile(ctx *InstallContext, targetPath string, content string) error {
	if ctx != nil && ctx.Logger != nil {
	}

	// Prepend mount root for chroot-relative paths, but leave host paths as-is
	hostPath := targetPath
	if !strings.HasPrefix(targetPath, shared.MountRoot) {
		hostPath = shared.MountRoot + targetPath
	}
	cmd := exec.Command("sudo", ctx.Helper, HelperCmdAppendFile, hostPath)
	cmd.Stdin = strings.NewReader(content)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx != nil && ctx.Logger != nil && len(output) > 0 {
			ctx.Logger.Command(string(output))
		}
		return fmt.Errorf("failed to append to %s: %w", targetPath, err)
	}
	return nil
}

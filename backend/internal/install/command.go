package install

import (
	"fmt"
	"os/exec"
	"strings"

	"aestheticos-installer/backend/internal/shared"
)

// run executes a privileged command via the helper binary.
// Wraps: sudo <helper> <args...>
func (ctx *InstallContext) run(args ...string) *exec.Cmd {
	return exec.Command("sudo", append([]string{ctx.Helper}, args...)...)
}

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
	// Prepend mount root for chroot-relative paths, but leave host paths (like ESP) as-is
	hostPath := targetPath
	if !strings.HasPrefix(targetPath, shared.MountRoot) {
		hostPath = shared.MountRoot + targetPath
	}
	cmd := ctx.run(HelperCmdWriteFile, hostPath)
	cmd.Stdin = strings.NewReader(content)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", targetPath, err)
	}
	return nil
}

// appendFile appends content to a file inside the installed system.
// targetPath should be absolute path inside the chroot (e.g., "/etc/vconsole.conf").
// If the path already starts with MountRoot, it is used as-is.
func appendFile(ctx *InstallContext, targetPath string, content string) error {
	// Prepend mount root for chroot-relative paths, but leave host paths as-is
	hostPath := targetPath
	if !strings.HasPrefix(targetPath, shared.MountRoot) {
		hostPath = shared.MountRoot + targetPath
	}
	cmd := ctx.run(HelperCmdAppendFile, hostPath)
	cmd.Stdin = strings.NewReader(content)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to append to %s: %w", targetPath, err)
	}
	return nil
}

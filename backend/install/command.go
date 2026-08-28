package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// Command execution primitives
// ============================================================================

// Run executes a command directly (no helper needed when running as root).
func Run(args ...string) *exec.Cmd {
	return exec.Command(args[0], args[1:]...)
}

// writeFile writes content to a file inside the installed system.
// targetPath should be absolute path inside the chroot (e.g., "/etc/fstab").
// If the path already starts with MountRoot (e.g., ESP paths), it is used as-is.
func writeFile(targetPath string, content string) error {
	// Prepend mount root for chroot-relative paths, but leave host paths (like ESP) as-is
	hostPath := targetPath
	if !strings.HasPrefix(targetPath, shared.MountRoot) {
		hostPath = shared.MountRoot + targetPath
	}
	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(hostPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(hostPath), err)
	}
	if err := os.WriteFile(hostPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", targetPath, err)
	}
	return nil
}

// appendFile appends content to a file inside the installed system.
// targetPath should be absolute path inside the chroot (e.g., "/etc/vconsole.conf").
// If the path already starts with MountRoot, it is used as-is.
func appendFile(targetPath string, content string) error {
	// Prepend mount root for chroot-relative paths, but leave host paths as-is
	hostPath := targetPath
	if !strings.HasPrefix(targetPath, shared.MountRoot) {
		hostPath = shared.MountRoot + targetPath
	}
	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(hostPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(hostPath), err)
	}
	f, err := os.OpenFile(hostPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open %s for appending: %w", targetPath, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("failed to append to %s: %w", targetPath, err)
	}
	return nil
}

// ============================================================================
// Executor interface and implementation
// ============================================================================

// Executor abstracts command execution for installation steps.
// Implementations can be host-based (real) or test-based (mocked).
type Executor interface {
	// Run executes a command and returns the prepared Cmd (caller must Run it).
	Run(args ...string) *exec.Cmd
	// RunChroot executes a command inside the installed system via arch-chroot.
	RunChroot(args ...string) ([]byte, error)
	// WriteFile writes content to a file inside the installed system.
	WriteFile(path, content string) error
	// AppendFile appends content to a file inside the installed system.
	AppendFile(path, content string) error
}

// HostExecutor is the default implementation that runs commands on the host.
type HostExecutor struct{}

func (e *HostExecutor) Run(args ...string) *exec.Cmd {
	return exec.Command(args[0], args[1:]...)
}

func (e *HostExecutor) RunChroot(args ...string) ([]byte, error) {
	return primitives.RunChroot(args...)
}

func (e *HostExecutor) WriteFile(path, content string) error {
	return writeFile(path, content)
}

func (e *HostExecutor) AppendFile(path, content string) error {
	return appendFile(path, content)
}

// NewHostExecutor returns the default HostExecutor.
func NewHostExecutor() *HostExecutor {
	return &HostExecutor{}
}

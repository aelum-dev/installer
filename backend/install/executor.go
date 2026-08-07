package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"aestheticos-installer/backend/shared"
)

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
	cmdArgs := append([]string{shared.MountRoot}, args...)
	cmd := exec.Command("arch-chroot", cmdArgs...)
	return cmd.CombinedOutput()
}

func (e *HostExecutor) WriteFile(path, content string) error {
	hostPath := path
	if !strings.HasPrefix(path, shared.MountRoot) {
		hostPath = shared.MountRoot + path
	}
	if err := os.MkdirAll(filepath.Dir(hostPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(hostPath), err)
	}
	if err := os.WriteFile(hostPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

func (e *HostExecutor) AppendFile(path, content string) error {
	hostPath := path
	if !strings.HasPrefix(path, shared.MountRoot) {
		hostPath = shared.MountRoot + path
	}
	if err := os.MkdirAll(filepath.Dir(hostPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(hostPath), err)
	}
	f, err := os.OpenFile(hostPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open %s for appending: %w", path, err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("failed to append to %s: %w", path, err)
	}
	return nil
}

// NewHostExecutor returns the default HostExecutor.
func NewHostExecutor() *HostExecutor {
	return &HostExecutor{}
}



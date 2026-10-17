package primitives

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"aelum-installer/backend/shared"
)

// ============================================================================
// Command execution primitives
// ============================================================================

// Command is a handle to an external process. It mirrors the subset of
// *os/exec.Cmd that the install steps use — the Args slice, a Stdin reader, and
// the Run/Output/CombinedOutput terminal methods — so those steps can be unit
// tested with an injected CommandRunner instead of shelling out for real.
//
// Args[0] is the program name and Args[1:] its arguments, exactly like
// exec.Cmd, so callers that build argv incrementally keep working.
type Command struct {
	// Args holds the program name followed by its arguments, mirroring
	// exec.Cmd.Args (Args[0] is the program name).
	Args []string
	// Stdin, when non-nil, is connected to the command's standard input.
	Stdin io.Reader
	// runner executes this command. It is captured when the Command is built
	// so a test that swaps the runner sees the swap.
	runner CommandRunner
}

// Run starts the command and waits for it to complete.
func (c *Command) Run() error { return c.runner.Run(c) }

// Output runs the command and returns its standard output.
func (c *Command) Output() ([]byte, error) { return c.runner.Output(c) }

// CombinedOutput runs the command and returns its combined stdout and stderr.
func (c *Command) CombinedOutput() ([]byte, error) { return c.runner.CombinedOutput(c) }

// CommandRunner executes the external commands used by the install steps. The
// default implementation shells out via os/exec; tests inject a fake (see
// SetCommandRunner) to assert argv and simulate output or failure without root.
type CommandRunner interface {
	Run(cmd *Command) error
	Output(cmd *Command) ([]byte, error)
	CombinedOutput(cmd *Command) ([]byte, error)
}

// defaultRunner is the process-wide CommandRunner. Only tests swap it.
var defaultRunner CommandRunner = execRunner{}

// SetCommandRunner replaces the runner used by Run and RunChroot and returns a
// function that restores the previous one. It is intended for tests:
//
//	defer primitives.SetCommandRunner(fake)()
func SetCommandRunner(r CommandRunner) (restore func()) {
	prev := defaultRunner
	defaultRunner = r
	return func() { defaultRunner = prev }
}

// execRunner runs commands for real via os/exec.
type execRunner struct{}

func (execRunner) build(cmd *Command) *exec.Cmd {
	c := exec.Command(cmd.Args[0], cmd.Args[1:]...)
	c.Stdin = cmd.Stdin
	return c
}

func (r execRunner) Run(cmd *Command) error { return r.build(cmd).Run() }

func (r execRunner) Output(cmd *Command) ([]byte, error) { return r.build(cmd).Output() }

func (r execRunner) CombinedOutput(cmd *Command) ([]byte, error) {
	return r.build(cmd).CombinedOutput()
}

// Run builds a command to execute directly (no helper needed when running as
// root). The command is not started until one of its terminal methods is called.
func Run(args ...string) *Command {
	return &Command{Args: append([]string(nil), args...), runner: defaultRunner}
}

// RunChroot executes a command inside the installed system via arch-chroot.
func RunChroot(args ...string) ([]byte, error) {
	chrootArgs := append([]string{"arch-chroot", shared.MountRoot}, args...)
	return Run(chrootArgs...).CombinedOutput()
}

// ============================================================================
// File system primitives
// ============================================================================

// FileSystem abstracts the file operations used by the install steps. The
// default implementation talks to the real filesystem; tests inject an
// in-memory implementation (see primitivestest.MemFS) via SetFileSystem to
// exercise file-driven logic without writing under /mnt.
//
// All paths are host paths. Chroot-relative paths are prefixed with MountRoot by
// WriteFile/AppendFile before they reach a FileSystem, so implementations never
// need to know about the install root.
type FileSystem interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	// AppendFile appends data, creating the file if needed with the given perm.
	AppendFile(path string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Stat(path string) (os.FileInfo, error)
	RemoveAll(path string) error
	// Chmod changes path's permission bits.
	Chmod(path string, mode os.FileMode) error
}

// defaultFS is the process-wide FileSystem. Only tests swap it.
var defaultFS FileSystem = osFS{}

// SetFileSystem replaces the filesystem used by the primitives and returns a
// function that restores the previous one. It is intended for tests:
//
//	defer primitives.SetFileSystem(fake)()
func SetFileSystem(fs FileSystem) (restore func()) {
	prev := defaultFS
	defaultFS = fs
	return func() { defaultFS = prev }
}

// osFS is the real-filesystem FileSystem, backed by the os package.
type osFS struct{}

func (osFS) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func (osFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func (osFS) AppendFile(path string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = f.Write(data)
	return err
}

func (osFS) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func (osFS) Stat(path string) (os.FileInfo, error)        { return os.Stat(path) }
func (osFS) RemoveAll(path string) error                  { return os.RemoveAll(path) }
func (osFS) Chmod(path string, mode os.FileMode) error    { return os.Chmod(path, mode) }

// ReadFile reads a host path through the active FileSystem seam.
func ReadFile(path string) ([]byte, error) { return defaultFS.ReadFile(path) }

// Stat reports file information through the active FileSystem seam.
func Stat(path string) (os.FileInfo, error) { return defaultFS.Stat(path) }

// RemoveAll removes a host path and its contents through the active FileSystem
// seam.
func RemoveAll(path string) error { return defaultFS.RemoveAll(path) }

// Chmod changes a host path's permission bits through the active FileSystem
// seam.
func Chmod(path string, mode os.FileMode) error { return defaultFS.Chmod(path, mode) }

// WriteFileMode writes content to an absolute host path with the given
// permissions through the active FileSystem seam. Unlike WriteFile it applies no
// MountRoot prefixing, so callers must pass a host path.
func WriteFileMode(path string, content string, perm os.FileMode) error {
	return defaultFS.WriteFile(path, []byte(content), perm)
}

// WriteFile writes content to a file inside the installed system.
// targetPath should be absolute path inside the chroot (e.g., "/etc/fstab").
// If the path already starts with MountRoot (e.g., ESP paths), it is used as-is.
func WriteFile(targetPath string, content string) error {
	// Prepend mount root for chroot-relative paths, but leave host paths (like ESP) as-is
	hostPath := targetPath
	if !strings.HasPrefix(targetPath, shared.MountRoot) {
		hostPath = shared.MountRoot + targetPath
	}
	// Create parent directories
	if err := defaultFS.MkdirAll(filepath.Dir(hostPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(hostPath), err)
	}
	if err := defaultFS.WriteFile(hostPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", targetPath, err)
	}
	return nil
}

// AppendFile appends content to a file inside the installed system.
// targetPath should be absolute path inside the chroot (e.g., "/etc/vconsole.conf").
// MountRoot is the installed-system mount point, so chroot-relative paths get
// it prepended. Paths already under MountRoot (e.g. ESP files at /mnt/boot/...)
// are used as-is.
func AppendFile(targetPath string, content string) error {
	// Prepend mount root for chroot-relative paths, but leave paths already
	// under MountRoot (e.g. ESP files at /mnt/boot/...) as-is.
	hostPath := targetPath
	if !strings.HasPrefix(targetPath, shared.MountRoot) {
		hostPath = shared.MountRoot + targetPath
	}
	// Create parent directories if needed
	if err := defaultFS.MkdirAll(filepath.Dir(hostPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(hostPath), err)
	}
	if err := defaultFS.AppendFile(hostPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to append to %s: %w", targetPath, err)
	}
	return nil
}

// ============================================================================
// Validation helpers
// ============================================================================

// ValidateMountedSystem checks that /mnt is properly mounted with a root partition.
// This is a prerequisite for steps that operate on the installed system.
func ValidateMountedSystem() error {
	// Check if /mnt is mounted
	cmd := Run("findmnt", "-n", "-o", "SOURCE", shared.MountRoot)
	output, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(output)) == "" {
		return fmt.Errorf("/mnt is not mounted. Run --mount-filesystem <disk> first to mount partitions")
	}

	return nil
}

// ============================================================================
// Cleanup
// ============================================================================

// BuildMountOpts constructs mount options string from a BtrfsSubvolume's properties.
// Returns "subvol=<name>,compress=<val>,nodatacow" or similar based on properties.
func BuildMountOpts(sv *shared.BtrfsSubvolume) string {
	opts := shared.BtrfsSubvolumeMountPrefix + sv.Name
	if sv.Compression != "" {
		opts += ",compress=" + sv.Compression
	}
	if sv.NoDataCow {
		opts += ",nodatacow"
	}
	return opts
}

// CleanupMounts forcefully unmounts everything under /mnt.
func CleanupMounts(logger shared.Logger) error {
	pids, err := findHoldingPIDs(shared.MountRoot)
	if err != nil {
		return fmt.Errorf("find holding PIDs: %w", err)
	}

	if len(pids) > 0 {
		if err := killAndVerifyPIDs(pids); err != nil {
			return fmt.Errorf("kill PIDs %s: %w", strings.Join(pids, ", "), err)
		}
	}

	if err := unmountRecursive(shared.MountRoot); err != nil {
		return fmt.Errorf("umount -R %s: %w", shared.MountRoot, err)
	}

	return nil
}

func findHoldingPIDs(mountPoint string) ([]string, error) {
	output, err := Run("fuser", "-m", "-M", mountPoint).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return nil, nil
			}
		}
		return nil, err
	}
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return nil, nil
	}
	return strings.Fields(trimmed), nil
}

func killAndVerifyPIDs(pids []string) error {
	for _, pid := range pids {
		if err := Run("kill", "-KILL", pid).Run(); err != nil {
			if strings.Contains(err.Error(), "No such process") {
				continue
			}
			return fmt.Errorf("kill -KILL %s: %w", pid, err)
		}
		for i := 0; i < 50; i++ {
			if err := Run("kill", "-0", pid).Run(); err != nil {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	return nil
}

func unmountRecursive(mountPoint string) error {
	output, err := Run("umount", "-R", mountPoint).CombinedOutput()
	if err != nil {
		errMsg := strings.ToLower(string(output))
		if strings.Contains(errMsg, "not mounted") || strings.Contains(errMsg, "no such device") {
			return nil
		}
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

package primitives

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	// Symlink creates newname as a symbolic link to oldname. oldname is written
	// verbatim: a relative target stays relative, which is what a link created
	// inside an unmounted install root needs.
	Symlink(oldname, newname string) error
	// ReadDirNames lists the immediate entries of a directory, sorted.
	ReadDirNames(path string) ([]string, error)
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
func (osFS) Symlink(oldname, newname string) error        { return os.Symlink(oldname, newname) }

func (osFS) ReadDirNames(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

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

// Symlink creates newname as a symbolic link to oldname through the active
// FileSystem seam.
func Symlink(oldname, newname string) error { return defaultFS.Symlink(oldname, newname) }

// ReadDirNames lists the immediate entries of a host directory through the
// active FileSystem seam. The result is sorted so callers that render it (for
// example refind.conf include lines) are deterministic.
func ReadDirNames(path string) ([]string, error) {
	names, err := defaultFS.ReadDirNames(path)
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

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
// For the subvolume that holds the system, subvol= names the initial snapshot
// inside it rather than the container: that is the subvolume being installed into
// and the one the installed system boots.
func BuildMountOpts(sv *shared.BtrfsSubvolume) string {
	return btrfsMountOptions(shared.LiveSubvolumeName(sv), sv)
}

// BuildFstabMountOpts builds the fstab options for one subvolume.
//
// The subvolume that holds the system gets no subvol= at all. Its mount follows
// the filesystem's default subvolume, and a rollback moves that pointer - which is
// how "boot the state I just chose" works without rewriting fstab or the boot
// entry, and why the entry stays byte-identical across a rollback.
func BuildFstabMountOpts(sv *shared.BtrfsSubvolume) string {
	if sv.SnapshotRoot {
		return btrfsMountOptions("", sv)
	}
	return btrfsMountOptions(sv.Name, sv)
}

// BuildSnapshotsMountOpts builds the mount and fstab options for the mount that
// keeps a subvolume's .snapshots reachable at a fixed path. It carries the
// properties of the subvolume it belongs to so the history is stored the same way
// as the system it records.
func BuildSnapshotsMountOpts(sv *shared.BtrfsSubvolume) string {
	return btrfsMountOptions(shared.SnapshotDirName(sv), sv)
}

func btrfsMountOptions(subvol string, sv *shared.BtrfsSubvolume) string {
	opts := make([]string, 0, 3)
	if subvol != "" {
		opts = append(opts, shared.BtrfsSubvolumeMountPrefix+subvol)
	}
	if sv.Compression != "" {
		opts = append(opts, "compress="+sv.Compression)
	}
	if sv.NoDataCow {
		opts = append(opts, "nodatacow")
	}
	return strings.Join(opts, ",")
}

// CleanupMounts forcefully unmounts everything under /mnt.
func CleanupMounts(logger shared.Logger) error {
	pids, err := findHoldingPIDs(shared.MountRoot)
	if err != nil {
		return fmt.Errorf("find holding PIDs: %w", err)
	}

	if len(pids) > 0 {
		if err := killAndVerifyPIDs(pids); err != nil {
			// A holder can exit between fuser and kill. Killing is
			// best-effort: failing to signal a dead PID must never prevent
			// the unmount below (that would leave /mnt mounted and make the
			// next install attempt fail with EBUSY).
			if logger != nil {
				logger.Error("Cleanup: could not kill all holders (continuing): %v", err)
			}
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
		out, err := Run("kill", "-KILL", pid).CombinedOutput()
		if err != nil {
			// The process may have exited between fuser(1) and kill(1). That is
			// success for cleanup purposes; only a real kill failure is reported.
			if isHarmlessKillError(out) {
				continue
			}
			return fmt.Errorf("kill -KILL %s: %w: %s", pid, err, strings.TrimSpace(string(out)))
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

// isHarmlessKillError reports whether a kill(1) failure is safe to ignore: the
// target PID is already gone. kill prints "No such process" to stderr; when a
// runner discards stderr (or a thin kill implementation returns a bare status),
// there is no diagnostic and the unmount that follows is the real test.
func isHarmlessKillError(output []byte) bool {
	msg := strings.ToLower(string(output))
	if strings.TrimSpace(msg) == "" {
		return true
	}
	return strings.Contains(msg, "no such process") || strings.Contains(msg, "no process")
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

package gui

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aelum-installer/backend/shared"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	EventInstallationProgress  = "installation:progress"
	EventInstallationComplete  = "installation:complete"
	EventInstallationError     = "installation:error"
	EventInstallationCancelled = "installation:cancelled"
)

// InstallationProgress is emitted as an event to the frontend.
type InstallationProgress struct {
	StepID  string `json:"stepId"` // stable step identifier, localized by the UI
	Label   string `json:"label"`
	Percent int    `json:"percent"`
	Phase   string `json:"phase"` // "running" | "done" | "error"
}

// InstallationComplete is the payload of the installation:complete event: how
// long the install took, measured from CLI start to a clean exit. The finish
// page shows it, so the number covers the whole install, not just the last step.
type InstallationComplete struct {
	ElapsedSeconds int `json:"elapsedSeconds"`
}

// cleanupWaitTimeout is how long WaitForCleanup waits for a cancelled install to
// release the disk. Tests shorten it; the shipped value is the measured cancel
// (about 600 ms) with room for a slow disk.
var cleanupWaitTimeout = time.Duration(shared.CleanupWaitTimeoutMs) * time.Millisecond

// findCLIBinary locates the aelum-installer-cli binary.
func findCLIBinary() string {
	if exe, err := os.Executable(); err == nil {
		if p := filepath.Join(filepath.Dir(exe), "aelum-installer-cli"); fileExists(p) {
			return p
		}
	}
	for _, dir := range []string{"/usr/local/bin", "/usr/bin", "."} {
		if p := filepath.Join(dir, "aelum-installer-cli"); fileExists(p) {
			return p
		}
	}
	return "aelum-installer-cli"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readCLILines hands every line of one of the CLI's output pipes to handle,
// until the pipe reaches EOF.
//
// It reads with a bufio.Reader rather than a bufio.Scanner for the same reason
// installbase reads pacstrap with one: a Scanner ends its loop at the first token
// longer than its 64 KiB buffer, and a reader that ends early is a pipe nobody
// drains any more. The CLI logs on both pipes at every step — human-readable
// lines on stdout, progress events on stderr — so a pipe that fills stops the CLI
// on its next log write and the install freezes mid-flight: bar stopped, no
// error, nothing on screen to explain it. What grows past a buffer is what the
// CLI echoes from a command it ran: a "Packages (N) …" line follows the size of
// the transaction, and an error can carry a tool's whole output. A bufio.Reader
// has no such cap: ReadString accumulates across buffer fills, so the size below
// is only how much is read at a time.
//
// A read error that is not EOF is reported rather than ending the loop quietly:
// the CLI is still running, so a reader that died is a stuck install waiting to
// happen, and the install log is the only place anyone will look for it.
func readCLILines(what string, r io.Reader, handle func(string)) {
	reader := bufio.NewReaderSize(r, 64*1024)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSuffix(line, "\r")
			handle(line)
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				shared.GetLogger().Info("Reading the CLI's %s stopped: %v", what, err)
			}
			return
		}
	}
}

// StartInstallation spawns the CLI binary via sudo. Returns an error if one is
// already running or if the CLI binary cannot be found.
func (a *App) StartInstallation() error {
	if a.installationRunning.Load() {
		return fmt.Errorf("an installation is already in progress")
	}

	// Encryption without a passphrase would make the CLI die before its first
	// step (it reads the passphrase from stdin), showing the user a bare failure
	// at 0%. The frontend sends the passphrase and the flag as two separate
	// Wails calls, so refuse to start rather than spawn a doomed process.
	if a.luks && !a.dryRun && len(a.luksPassphrase) == 0 {
		return fmt.Errorf("encryption is enabled but no passphrase was provided")
	}

	cliBinary := findCLIBinary()

	diskID := a.diskSelection.DiskID
	if diskID == "" && a.dryRun {
		diskID = "/dev/loop0"
	}
	// Without a disk the CLI prints usage and exits 1, surfacing as an opaque
	// "installation failed" at 0%. Refuse up front with the real cause.
	if diskID == "" {
		return fmt.Errorf("no target disk selected")
	}

	args := []string{
		"--install", diskID,
		"--username", a.userAccount.Username,
		"--filesystem", a.fileSystem,
		// Language and location travel as separate flags: the CLI writes the
		// system locale from one, and the clock, console keymap and mirrors from
		// the other. An empty region code means "the country of the language",
		// which is what the CLI falls back to.
		"--language", a.selectedLanguage,
		"--region", a.region.Code,
	}
	if a.luks {
		args = append(args, "--luks")
	}
	if a.lvm {
		args = append(args, "--lvm")
	}
	if a.dryRun {
		args = append(args, "--dry-run")
	}

	// Manual mode: pass the user-designed layout so the CLI only formats the
	// Boot/System partitions. It is serialized as JSON.
	if a.manualMode {
		layoutJSON, err := json.Marshal(a.manualLayout)
		if err != nil {
			return fmt.Errorf("failed to serialize manual layout: %w", err)
		}
		args = append(args, "--manual-layout", string(layoutJSON))
	}

	// The GUI computes the log path once and passes it to the CLI so both
	// processes report and write the same file (including the /shared mirror).
	logPath := shared.GetLogPath()
	args = append(args, "--log-path", logPath)

	ctx, cancel := context.WithCancel(context.Background())

	cmdArgs := append([]string{cliBinary}, args...)
	if !a.dryRun {
		cmdArgs = append([]string{"sudo"}, cmdArgs...)
	}
	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start CLI: %w", err)
	}

	// The install clock starts when the CLI is actually running, so the
	// duration covers the install itself, not argument parsing.
	a.installStartedAt = time.Now()

	a.installCmd = cmd
	a.installCancel = cancel
	a.installDone = make(chan struct{})
	a.installCancelled.Store(false)
	a.installationRunning.Store(true)

	// Write password to stdin, then (when encrypting) the LUKS passphrase. The
	// order matches the CLI's sequential stdin reads: username flag → account pw
	// → LUKS pw.
	go func() {
		if len(a.userAccount.Password) > 0 {
			if _, err := stdin.Write([]byte(a.userAccount.Password + "\n")); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to write password to CLI: %v\n", err)
			}
		}
		if a.luks && !a.dryRun && len(a.luksPassphrase) > 0 {
			if _, err := stdin.Write(append(a.luksPassphrase, '\n')); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to write LUKS passphrase to CLI: %v\n", err)
			}
			// Zero the passphrase once handed to the CLI.
			for k := range a.luksPassphrase {
				a.luksPassphrase[k] = 0
			}

			a.luksPassphrase = nil
		}
		if err := stdin.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close CLI stdin: %v\n", err)
		}
	}()

	// Both pipes are read to EOF, and the completion goroutine gives the readers
	// time to finish before it calls Wait: Wait closes the pipes it created, so a
	// Wait that wins the race discards whatever the CLI wrote last — the 100%
	// progress event included. os/exec calls it incorrect to wait before every read
	// from the pipe has completed.
	var readers sync.WaitGroup
	readers.Add(2)

	// Forward stdout to terminal.
	go func() {
		defer readers.Done()
		readCLILines("stdout", stdout, func(line string) {
			fmt.Fprintf(os.Stderr, "%s\n", line)
		})
	}()

	// Read stderr: emit JSON progress events, forward everything else to terminal.
	go func() {
		defer readers.Done()
		readCLILines("stderr", stderr, func(line string) {
			var p InstallationProgress
			if json.Unmarshal([]byte(line), &p) == nil {
				wailsRuntime.EventsEmit(a.ctx, EventInstallationProgress, p)
				return
			}
			fmt.Fprintf(os.Stderr, "%s\n", line)
		})
	}()

	// Wait for the CLI to finish, then emit completion/error events and
	// signal completion to WaitForCleanup callers.
	go func() {
		// Give the readers the time to reach EOF before reaping, but only so long:
		// a killed command can leave a grandchild holding the write end open, and a
		// writer that never closes it would otherwise hold up the event the person is
		// watching for. The bound is the GUI's existing measure for "the CLI's work
		// has stopped".
		drained := make(chan struct{})
		go func() {
			readers.Wait()
			close(drained)
		}()
		select {
		case <-drained:
		case <-time.After(cleanupWaitTimeout):
			shared.GetLogger().Info("The CLI's output had not stopped after %s — waiting for it no longer", cleanupWaitTimeout)
		}

		err := cmd.Wait()
		switch {
		case err == nil:
			wailsRuntime.EventsEmit(a.ctx, EventInstallationComplete, InstallationComplete{
				ElapsedSeconds: int(time.Since(a.installStartedAt).Seconds()),
			})
		case a.installCancelled.Load():
			// The person closed the window during the install: the CLI stopped on
			// request and exits with the signal code. That is not a failure of the
			// installer's, so it does not travel on the error event — it gets the
			// cancelled event, and the window that receives it stays open on the
			// cancelled page (see App.OnBeforeClose). The payload is empty on
			// purpose: the page names the disk from the answer the person gave.
			wailsRuntime.EventsEmit(a.ctx, EventInstallationCancelled)
		default:
			wailsRuntime.EventsEmit(a.ctx, EventInstallationError, "Installation failed",
				fmt.Sprintf("%s. For details, check: %s", err.Error(), logPath))
		}
		close(a.installDone)
		a.installCmd = nil
		a.installCancel = nil
		a.installationRunning.Store(false)
	}()

	return nil
}

// CancelInstallation sends SIGINT to the running CLI process and lets it exit
// gracefully. The CLI's own signal handler closes the cancel channel: the step
// loop stops before the next step, the command in flight is killed together with
// its whole process group, the defers run (CleanupMounts), and the CLI exits with
// the signal code. We do NOT cancel the exec.CommandContext here — doing so would
// SIGKILL the process before its defers get a chance to unmount /mnt.
//
// The two callers are closing the window mid-install and RebootAndClose; the
// wizard has no cancel button. The request is recorded first, so the completion
// goroutine can tell that stop from a CLI that died on its own, even when the CLI
// dies before it is signalled.
func (a *App) CancelInstallation() {
	if a.installCmd != nil && a.installCmd.Process != nil {
		a.installCancelled.Store(true)
		_ = a.installCmd.Process.Signal(os.Interrupt)
	}
}

// WaitForCleanup blocks until the current installation's cleanup has completed, or
// until waiting any longer would cost the person a window that does not answer.
// A cancelled CLI is killed as a process group and exits within a second, so a
// wait that runs past the timeout is a hung process — and the cancelled page is
// still the honest screen to be left on, not a dead callback.
func (a *App) WaitForCleanup() {
	if a.installDone == nil {
		return
	}
	select {
	case <-a.installDone:
	case <-time.After(cleanupWaitTimeout):
		shared.GetLogger().Info("Cleanup has not finished after %s — carrying on without it", cleanupWaitTimeout)
	}
}

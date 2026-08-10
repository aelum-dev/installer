package gui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	EventInstallationProgress = "installation:progress"
	EventInstallationComplete = "installation:complete"
	EventInstallationError    = "installation:error"
)

// InstallationProgress is emitted as an event to the frontend.
type InstallationProgress struct {
	Label   string `json:"label"`
	Percent int    `json:"percent"`
	Phase   string `json:"phase"` // "running" | "done" | "error"
}

// findCLIBinary locates the aestheticos-installer-cli binary.
func findCLIBinary() string {
	if exe, err := os.Executable(); err == nil {
		if p := filepath.Join(filepath.Dir(exe), "aestheticos-installer-cli"); fileExists(p) {
			return p
		}
	}
	for _, dir := range []string{"/usr/local/bin", "/usr/bin", "."} {
		if p := filepath.Join(dir, "aestheticos-installer-cli"); fileExists(p) {
			return p
		}
	}
	return "aestheticos-installer-cli"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// StartInstallation spawns the CLI binary via sudo. Returns an error if one is
// already running or if the CLI binary cannot be found.
func (a *App) StartInstallation() error {
	if a.installationRunning.Load() {
		return fmt.Errorf("an installation is already in progress")
	}

	if !a.dryRun && !a.CheckRoot().Authorized {
		wailsRuntime.LogError(a.ctx, "The installer needs sudo access. Please add your user to the sudoers file.")
		return fmt.Errorf("sudo access required")
	}

	cliBinary := findCLIBinary()

	diskID := a.diskSelection.DiskID
	if diskID == "" && a.dryRun {
		diskID = "/dev/loop0"
	}

	args := []string{"--install", diskID, "--username", a.userAccount.Username, "--filesystem", a.fileSystem}
	if a.dryRun {
		args = append(args, "--dry-run")
	}

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

	a.installCmd = cmd
	a.installCancel = cancel
	a.installDone = make(chan struct{})
	a.installationRunning.Store(true)

	// Write password to stdin.
	go func() {
		defer stdin.Close()
		if len(a.userAccount.Password) > 0 {
			stdin.Write([]byte(a.userAccount.Password + "\n"))
		}
	}()

	// Forward stdout to terminal.
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Fprintf(os.Stderr, "%s\n", scanner.Text())
		}
	}()

	// Read stderr: emit JSON progress events, forward everything else to terminal.
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			var p InstallationProgress
			if json.Unmarshal([]byte(line), &p) == nil {
				wailsRuntime.EventsEmit(a.ctx, EventInstallationProgress, p)
				continue
			}
			fmt.Fprintf(os.Stderr, "%s\n", line)
		}
	}()

	// Wait for the CLI to finish, then signal completion.
	go func() {
		cmd.Wait()
		close(a.installDone)
		a.installCmd = nil
		a.installCancel = nil
		a.installationRunning.Store(false)
	}()

	return nil
}

// CancelInstallation sends SIGINT to the running CLI process.
func (a *App) CancelInstallation() {
	if a.installCmd != nil && a.installCmd.Process != nil {
		_ = a.installCmd.Process.Signal(os.Interrupt)
	}
	if a.installCancel != nil {
		a.installCancel()
	}
}

// WaitForCleanup blocks until the current installation's cleanup has completed.
func (a *App) WaitForCleanup() {
	if a.installDone != nil {
		<-a.installDone
	}
}

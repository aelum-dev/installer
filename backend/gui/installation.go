package gui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

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

// cliProcess tracks the running CLI subprocess for cancellation.
type cliProcess struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	cancel func()
}

var activeCLI = &cliProcess{}

// findCLIBinary locates the aestheticos-installer-cli binary.
// Checks next to the GUI binary first, then PATH.
func findCLIBinary() string {
	// Check next to the GUI binary
	exe, err := os.Executable()
	if err == nil {
		cliPath := filepath.Join(filepath.Dir(exe), "aestheticos-installer-cli")
		if _, err := os.Stat(cliPath); err == nil {
			return cliPath
		}
	}

	// Check common install locations
	for _, dir := range []string{"/usr/local/bin", "/usr/bin", "."} {
		cliPath := filepath.Join(dir, "aestheticos-installer-cli")
		if _, err := os.Stat(cliPath); err == nil {
			return cliPath
		}
	}

	return "aestheticos-installer-cli" // fallback — should be in PATH
}



// StartInstallation begins the installation process by spawning the CLI binary via sudo.
func (a *App) StartInstallation() error {
	// Safety check: verify root privileges one more time before starting
	rootCheck := a.CheckRoot()
	if !rootCheck.Authorized {
		wailsRuntime.LogError(a.ctx, fmt.Sprintf("Root check failed: %s", rootCheck.Message))
		return fmt.Errorf("root check failed: %s", rootCheck.Message)
	}

	// Cancel any previous installation
	activeCLI.mu.Lock()
	if activeCLI.cmd != nil && activeCLI.cmd.Process != nil {
		activeCLI.cancel()
	}
	activeCLI.mu.Unlock()

	// Find the CLI binary
	cliBinary := findCLIBinary()
	if cliBinary == "" {
		return fmt.Errorf("aestheticos-installer-cli not found. Ensure it is installed next to the GUI binary or in PATH")
	}

	// Use provided hostname or fall back to default
	hostname := a.hostname
	if hostname == "" {
		hostname = "aestheticos"
	}

	// Build CLI arguments
	args := []string{
		"--install", a.diskSelection.DiskID,
		"--filesystem", a.fileSystem,
	}

	if a.dryRun {
		args = append(args, "--dry-run")
	}

	// Create sudo command to run the CLI
	cmd := exec.Command("sudo", append([]string{cliBinary}, args...)...)

	// Set up stdin for password
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Set up stdout reader for progress
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Set up stderr for error logging
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start CLI: %w", err)
	}

	// Store process reference for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	activeCLI.mu.Lock()
	activeCLI.cmd = cmd
	activeCLI.cancel = cancel
	activeCLI.mu.Unlock()

	// Mark installation as in-progress
	a.installationRunning.Store(true)

	// Write password to stdin in a goroutine
	go func() {
		defer stdin.Close()
		if len(a.userAccount.Password) > 0 {
			stdin.Write([]byte(a.userAccount.Password + "\n"))
		}
	}()

	// Read stdout and forward to terminal — clean human-readable output.
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Fprintf(os.Stderr, "%s\n", scanner.Text())
		}
	}()

	// Read stderr for JSON progress events and error logging.
	// The CLI emits JSON progress lines to stderr alongside error messages.
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			var progress InstallationProgress
			if err := json.Unmarshal([]byte(line), &progress); err == nil {
				wailsRuntime.EventsEmit(a.ctx, EventInstallationProgress, progress)
				continue
			}
			// Forward non-JSON lines (errors, password prompt) to terminal.
			fmt.Fprintf(os.Stderr, "%s\n", line)
			if strings.Contains(line, "ERROR") || strings.Contains(line, "Error:") {
				GetLogger().WriteErrorLog("CLI stderr", line)
			}
		}
	}()

	// Wait for completion in a goroutine
	go func() {
		defer func() {
			a.installationRunning.Store(false)
			activeCLI.mu.Lock()
			activeCLI.cmd = nil
			activeCLI.cancel = nil
			activeCLI.mu.Unlock()
		}()

		err := cmd.Wait()
		cancel()

		if err != nil {
			// Check if cancelled
			if ctx.Err() == context.Canceled {
				return
			}

			// Report error
			progress := InstallationProgress{
				Label:   "Installation failed",
				Percent: 0,
				Phase:   "error",
			}
			wailsRuntime.EventsEmit(a.ctx, EventInstallationError, progress, "Install failed. You can reboot or use --rollback flag on aestheticos-installer-cli. For detailed information check installation.log", err.Error())
			return
		}

		// Report success
		label := "Installation completed!"
		if a.dryRun {
			label = "[DRY RUN] Installation completed!"
		}
		wailsRuntime.EventsEmit(a.ctx, EventInstallationComplete, InstallationProgress{
			Label:   label,
			Percent: 100,
			Phase:   "done",
		})
	}()

	return nil
}

// CancelInstallation sends SIGINT to the running CLI process.
func (a *App) CancelInstallation() error {
	CancelActiveCLI()
	return nil
}

// CancelActiveCLI sends SIGINT to the currently running CLI process.
// Exported for use by main.go's signal handler.
func CancelActiveCLI() {
	activeCLI.mu.Lock()
	defer activeCLI.mu.Unlock()

	if activeCLI.cmd != nil && activeCLI.cmd.Process != nil {
		_ = activeCLI.cmd.Process.Signal(os.Interrupt)
	}
}

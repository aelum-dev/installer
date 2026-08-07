package install

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aestheticos-installer/backend/shared"
)

// InstallationProgress is emitted as an event to the frontend.
type InstallationProgress struct {
	Label   string `json:"label"`
	Percent int    `json:"percent"`
	Phase   string `json:"phase"` // "running" | "done" | "error"
}

// Installer orchestrates the full installation process.
// Single source of truth — used by both CLI and GUI.
type Installer struct {
	diskID     string
	username   string
	password   []byte // zeroed after being copied to InstallContext
	hostname   string
	region     shared.Region
	fileSystem string // root filesystem: "ext4" or "btrfs"
	dryRun     bool
	esp        string // ESP mount point (default: /mnt/boot)

	// Logger for step output (injected by CLI/GUI; default is ConsoleLogger)
	Logger Logger

	// Progress callback (nil for CLI, sets up UI events for GUI)
	OnProgress func(label string, percent int, phase string)
	// Error callback
	OnError func(label, message string)

	// interrupted is set to true when SIGINT is received. Used by the CLI to
	// decide the exit code (130 for SIGINT vs 1 for other errors).
	interrupted atomic.Bool

	// cleanupOnce ensures CleanupMounts runs at most once, regardless of which
	// path triggers it (defer, OnBeforeClose, SIGTERM handler, panic recovery).
	cleanupOnce sync.Once

	// cancel is closed when SIGINT is received during a long-running command
	// (e.g. pacstrap). This unblocks scanner goroutines so they return promptly.
	cancel chan struct{}
}

// isInterruptError reports whether err was caused by SIGINT (Ctrl+C).
// This is more reliable than checking the interrupted flag because the
// child process (e.g. pacstrap) may receive SIGINT and return before the
// Go signal handler goroutine has a chance to set the flag.
func isInterruptError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	// exec.Command returns "signal: interrupt" when the child is killed by SIGINT.
	if strings.Contains(errMsg, "signal: interrupt") {
		return true
	}
	// Also check for exit code 130 (standard POSIX SIGINT exit code).
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 130 {
		return true
	}
	return false
}

// NewInstaller creates a new Installer instance.
func NewInstaller(diskID, username, password, hostname string, region shared.Region, fileSystem string, dryRun bool) *Installer {
	return &Installer{
		diskID:     diskID,
		username:   username,
		password:   []byte(password),
		hostname:   hostname,
		region:     region,
		fileSystem: fileSystem,
		dryRun:     dryRun,
		esp:        shared.BootMountPoint,
		Logger:     NewConsoleLogger(),
	}
}

// Install runs the full installation process.
// Returns nil on success, error on failure.
// On failure, cleans up any mounted filesystems so the installer can be retried.
func (inst *Installer) Install() error {
	if inst.diskID == "" {
		return fmt.Errorf("disk ID is required")
	}
	if inst.region.Code == "" {
		return fmt.Errorf("region not set")
	}

	// Clear previous log
	logPath := shared.GetLogPath()
	_ = os.WriteFile(logPath, nil, shared.LogFileMode)

	// Ensure mounts are cleaned up on any failure so the installer is retryable.
	// Cleanup is guarded by sync.Once — only the first caller wins, regardless
	// of whether it's the defer, OnBeforeClose, a signal handler, or panic recovery.
	defer inst.RunCleanup()

	ctx := inst.context()

	plan := DefaultPlan()
	stepList := plan.Steps
	percentages := plan.cumulativePercentages()

	for idx, step := range stepList {
		label := step.Label()
		if inst.dryRun {
			label = "[DRY RUN] " + label
		}

		startPercent := percentages[idx]
		endPercent := percentages[idx+1]

		// Wire progress callback so the tick goroutine can update the UI.
		ctx.ProgressCallback = func(percent int) {
			// Convert step-relative percentage (0-100) to absolute percentage (startPercent-endPercent)
			absPercent := int(float64(startPercent) + (float64(percent) * float64(endPercent-startPercent) / 100.0))
			inst.emitProgress(label, absPercent, "running")
		}

		inst.emitProgress(label, startPercent, "running")
		inst.logInfo("Starting step: %s", step.Label())

		// Launch tick goroutine: slowly advances percentage during long steps.
		// Fast steps (format, mount) finish before the first tick fires.
		stopTick := make(chan struct{})
		go inst.tickProgress(label, startPercent, endPercent, stopTick)

		var err error
		if inst.dryRun {
			err = simulateStep()
		} else {
			err = step.Run(ctx)
		}
		close(stopTick) // stop the tick goroutine

		if err != nil {
			if !isInterruptError(err) {
				inst.logError("Step failed: %s - %v", step.Label(), err)
				inst.emitError(step.Label(), err.Error())
			}
			return fmt.Errorf("step \"%s\" failed: %w", step.Label(), err)
		}

		inst.logInfo("Step completed: %s", step.Label())
		inst.emitProgress(label, endPercent, "done")
	}

	inst.logInfo("Installation completed successfully")
	return nil
}

// tickProgress slowly advances the percentage from start to end, emitting
// progress events at regular intervals. Used to keep the UI feeling alive
// during long-running steps (e.g. pacstrap). Stops when stopTick is closed.
func (inst *Installer) tickProgress(label string, start, end int, stopTick chan struct{}) {
	if start >= end {
		return
	}

	// Initial delay — fast steps finish before the first tick fires.
	select {
	case <-time.After(time.Duration(shared.ProgressTickDelayMs) * time.Millisecond):
	case <-stopTick:
		return
	}

	current := start
	ticker := time.NewTicker(time.Duration(shared.ProgressTickIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			current++
			if current > end {
				current = end
			}
			inst.emitProgress(label, current, "running")
			if current >= end {
				return
			}
		case <-stopTick:
			return
		}
	}
}

// installationSteps returns the list of installation steps from the default plan.
func (inst *Installer) installationSteps() []Step {
	return DefaultPlan().Steps
}

// validStepNames returns the list of valid step names for error messages.
func validStepNames() []string {
	names := make([]string, len(DefaultPlan().Steps))
	for i, s := range DefaultPlan().Steps {
		names[i] = s.Name()
	}
	return names
}

// RunStep runs a single named installation step.
// Returns the step label, the percentage (0-100, always 100 for single step), and any error.
// On failure, cleans up any mounted filesystems so the step can be retried.
func (inst *Installer) RunStep(stepName string) (label string, percent int, err error) {
	// Ensure mounts are cleaned up on failure so the step is retryable.
	defer inst.RunCleanup()

	allSteps := inst.installationSteps()
	for _, s := range allSteps {
		if s.Name() == stepName {
			label = s.Label()
			if inst.dryRun {
				label = "[DRY RUN] " + label
			}
			inst.emitProgress(label, 0, "running")
			inst.logInfo("Starting step: %s", s.Label())
			var stepErr error
			if inst.dryRun {
				stepErr = simulateStep()
			} else {
				stepErr = s.Run(inst.context())
			}
			if stepErr != nil {
				inst.logError("Step failed: %s - %v", s.Label(), stepErr)
				inst.emitError(s.Label(), stepErr.Error())
				return label, 0, fmt.Errorf("step \"%s\" failed: %w", s.Label(), stepErr)
			}
			inst.emitProgress(label, 100, "done")
			inst.logInfo("Step completed: %s", s.Label())
			return label, 100, nil
		}
	}
	valid := validStepNames()
	return "", 0, fmt.Errorf("unknown step %q, valid steps: %v", stepName, valid)
}

// context creates an InstallContext for this installer.
// The password is copied to the context and then zeroed from the Installer
// to minimize the time the plaintext password resides in memory.
func (inst *Installer) context() *InstallContext {
	password := make([]byte, len(inst.password))
	copy(password, inst.password)
	// Zero our copy immediately — the context now owns the only copy.
	for k := range inst.password {
		inst.password[k] = 0
	}
	// Create a fresh cancel channel for this installation run.
	inst.cancel = make(chan struct{})
	return &InstallContext{
		DiskID:     inst.diskID,
		ESP:        inst.esp,
		Username:   inst.username,
		Password:   password,
		Hostname:   inst.hostname,
		Region:     inst.region,
		FileSystem: inst.fileSystem,
		Logger:     inst.Logger,
		Cancel:     inst.cancel,
	}
}

// Cancel closes the install cancellation channel, unblocking any scanner
// goroutines (e.g. pacstrap output reader) that are waiting on input.
// Called by the CLI signal handler on SIGINT.
func (inst *Installer) Cancel() {
	if inst.cancel != nil {
		close(inst.cancel)
	}
}

// emitProgress reports progress to the UI.
func (inst *Installer) emitProgress(label string, percent int, phase string) {
	if inst.OnProgress != nil {
		inst.OnProgress(label, percent, phase)
	}
}

// emitError reports an error to the UI.
func (inst *Installer) emitError(label, message string) {
	if inst.OnError != nil {
		inst.OnError(label, message)
	}
}

// logInfo logs an informational message via the step logger.
func (inst *Installer) logInfo(format string, args ...interface{}) {
	if inst.Logger != nil {
		inst.Logger.Info(format, args...)
	}
}

// logError logs an error message via the step logger.
func (inst *Installer) logError(format string, args ...interface{}) {
	if inst.Logger != nil {
		inst.Logger.Error(format, args...)
	}
}

// LogInfo logs an informational message via the step logger.
// Exported for use by callers in other packages (e.g. CLI signal handlers).
func (inst *Installer) LogInfo(format string, args ...interface{}) {
	if inst.Logger != nil {
		inst.Logger.Info(format, args...)
	}
}

// LogError logs an error message via the step logger.
// Exported for use by callers in other packages (e.g. CLI signal handlers).
func (inst *Installer) LogError(format string, args ...interface{}) {
	if inst.Logger != nil {
		inst.Logger.Error(format, args...)
	}
}

// simulateStep simulates a step by sleeping for a short duration.
// Used in dry-run mode to give the user a sense of progress.
func simulateStep() error {
	time.Sleep(time.Duration(shared.SimulateStepSleepDuration) * time.Millisecond)
	return nil
}



// Rollback cleans up a partial or failed installation.
// Delegates to the standalone Rollback function.
func (inst *Installer) Rollback() error {
	return Rollback(inst.Logger, inst.diskID)
}

// RunCleanup performs cleanup (unmounts) at most once, regardless of how many
// paths trigger it. Safe to call from defer, OnBeforeClose, signal handlers,
// or panic recovery — sync.Once ensures only the first call does work.
func (inst *Installer) RunCleanup() {
	inst.cleanupOnce.Do(func() {
		inst.logInfo("Cleanup: running")
		if err := CleanupMounts(inst.Logger, inst.diskID); err != nil {
			inst.logError("Cleanup: %v", err)
		} else {
			inst.logInfo("Cleanup: done")
		}
	})
}

// Interrupted reports whether SIGINT was received during installation.
// Used by the CLI to decide the exit code (130 for SIGINT vs 1 for other errors).
func (inst *Installer) Interrupted() bool {
	return inst.interrupted.Load()
}

// SetInterrupted marks the installer as interrupted by SIGINT.
// Exported for use by the CLI signal handler.
func (inst *Installer) SetInterrupted() {
	inst.interrupted.Store(true)
}



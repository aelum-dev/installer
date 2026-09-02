package install

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"aelum-installer/backend/shared"
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

	// manualMode enables dualboot-safe manual partitioning. When set, the
	// format step runs partition.Apply with manualLayout instead of the
	// automatic format.Run. Only the Boot/Root partitions in the layout are
	// touched; every other partition is preserved.
	manualMode   bool
	manualLayout *shared.ManualLayout

	// Logger for step output (injected by CLI/GUI; default is ConsoleLogger)
	Logger Logger

	// Progress callback (nil for CLI, sets up UI events for GUI)
	OnProgress func(label string, percent int, phase string)

	// interrupted is set to true when SIGINT is received. Used by the CLI to
	// decide the exit code (130 for SIGINT vs 1 for other errors).
	interrupted atomic.Bool

	// cancelOnce ensures Cancel() only closes the cancel channel once.
	cancelOnce sync.Once

	// cleanupOnce ensures Cleanup runs at most once, regardless of which
	// path triggers it (defer, OnBeforeClose, a signal handler, or panic recovery).
	cleanupOnce sync.Once

	// cancel is closed when SIGINT is received during a long-running command
	// (e.g. pacstrap). This unblocks scanner goroutines so they return promptly.
	cancel chan struct{}
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

	// In manual mode, validate the layout up front so the user gets a clear
	// error before any step runs.
	if inst.manualMode && inst.manualLayout == nil {
		return fmt.Errorf("manual partition layout not set")
	}

	// Clear previous log
	logPath := shared.GetLogPath()
	_ = os.WriteFile(logPath, nil, shared.LogFileMode)

	// Ensure mounts are cleaned up on any failure so the installer is retryable.
	// Cleanup is guarded by sync.Once — only the first caller wins, regardless
	// of whether it's the defer, OnBeforeClose, a signal handler, or panic recovery.
	defer inst.RunCleanup()

	ctx := inst.context()

	steps := Steps()
	weights := stepWeights()
	totalWeight := 0
	for _, w := range weights {
		totalWeight += w
	}
	cumulativePercent := 0

	for idx, step := range steps {
		label := step.Label()
		if inst.dryRun {
			label = "[DRY RUN] " + label
		}

		startPercent := cumulativePercent
		cumulativePercent += weights[idx]
		endPercent := cumulativePercent * 100 / totalWeight

		// Wire progress callback to update context + emit JSON only.
		// Command() already writes the percentage to stdout, so we don't call Progress() here.
		ctx.ProgressCallback = func(percent int) {
			// Convert step-relative percentage (0-100) to absolute percentage (startPercent-endPercent)
			absPercent := int(float64(startPercent) + (float64(percent) * float64(endPercent-startPercent) / 100.0))
			if inst.Logger != nil {
				inst.Logger.SetProgressContext(label, absPercent)
				inst.Logger.ProgressJSON(label, absPercent, "running")
			}
		}

		inst.emitProgress(label, startPercent, "running")
		inst.logInfo("Starting step: %s", step.Label())

		// Launch tick goroutine: slowly advances percentage during long steps.
		// Fast steps (format, mount) finish before the first tick fires.
		stopTick := make(chan struct{})
		var stepDuration time.Duration
		if inst.dryRun {
			stepDuration = time.Duration(shared.DryRunStepDurationMs) * time.Millisecond
		}
		go inst.tickProgress(label, startPercent, endPercent, stepDuration, stopTick)

		var err error
		if inst.dryRun {
			err = simulateStep(ctx.Cancel, time.Duration(shared.DryRunStepDurationMs)*time.Millisecond)
		} else {
			err = step.Run(ctx)
		}
		close(stopTick) // stop the tick goroutine

		if err != nil {
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
// In dry-run mode, stepDuration controls the total sleep time and tick interval
// is calculated so that 1% is emitted per tick (e.g. 22s step → ~1 tick/sec).
func (inst *Installer) tickProgress(label string, start, end int, stepDuration time.Duration, stopTick chan struct{}) {
	if start >= end {
		return
	}

	current := start
	var ticker *time.Ticker

	if stepDuration > 0 {
		// Dry-run: tick interval proportional to step duration so 1% ≈ 1 tick.
		percentRange := end - start
		if percentRange <= 0 {
			return
		}
		intervalMs := int(stepDuration.Milliseconds()) / percentRange
		if intervalMs < 100 {
			intervalMs = 100
		}
		ticker = time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	} else {
		// Real install: initial delay then slow ticks (pacstrap drives progress).
		select {
		case <-time.After(time.Duration(shared.ProgressTickDelayMs) * time.Millisecond):
		case <-stopTick:
			return
		}
		ticker = time.NewTicker(time.Duration(shared.ProgressTickIntervalMs) * time.Millisecond)
	}
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			current++
			if current > end {
				current = end
			}
			// Only emit JSON in dry-run mode — normal mode gets progress from pacstrap output.
			if inst.Logger != nil {
				inst.Logger.UpdateContext(label, current)
				if stepDuration > 0 {
					inst.Logger.ProgressJSON(label, current, "running")
				}
			}
			if current >= end {
				return
			}
		case <-stopTick:
			return
		}
	}
}

// installationSteps returns the list of installation steps.
func (inst *Installer) installationSteps() []Step {
	return Steps()
}

// validStepNames returns the list of valid step names for error messages.
func validStepNames() []string {
	steps := Steps()
	names := make([]string, len(steps))
	for i, s := range steps {
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
			ctx := inst.context()
			var stepErr error
			if inst.dryRun {
				stepErr = simulateStep(ctx.Cancel, time.Duration(shared.DryRunStepDurationMs)*time.Millisecond)
			} else {
				stepErr = s.Run(ctx)
			}
			if stepErr != nil {
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
	ctx := &InstallContext{
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
	ctx.ManualMode = inst.manualMode
	ctx.ManualLayout = inst.manualLayout
	return ctx
}

// Cancel closes the install cancellation channel, unblocking any scanner
// goroutines (e.g. pacstrap output reader) that are waiting on input.
// Called by the CLI signal handler on SIGINT.
func (inst *Installer) Cancel() {
	inst.cancelOnce.Do(func() {
		if inst.cancel != nil {
			close(inst.cancel)
		}
	})
}

// emitProgress reports progress to the UI.
func (inst *Installer) emitProgress(label string, percent int, phase string) {
	if inst.OnProgress != nil {
		inst.OnProgress(label, percent, phase)
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

// simulateStep simulates a step by sleeping for the given duration.
// Respects the cancel channel — exits early if SIGINT is received.
// Used in dry-run mode to give the user a sense of progress.
// Can be overridden in tests for faster execution.
var simulateStep = func(cancel chan struct{}, duration time.Duration) error {
	select {
	case <-time.After(duration):
		return nil
	case <-cancel:
		return fmt.Errorf("interrupted")
	}
}

// SetManualLayout enables manual (dualboot-safe) partitioning with the given
// layout. The layout is validated by the orchestrator before the first step.
func (inst *Installer) SetManualLayout(layout *shared.ManualLayout) {
	inst.manualMode = true
	inst.manualLayout = layout
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
		inst.logInfo("Cleanup: started")
		if err := CleanupMounts(inst.Logger); err != nil {
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

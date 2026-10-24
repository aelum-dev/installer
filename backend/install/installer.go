package install

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aelum-installer/backend/install/luks"
	"aelum-installer/backend/install/lvm"
	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// InstallationProgress is emitted as an event to the frontend.
type InstallationProgress struct {
	StepID  string `json:"stepId"` // stable step identifier, localized by the UI
	Label   string `json:"label"`
	Percent int    `json:"percent"`
	Phase   string `json:"phase"` // "running" | "done" | "error"
}

// Installer orchestrates the full installation process.
// Single source of truth — used by both CLI and GUI.
type Installer struct {
	diskID         string
	username       string
	password       []byte // zeroed after being copied to InstallContext
	hostname       string
	region         shared.Region
	fileSystem     string // root filesystem: "ext4" or "btrfs"
	luks           bool
	luksPassphrase []byte // zeroed after being copied to InstallContext
	lvm            bool
	dryRun         bool
	esp            string // ESP mount point (default: /mnt/efi)

	// manualMode enables manual partitioning. When set, the format step runs
	// partition.Apply with manualLayout instead of the automatic format.Run.
	// Only the Boot/System partitions in the layout are formatted; every other
	// partition is preserved.
	manualMode   bool
	manualLayout *shared.ManualLayout

	// Logger for step output (injected by CLI/GUI; default is ConsoleLogger)
	Logger shared.Logger

	// Progress callback (nil for CLI, sets up UI events for GUI)
	OnProgress func(stepID, label string, percent int, phase string)

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

// InstallerConfig holds the inputs for a new installation run.
//
// Using a struct instead of a long positional parameter list prevents callers
// from accidentally transposing the many string and bool arguments (for
// example, swapping the account password for the LUKS passphrase) and makes
// future fields additive instead of signature-breaking.
type InstallerConfig struct {
	DiskID         string
	Username       string
	Password       string
	Hostname       string
	Region         shared.Region
	FileSystem     string // root filesystem: "ext4" or "btrfs"
	Luks           bool
	LuksPassphrase string
	// Lvm places the root (and, for ext4, home) on logical volumes. LUKS always
	// sits below LVM, so enabling both encrypts the single physical volume.
	Lvm    bool
	DryRun bool
}

// NewInstaller creates a new Installer instance from cfg.
func NewInstaller(cfg InstallerConfig) *Installer {
	return &Installer{
		diskID:         cfg.DiskID,
		username:       cfg.Username,
		password:       []byte(cfg.Password),
		hostname:       cfg.Hostname,
		region:         cfg.Region,
		fileSystem:     cfg.FileSystem,
		luks:           cfg.Luks,
		luksPassphrase: []byte(cfg.LuksPassphrase),
		lvm:            cfg.Lvm,
		dryRun:         cfg.DryRun,
		esp:            shared.ESPMountPoint,
		Logger:         shared.NewConsoleLogger(),
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

	// Clear previous log (primary + optional /shared mirror).
	for _, logPath := range []string{shared.GetLogPath(), shared.GetLogMirrorPath()} {
		if logPath != "" {
			_ = os.WriteFile(logPath, nil, shared.LogFileMode)
		}
	}

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
		stepID := step.Name()
		label := step.Label()
		if inst.dryRun {
			label = "[DRY RUN] " + label
		}

		startPercent := cumulativePercent
		cumulativePercent += weights[idx]
		endPercent := cumulativePercent * 100 / totalWeight

		// high is this step's high-water mark (absolute percent). Both the
		// parsed pacstrap progress and the gap-filling ticker raise it, so the
		// bar never moves backwards and the ticker never overtakes real
		// progress.
		var high atomic.Int32
		high.Store(int32(startPercent))

		// Wire progress callback to update context + emit JSON only.
		// Command() already writes the percentage to stdout, so we don't call Progress() here.
		ctx.ProgressCallback = func(percent int) {
			// Convert step-relative percentage (0-100) to absolute percentage (startPercent-endPercent)
			absPercent := int(float64(startPercent) + (float64(percent) * float64(endPercent-startPercent) / 100.0))
			inst.raiseProgress(stepID, label, &high, absPercent)
		}

		inst.emitProgress(stepID, label, startPercent, "running")
		inst.logInfo("Starting step: %s", step.Label())

		// Launch tick goroutine: slowly advances percentage during long steps.
		// Fast steps (format, mount) finish before the first tick fires.
		stopTick := make(chan struct{})
		var stepDuration time.Duration
		if inst.dryRun {
			stepDuration = time.Duration(shared.DryRunStepDurationMs) * time.Millisecond
		}
		go inst.tickProgress(stepID, label, startPercent, endPercent, stepDuration, stopTick, &high)

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
		inst.emitProgress(stepID, label, endPercent, "done")
	}

	inst.logInfo("Installation completed successfully")
	return nil
}

// tickProgress slowly advances the percentage from start to end, emitting
// progress events at regular intervals. Used to keep the UI feeling alive
// during long-running steps (e.g. pacstrap). Stops when stopTick is closed.
//
// high is the shared high-water mark for the step: the ticker only ever raises
// it, so it fills the silent gaps between pacstrap's progress lines without
// overtaking the parsed progress (which raises the same mark). In dry-run mode
// stepDuration controls the total sleep time and tick interval is calculated so
// that 1% is emitted per tick (e.g. 22s step → ~1 tick/sec).
func (inst *Installer) tickProgress(stepID, label string, start, end int, stepDuration time.Duration, stopTick chan struct{}, high *atomic.Int32) {
	if start >= end {
		return
	}

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
		// Real install: initial delay then slow ticks. These fill gaps in
		// pacstrap output (a slow hook or a large extraction can be silent for
		// several seconds); parsed progress always wins because it raises the
		// same high-water mark.
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
			cur := high.Load()
			if int(cur) >= end {
				return
			}
			next := int(cur) + 1
			if next > end {
				next = end
			}
			if !high.CompareAndSwap(cur, int32(next)) {
				continue // parsed progress advanced; re-read its value next tick
			}
			if inst.Logger != nil {
				inst.Logger.SetProgressContext(label, next)
				inst.Logger.ProgressJSON(stepID, label, next, "running")
			}
		case <-stopTick:
			return
		}
	}
}

// raiseProgress records pct as the step's high-water mark, and when it advances
// updates the log context and emits a JSON progress event. Returns true when the
// mark advanced.
func (inst *Installer) raiseProgress(stepID, label string, high *atomic.Int32, pct int) bool {
	for {
		cur := high.Load()
		if int32(pct) <= cur {
			return false
		}
		if high.CompareAndSwap(cur, int32(pct)) {
			break
		}
	}
	if inst.Logger != nil {
		inst.Logger.SetProgressContext(label, pct)
		inst.Logger.ProgressJSON(stepID, label, pct, "running")
	}
	return true
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

	for _, s := range Steps() {
		if s.Name() == stepName {
			label = s.Label()
			if inst.dryRun {
				label = "[DRY RUN] " + label
			}
			inst.emitProgress(s.Name(), label, 0, "running")
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
			inst.emitProgress(s.Name(), label, 100, "done")
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

	var luksPassphrase []byte
	if inst.luks {
		luksPassphrase = make([]byte, len(inst.luksPassphrase))
		copy(luksPassphrase, inst.luksPassphrase)
		// Zero our copy immediately — the context now owns the only copy.
		for k := range inst.luksPassphrase {
			inst.luksPassphrase[k] = 0
		}
	}

	// Create a fresh cancel channel for this installation run.
	inst.cancel = make(chan struct{})

	// Default an unset filesystem so the format/mount steps have a concrete type.
	fileSystem := inst.fileSystem
	if fileSystem == "" {
		fileSystem = shared.DefaultFileSystem
	}

	ctx := &InstallContext{
		DiskID:         inst.diskID,
		ESP:            inst.esp,
		Username:       inst.username,
		Password:       password,
		Hostname:       inst.hostname,
		Region:         inst.region,
		FileSystem:     fileSystem,
		Luks:           inst.luks,
		LuksPassphrase: luksPassphrase,
		Lvm:            inst.lvm,
		Logger:         inst.Logger,
		Cancel:         inst.cancel,
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
func (inst *Installer) emitProgress(stepID, label string, percent int, phase string) {
	if inst.OnProgress != nil {
		inst.OnProgress(stepID, label, percent, phase)
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

// SetManualLayout enables manual partitioning with the given layout. The layout
// is validated by the orchestrator before the first step.
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
		// Unmount everything under /mnt first. The root filesystem may live on
		// a LUKS mapper, and `cryptsetup close` refuses to tear down a device
		// that is still mounted (or otherwise busy). Unmounting first is what
		// actually lets the mapper be released, so a failed install can be
		// retried cleanly.
		if err := primitives.CleanupMounts(inst.Logger); err != nil {
			inst.logError("Cleanup: unmount: %v", err)
		}
		// Deactivate and remove any volume group on the target before the LUKS
		// mapper is closed: an active logical volume keeps the mapper (the PV)
		// open, so cryptsetup close would fail with EBUSY. Best-effort — the
		// cleanup path must always finish.
		if inst.lvm {
			lvm.ReleaseForDisk(inst.diskID, inst.Logger)
		}
		// Close the LUKS mapper (if any) so a failed install can be retried
		// without a leaked dm device. Best-effort — a closed mapper errors.
		// Retried briefly because udev/blkid can hold the device for a moment
		// right after the unmount, and "Device or resource busy" would leave the
		// mapper open, making the next attempt's luksFormat fail with EBUSY.
		if inst.luks {
			if err := closeLuksMapperWithRetry(); err != nil && inst.Logger != nil {
				inst.logInfo("Cleanup: LUKS mapper close (non-fatal): %v", err)
			}
		}
		inst.logInfo("Cleanup: done")
	})
}

// luksCloseRetryDelay is the wait between close attempts. A variable so tests
// can shorten it.
var luksCloseRetryDelay = 200 * time.Millisecond

// luksCloseRetryAttempts is how many times the root mapper close is attempted.
const luksCloseRetryAttempts = 5

// closeLuksMapperWithRetry closes the root LUKS mapper, retrying while the
// device is transiently busy. Non-transient errors (e.g. "not active") are
// returned immediately so cleanup stays fast.
func closeLuksMapperWithRetry() error {
	var err error
	for attempt := 0; attempt < luksCloseRetryAttempts; attempt++ {
		if err = luks.LuksClose(shared.LuksMapperName); err == nil {
			return nil
		}
		if !isTransientCloseError(err) {
			return err
		}
		if attempt < luksCloseRetryAttempts-1 {
			time.Sleep(luksCloseRetryDelay)
		}
	}
	return err
}

// isTransientCloseError reports whether a cryptsetup close failure is worth
// retrying: the mapper still has a holder that is about to go away.
func isTransientCloseError(err error) bool {
	msg := strings.ToLower(err.Error())
	for _, s := range []string{"in use", "busy", "resource temporarily unavailable"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
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

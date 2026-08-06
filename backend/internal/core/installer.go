package core

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"aestheticos-installer/backend/internal/install"
	"aestheticos-installer/backend/internal/shared"
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
	helperPath string
	username   string
	password   []byte // zeroed after being copied to InstallContext
	hostname   string
	region     shared.Region
	fileSystem string // root filesystem: "ext4" or "btrfs"
	dryRun     bool
	esp        string // ESP mount point (default: /mnt/boot)

	// Logger for step output (injected by CLI/GUI; default is ConsoleLogger)
	Logger install.Logger

	// Progress callback (nil for CLI, sets up UI events for GUI)
	OnProgress func(label string, percent int, phase string)
	// Error callback
	OnError func(label, message string)

	// interrupted is set to true when SIGINT is received. Used to prevent
	// double cleanup when both the signal handler and Install()'s defer try
	// to clean up mounted filesystems.
	interrupted atomic.Bool

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
func NewInstaller(diskID, helperPath, username, password, hostname string, region shared.Region, fileSystem string, dryRun bool) *Installer {
	return &Installer{
		diskID:     diskID,
		helperPath: helperPath,
		username:   username,
		password:   []byte(password),
		hostname:   hostname,
		region:     region,
		fileSystem: fileSystem,
		dryRun:     dryRun,
		esp:        install.BootMountPoint,
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
	// Skip cleanup on SIGINT — the signal handler already performs it, and
	// running it twice can race (double unmount, duplicate log output).
	wasInterrupted := false
	defer func() {
		if wasInterrupted {
			inst.logInfo("Cleanup: skipped (SIGINT)")
			return
		}
		inst.logInfo("Cleanup: running deferred cleanup")
		if err := install.CleanupMounts(inst.helperPath, inst.Logger, inst.diskID); err != nil {
			inst.logError("Cleanup: %v", err)
		} else {
			inst.logInfo("Cleanup: done")
		}
	}()

	ctx := inst.context()

	stepList := inst.installationSteps()
	percentages := cumulativePercentages(stepList)

	for idx, step := range stepList {
		label := step.label
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
		inst.logInfo("Starting step: %s", step.label)

		// Launch tick goroutine: slowly advances percentage during long steps.
		// Fast steps (format, mount) finish before the first tick fires.
		stopTick := make(chan struct{})
		go inst.tickProgress(label, startPercent, endPercent, stopTick)

		var err error
		if inst.dryRun {
			err = simulateStep()
		} else {
			err = step.run(ctx)
		}
		close(stopTick) // stop the tick goroutine

		if err != nil {
			if isInterruptError(err) {
				// SIGINT — the signal handler already logged the interruption.
				// Mark so the defer skips its cleanup to avoid duplication.
				wasInterrupted = true
				inst.SetInterrupted()
			} else {
				inst.logError("Step failed: %s - %v", step.label, err)
				inst.emitError(step.label, err.Error())
			}
			return fmt.Errorf("step \"%s\" failed: %w", step.label, err)
		}

		inst.logInfo("Step completed: %s", step.label)
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
	case <-time.After(time.Duration(install.ProgressTickDelayMs) * time.Millisecond):
	case <-stopTick:
		return
	}

	current := start
	ticker := time.NewTicker(time.Duration(install.ProgressTickIntervalMs) * time.Millisecond)
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

// installationStep defines a single step in the installation process.
type installationStep struct {
	name  string // programmatic name, e.g. "format_disk"
	label string // human-readable label, e.g. "Formatting disk"
	run   func(ctx *install.InstallContext) error
}

// stepDefinitions is the canonical list of installation steps.
// Accessible without instantiating an Installer.
var stepDefinitions = []installationStep{
	{name: "format_disk", label: "Formatting disk", run: install.FormatDisk},
	{name: "mount_filesystem", label: "Mounting filesystem", run: install.MountFilesystem},
	{name: "install_base", label: "Installing base system", run: install.Install},
	{name: "install_desktop", label: "Installing desktop environment", run: install.InstallDesktop},
	{name: "setup_users", label: "Setting up users", run: install.SetupUsers},
	{name: "configure_system", label: "Configuring system", run: install.ConfigureSystem},
}

// stepWeights assigns relative weight to each step for progress bar calculation.
// The two pacstrap steps (base + desktop) together account for ~80% of total
// installation time, so they receive proportionally higher weights.
var stepWeights = []int{
	WeightFormatDisk,
	WeightMountFilesystem,
	WeightInstallBase,
	WeightInstallDesktop,
	WeightSetupUsers,
	WeightConfigureSystem,
}

// Progress weight constants for installation steps.
const (
	WeightFormatDisk      = 3
	WeightMountFilesystem = 2
	WeightInstallBase     = 45 // base pacstrap — longest step
	WeightInstallDesktop  = 35 // desktop pacstrap — second longest
	WeightSetupUsers      = 2
	WeightConfigureSystem = 13
)

// cumulativePercentages calculates the cumulative percentage at each step boundary
// based on step weights. Returns a slice of length len(steps)+1, where result[0] = 0
// and result[len] = 100.
func cumulativePercentages(steps []installationStep) []int {
	totalWeight := 0
	for _, w := range stepWeights {
		totalWeight += w
	}

	result := make([]int, len(steps)+1)
	result[0] = 0
	cumulativeWeight := 0
	for i := range steps {
		if i < len(stepWeights) {
			cumulativeWeight += stepWeights[i]
			result[i+1] = int(float64(cumulativeWeight) / float64(totalWeight) * 100)
		}
	}
	return result
}

// installationSteps returns the list of installation steps.
func (inst *Installer) installationSteps() []installationStep {
	return stepDefinitions
}

// validStepNames returns the list of valid step names for error messages.
func validStepNames() []string {
	names := make([]string, len(stepDefinitions))
	for i, s := range stepDefinitions {
		names[i] = s.name
	}
	return names
}

// RunStep runs a single named installation step.
// Returns the step label, the percentage (0-100, always 100 for single step), and any error.
// On failure, cleans up any mounted filesystems so the step can be retried.
func (inst *Installer) RunStep(stepName string) (label string, percent int, err error) {
	// Ensure mounts are cleaned up on failure so the step is retryable.
	defer install.CleanupMounts(inst.helperPath, inst.Logger, inst.diskID)

	allSteps := inst.installationSteps()
	for _, s := range allSteps {
		if s.name == stepName {
			label = s.label
			if inst.dryRun {
				label = "[DRY RUN] " + label
			}
			inst.emitProgress(label, 0, "running")
			inst.logInfo("Starting step: %s", s.label)
			var stepErr error
			if inst.dryRun {
				stepErr = simulateStep()
			} else {
				stepErr = s.run(inst.context())
			}
			if stepErr != nil {
				inst.logError("Step failed: %s - %v", s.label, stepErr)
				inst.emitError(s.label, stepErr.Error())
				return label, 0, fmt.Errorf("step \"%s\" failed: %w", s.label, stepErr)
			}
			inst.emitProgress(label, 100, "done")
			inst.logInfo("Step completed: %s", s.label)
			return label, 100, nil
		}
	}
	valid := validStepNames()
	return "", 0, fmt.Errorf("unknown step %q, valid steps: %v", stepName, valid)
}

// context creates an InstallContext for this installer.
// The password is copied to the context and then zeroed from the Installer
// to minimize the time the plaintext password resides in memory.
func (inst *Installer) context() *install.InstallContext {
	password := make([]byte, len(inst.password))
	copy(password, inst.password)
	// Zero our copy immediately — the context now owns the only copy.
	for k := range inst.password {
		inst.password[k] = 0
	}
	// Create a fresh cancel channel for this installation run.
	inst.cancel = make(chan struct{})
	return &install.InstallContext{
		DiskID:     inst.diskID,
		ESP:        inst.esp,
		Helper:     inst.helperPath,
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
	time.Sleep(time.Duration(install.SimulateStepSleepDuration) * time.Millisecond)
	return nil
}

// Rollback cleans up a partial or failed installation:
// unmounts everything under /mnt, wipes filesystem signatures from all
// partitions and the disk itself, then removes the /mnt mount point.
// After rollback the disk is ready for a fresh installation.
func (inst *Installer) Rollback() error {
	if inst.diskID == "" {
		return fmt.Errorf("disk ID is required")
	}

	inst.logInfo("Rolling back: cleaning up %s", inst.diskID)

	// 1. Unmount everything under /mnt (lazy recursive), kill processes,
	//    partprobe, dmsetup remove_all.
	//    Fail fast if cleanup fails — proceeding to wipe mounted filesystems
	//    can cause kernel panics or silent corruption.
	if err := install.CleanupMounts(inst.helperPath, inst.Logger, inst.diskID); err != nil {
		inst.logError("Rollback: cleanup failed: %v", err)
		return fmt.Errorf("rollback cleanup failed: %w", err)
	}

	// 2. Wipe filesystem signatures from all partitions.
	//    Use lsblk to discover partitions (handles any number, not just 2).
	//    If the disk has no partitions (FormatDisk never completed), skip.
	partitions := discoverPartitions(inst.diskID)
	if len(partitions) == 0 {
		inst.logInfo("Rollback: no partitions found on %s, skipping wipe", inst.diskID)
	} else {
		for _, part := range partitions {
			inst.logInfo("Wiping signatures on %s", part)
			cmd := inst.run("run", "wipefs", "--all", "-f", part)
			if out, err := cmd.CombinedOutput(); err != nil {
				inst.logError("Failed to wipe %s: %v (%s)", part, err, string(out))
				// Don't fail — some partitions may not exist yet if FormatDisk
				// failed early. Continue wiping the rest.
			}
		}
	}

	// 3. Wipe signatures on the disk itself (GPT backup, etc.).
	inst.logInfo("Wiping signatures on %s", inst.diskID)
	cmd := inst.run("run", "wipefs", "--all", "-f", inst.diskID)
	if out, err := cmd.CombinedOutput(); err != nil {
		inst.logError("Failed to wipe disk %s: %v (%s)", inst.diskID, err, string(out))
		// Non-fatal — disk may not have been partitioned yet.
	}

	// 4. Clear the GPT partition table so no stale partition entries remain.
	//    wipefs removes signatures but leaves partition entries intact;
	//    sgdisk -Z removes everything for a completely clean slate.
	inst.logInfo("Clearing GPT partition table on %s", inst.diskID)
	cmd = inst.run("run", "sgdisk", "-Z", inst.diskID)
	if out, err := cmd.CombinedOutput(); err != nil {
		inst.logError("Failed to clear GPT table on %s: %v (%s)", inst.diskID, err, string(out))
		// Non-fatal — disk may not have a GPT table yet.
	}

	// 5. Remove the /mnt mount point directory.
	cmd = inst.run("run", "rmdir", shared.MountRoot)
	_ = cmd.Run()

	inst.logInfo("Rollback complete: %s is ready for a fresh install", inst.diskID)
	return nil
}

// discoverPartitions uses lsblk to find all partition devices for a given disk.
// Returns device paths like /dev/vda1, /dev/vda2, etc.
func discoverPartitions(diskID string) []string {
	cmd := exec.Command("lsblk", "-lnbo", "KNAME,TYPE", diskID)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return nil
	}

	var partitions []string
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[1] == "part" {
			name := fields[0]
			// Avoid double-prefixing if lsblk already returns an absolute path.
			if !strings.HasPrefix(name, "/dev/") {
				name = "/dev/" + name
			}
			partitions = append(partitions, name)
		}
	}
	return partitions
}

// Cleanup runs deferred cleanup (unmounts) for this installer.
// Called on panic recovery to ensure no mounts are left hanging.
func (inst *Installer) Cleanup() {
	if err := install.CleanupMounts(inst.helperPath, inst.Logger, inst.diskID); err != nil {
		inst.logError("Cleanup: %v", err)
	}
}

// Interrupted reports whether SIGINT was received during installation.
func (inst *Installer) Interrupted() bool {
	return inst.interrupted.Load()
}

// SetInterrupted marks the installer as interrupted by SIGINT.
// Exported for use by the CLI signal handler.
func (inst *Installer) SetInterrupted() {
	inst.interrupted.Store(true)
}

// run executes a privileged command via the helper binary.
func (inst *Installer) run(args ...string) *exec.Cmd {
	return exec.Command("sudo", append([]string{inst.helperPath}, args...)...)
}

// FindHelper finds the aestheticos-helper binary next to the running executable.
func FindHelper() string {
	exe, err := os.Executable()
	if err != nil {
		return "./aestheticos-helper"
	}
	return filepath.Dir(exe) + "/aestheticos-helper"
}

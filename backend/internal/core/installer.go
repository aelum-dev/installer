package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	defer install.CleanupMounts(inst.helperPath)

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
			inst.logError("Step failed: %s - %v", step.label, err)
			inst.emitError(step.label, err.Error())
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
func (inst *Installer) RunStep(stepName string) (label string, percent int, err error) {
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

// simulateStep simulates a step by sleeping for a short duration.
// Used in dry-run mode to give the user a sense of progress.
func simulateStep() error {
	time.Sleep(time.Duration(install.SimulateStepSleepDuration) * time.Millisecond)
	return nil
}

// Rollback cleans up a partial or failed installation:
// unmounts everything under /mnt, turns off swap, and wipes filesystem
// signatures from the target disk partitions. After rollback the disk is
// ready for a fresh installation.
func (inst *Installer) Rollback() error {
	if inst.diskID == "" {
		return fmt.Errorf("disk ID is required")
	}

	inst.logInfo("Rolling back: cleaning up %s", inst.diskID)

	// Clean up any mounted filesystems.
	install.CleanupMounts(inst.helperPath)

	// Wipe filesystem signatures from both partitions so the disk is clean.
	for i := 1; i <= 2; i++ {
		partPath := shared.DerivePartitionPath(inst.diskID, i)
		inst.logInfo("Wiping signatures on %s", partPath)
		cmd := inst.run("run", "wipefs", "--all", "-f", partPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			inst.logError("Failed to wipe %s: %v (%s)", partPath, err, string(out))
			return fmt.Errorf("rollback: failed to wipe %s: %w", partPath, err)
		}
	}

	// Reprobe partition table.
	cmd := inst.run(install.HelperCmdRun, install.HelperCmdPartprobe, inst.diskID)
	if out, err := cmd.CombinedOutput(); err != nil {
		inst.logError("partprobe failed: %v (%s)", err, string(out))
		return fmt.Errorf("rollback: partprobe failed: %w", err)
	}

	inst.logInfo("Rollback complete: %s is ready for a fresh install", inst.diskID)
	return nil
}

// Cleanup runs deferred cleanup (unmounts, swapoff) for this installer.
// Called on panic recovery to ensure no mounts are left hanging.
func (inst *Installer) Cleanup() {
	install.CleanupMounts(inst.helperPath)
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

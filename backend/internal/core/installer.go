package core

import (
	"fmt"
	"os"
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
	password   string
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
		password:   password,
		hostname:   hostname,
		region:     region,
		fileSystem: fileSystem,
		dryRun:     dryRun,
		esp:        install.BootMountPoint,
		Logger:     &install.ConsoleLogger{},
	}
}

// Install runs the full installation process.
// Returns nil on success, error on failure.
func (inst *Installer) Install() error {
	if inst.diskID == "" {
		return fmt.Errorf("disk ID is required")
	}
	if inst.region.Code == "" {
		return fmt.Errorf("region not set")
	}

	// Clear previous log
	logPath := shared.GetLogPath()
	os.WriteFile(logPath, nil, shared.LogFileMode)

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
		WriteLog("Starting step: %s", step.label)

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
			WriteLog("Step failed: %s - %v", step.label, err)
			inst.emitError(step.label, err.Error())
			return fmt.Errorf("step \"%s\" failed: %w", step.label, err)
		}

		inst.logInfo("Step completed: %s", step.label)
		WriteLog("Step completed: %s", step.label)
		inst.emitProgress(label, endPercent, "done")
	}

	WriteLog("Installation completed successfully")
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
	{name: "create_swap", label: "Creating swap", run: install.CreateSwap},
	{name: "install", label: "Installing base system", run: install.Install},
	{name: "install_desktop", label: "Installing desktop environment", run: install.InstallDesktop},
	{name: "generate_fstab", label: "Generating fstab", run: install.GenerateFstab},
	{name: "configure_bootloader", label: "Configuring bootloader", run: install.ConfigureBootloader},
	{name: "setup_users", label: "Setting up users", run: install.SetupUsers},
	{name: "configure_system", label: "Configuring system", run: install.ConfigureSystem},
	{name: "configure_greetd", label: "Configuring login manager", run: install.ConfigureGreetd},
	{name: "enable_services", label: "Enabling services", run: install.EnableServices},
}

// stepWeights assigns relative weight to each step for progress bar calculation.
// The two pacstrap steps (base + desktop) together account for ~80% of total
// installation time, so they receive proportionally higher weights.
var stepWeights = []int{
	3,  // format_disk
	2,  // mount_filesystem
	2,  // create_swap
	45, // install (base pacstrap — longest step)
	35, // install_desktop (desktop pacstrap — second longest)
	2,  // generate_fstab
	3,  // configure_bootloader
	2,  // setup_users
	2,  // configure_system
	2,  // configure_greetd
	2,  // enable_services
}

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
func (i *Installer) installationSteps() []installationStep {
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
func (i *Installer) RunStep(stepName string) (label string, percent int, err error) {
	allSteps := i.installationSteps()
	for _, s := range allSteps {
		if s.name == stepName {
			label = s.label
			if i.dryRun {
				label = "[DRY RUN] " + label
			}
			i.emitProgress(label, 0, "running")
			i.logInfo("Starting step: %s", s.label)
			WriteLog("Starting step: %s", s.label)
			var stepErr error
			if i.dryRun {
				stepErr = simulateStep()
			} else {
				stepErr = s.run(i.context())
			}
			if stepErr != nil {
				i.logError("Step failed: %s - %v", s.label, stepErr)
				WriteLog("Step failed: %s - %v", s.label, stepErr)
				i.emitError(s.label, stepErr.Error())
				return label, 0, fmt.Errorf("step \"%s\" failed: %w", s.label, stepErr)
			}
			i.emitProgress(label, 100, "done")
			i.logInfo("Step completed: %s", s.label)
			WriteLog("Step completed: %s", s.label)
			return label, 100, nil
		}
	}
	valid := validStepNames()
	return "", 0, fmt.Errorf("unknown step %q, valid steps: %v", stepName, valid)
}

// context creates an InstallContext for this installer.
func (i *Installer) context() *install.InstallContext {
	return &install.InstallContext{
		DiskID:     i.diskID,
		ESP:        i.esp,
		Helper:     i.helperPath,
		Username:   i.username,
		Password:   i.password,
		Hostname:   i.hostname,
		Region:     i.region,
		FileSystem: i.fileSystem,
		Logger:     i.Logger,
	}
}

// emitProgress reports progress to the UI.
func (i *Installer) emitProgress(label string, percent int, phase string) {
	if i.OnProgress != nil {
		i.OnProgress(label, percent, phase)
	}
}

// emitError reports an error to the UI.
func (i *Installer) emitError(label, message string) {
	if i.OnError != nil {
		i.OnError(label, message)
	}
}

// logInfo logs an informational message via the step logger.
func (i *Installer) logInfo(format string, args ...interface{}) {
	if i.Logger != nil {
		i.Logger.Info(format, args...)
	}
}

// logError logs an error message via the step logger.
func (i *Installer) logError(format string, args ...interface{}) {
	if i.Logger != nil {
		i.Logger.Error(format, args...)
	}
}

// simulateStep simulates a step by sleeping for a short duration.
// Used in dry-run mode to give the user a sense of progress.
func simulateStep() error {
	time.Sleep(time.Duration(install.SimulateStepSleepDuration) * time.Millisecond)
	return nil
}

// FindHelper finds the aestheticos-helper binary.
func FindHelper() string {
	// Try /shared first (testing/live ISO)
	if _, err := os.Stat("/usr/local/bin/aestheticos-installer-helper"); err == nil {
		return "/usr/local/bin/aestheticos-installer-helper"
	}
	// Fallback: next to the main executable
	exe, err := os.Executable()
	if err != nil {
		return "./aestheticos-helper"
	}
	// Get the directory of the executable
	dir := filepath.Dir(exe)
	return dir + "/aestheticos-helper"
}

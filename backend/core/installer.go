package core

import (
	"fmt"
	"os"
	"path/filepath"

	"aesthetic-os-installer/backend/shared"
	"aesthetic-os-installer/backend/steps"
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
	layoutName string // partition layout: "default" or "btrfs-subvolumes"
	dryRun     bool
	esp        string // ESP mount point (default: /mnt/boot)

	// Logger for step output (injected by CLI/GUI; default is ConsoleLogger)
	Logger steps.Logger

	// Progress callback (nil for CLI, sets up UI events for GUI)
	OnProgress func(label string, percent int, phase string)
	// Error callback
	OnError func(label, message string)
}

// NewInstaller creates a new Installer instance.
func NewInstaller(diskID, helperPath, username, password, hostname string, region shared.Region, fileSystem, layoutName string, dryRun bool) *Installer {
	return &Installer{
		diskID:     diskID,
		helperPath: helperPath,
		username:   username,
		password:   password,
		hostname:   hostname,
		region:     region,
		fileSystem: fileSystem,
		layoutName: layoutName,
		dryRun:     dryRun,
		esp:        "/mnt/boot",
		Logger:     &steps.ConsoleLogger{},
	}
}

// Install runs the full installation process.
// Returns nil on success, error on failure.
func (inst *Installer) Install() error {
	if inst.diskID == "" {
		return fmt.Errorf("disk ID is required")
	}

	// Clear previous log
	logPath := GetLogPath()
	os.WriteFile(logPath, nil, 0644)

	ctx := &steps.InstallContext{
		DiskID:   inst.diskID,
		ESP:      inst.esp,
		Helper:   inst.helperPath,
		Username: inst.username,
		Password: inst.password,
		Hostname: inst.hostname,
		Region:   inst.region,
		Logger:   inst.Logger,
	}

	stepList := inst.installationSteps()

	for idx, step := range stepList {
		label := step.label
		if inst.dryRun {
			label = "[DRY RUN] " + label
		}

		inst.emitProgress(label, int(float64(idx)/float64(len(stepList))*100), "running")
		inst.logInfo("Starting step: %s", step.label)
		WriteLog("Starting step: %s", step.label)

		var err error
		if inst.dryRun {
			err = simulateStep()
		} else {
			err = step.run(ctx)
		}

		if err != nil {
			inst.logError("Step failed: %s - %v", step.label, err)
			WriteLog("Step failed: %s - %v", step.label, err)
			inst.emitError(step.label, err.Error())
			return fmt.Errorf("step \"%s\" failed: %w", step.label, err)
		}

		inst.logInfo("Step completed: %s", step.label)
		WriteLog("Step completed: %s", step.label)
		percent := int(float64(idx+1) / float64(len(stepList)) * 100)
		inst.emitProgress(label, percent, "done")
	}

	WriteLog("Installation completed successfully")
	return nil
}

// installationStep defines a single step in the installation process.
type installationStep struct {
	name  string // programmatic name, e.g. "format_disk"
	label string // human-readable label, e.g. "Formatting disk"
	run   func(ctx *steps.InstallContext) error
}

// installationSteps returns the list of installation steps.
func (i *Installer) installationSteps() []installationStep {
	return []installationStep{
		{name: "format_disk", label: "Formatting disk", run: steps.FormatDisk},
		{name: "mount_filesystem", label: "Mounting filesystem", run: steps.MountFilesystem},
		{name: "install", label: "Installing base system", run: steps.Install},
		{name: "generate_fstab", label: "Generating fstab", run: steps.GenerateFstab},
		{name: "configure_bootloader", label: "Configuring bootloader", run: steps.ConfigureBootloader},
		{name: "setup_users", label: "Setting up users", run: steps.SetupUsers},
		{name: "configure_system", label: "Configuring system", run: steps.ConfigureSystem},
	}
}

// validStepNames returns the list of valid step names for error messages.
func validStepNames() []string {
	steps := (&Installer{}).installationSteps()
	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.name
	}
	return names
}

// RunStep runs a single named installation step.
// Returns the step label, the percentage (0-100, always 100 for single step), and any error.
func (i *Installer) RunStep(stepName string) (label string, percent int, err error) {
	// Clear previous log for standalone step runs
	logPath := GetLogPath()
	os.WriteFile(logPath, nil, 0644)
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
func (i *Installer) context() *steps.InstallContext {
	return &steps.InstallContext{
		DiskID:     i.diskID,
		ESP:        i.esp,
		Helper:     i.helperPath,
		Username:   i.username,
		Password:   i.password,
		Hostname:   i.hostname,
		Region:     i.region,
		FileSystem: i.fileSystem,
		LayoutName: i.layoutName,
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

// simulateStep simulates a step by sleeping for a random duration.
func simulateStep() error {
	// Not used anymore — kept for compatibility
	return nil
}

// FindHelper finds the aestheticos-helper binary.
func FindHelper() string {
	// Try /shared first (testing/live ISO)
	if _, err := os.Stat("/shared/aestheticos-helper"); err == nil {
		return "/shared/aestheticos-helper"
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

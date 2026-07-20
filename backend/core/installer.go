package core

import (
	"fmt"
	"os"

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
	locale     string
	dryRun     bool

	// Progress callback (nil for CLI, sets up UI events for GUI)
	OnProgress func(label string, percent int, phase string)
	// Error callback
	OnError func(label, message string)
}

// NewInstaller creates a new Installer instance.
func NewInstaller(diskID, helperPath, username, password, locale string, dryRun bool) *Installer {
	return &Installer{
		diskID:     diskID,
		helperPath: helperPath,
		username:   username,
		password:   password,
		locale:     locale,
		dryRun:     dryRun,
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

	steps := inst.installationSteps()

	for idx, step := range steps {
		label := step.label
		if inst.dryRun {
			label = "[DRY RUN] " + label
		}

		inst.emitProgress(label, int(float64(idx)/float64(len(steps))*100), "running")
		WriteLog("Starting step: %s", step.label)

		var err error
		if inst.dryRun {
			err = simulateStep()
		} else {
			err = step.run(inst.diskID, inst.helperPath, inst.username, inst.password, inst.locale)
		}

		if err != nil {
			WriteLog("Step failed: %s - %v", step.label, err)
			inst.emitError(step.label, err.Error())
			return fmt.Errorf("step \"%s\" failed: %w", step.label, err)
		}

		WriteLog("Step completed: %s", step.label)
		percent := int(float64(idx+1) / float64(len(steps)) * 100)
		inst.emitProgress(label, percent, "done")
	}

	WriteLog("Installation completed successfully")
	return nil
}

// installationSteps returns the list of installation steps.
func (i *Installer) installationSteps() []installationStep {
	return []installationStep{
		{label: "Formatting disk", run: steps.FormatDisk},
		{label: "Mounting filesystem", run: steps.MountFilesystem},
		{label: "Installing base system", run: steps.Install},
		{label: "Generating fstab", run: steps.GenerateFstab},
		{label: "Configuring bootloader", run: func(diskID, helper, username, password, language string) error {
			return steps.ConfigureBootloader(diskID, helper, "/mnt/boot/efi", username, password, language)
		}},
		{label: "Setting up users", run: steps.SetupUsers},
		{label: "Configuring system", run: steps.ConfigureSystem},
	}
}

// installationStep defines a single step in the installation process.
type installationStep struct {
	label string
	run   func(diskID, helper, username, password, language string) error
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
	dir := exe[:len(exe)-len("aestheticos-installer")]
	return dir + "aestheticos-helper"
}

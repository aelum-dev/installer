package app

import (
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"aesthetic-os-installer/backend/steps"

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

// StartInstallation begins the installation process asynchronously.
// Privileged operations are performed via pkexec using the root-helper binary.
func (a *App) StartInstallation() error {
	// Run installation in a goroutine so it doesn't block the UI
	go a.runInstallation()
	return nil
}

func (a *App) runInstallation() {
	totalSteps := len(installationSteps)

	for i, step := range installationSteps {
		// Report step start
		label := step.label
		if a.dryRun {
			label = "[DRY RUN] " + step.label
		}
		a.emitProgress(label, int(float64(i)/float64(totalSteps)*100), "running")

		// Execute the step (or simulate in dry-run mode)
		var err error
		helper := findHelper()

		// Execute the step (or simulate in dry-run mode)
		if a.dryRun {
			err = simulateStep()
		} else {
			err = step.run(a.diskSelection.DiskID, helper, a.userAccount.Username, a.userAccount.Password, a.selectedLanguage)
		}

		if err != nil {
			a.emitError(step.label, err.Error())
			return
		}

		// Report step done
		percent := int(float64(i+1) / float64(totalSteps) * 100)
		a.emitProgress(label, percent, "done")
	}

	// Report completion
	label := "Installation completed!"
	if a.dryRun {
		label = "[DRY RUN] Installation completed!"
	}
	wailsRuntime.EventsEmit(a.ctx, EventInstallationComplete, InstallationProgress{
		Label:   label,
		Percent: 100,
		Phase:   "done",
	})
}

// findHelper finds the root-helper binary.
// For testing: looks in /shared first, then next to the main executable.
func findHelper() string {
	// Try /shared first (testing)
	if _, err := os.Stat("/shared/root-helper"); err == nil {
		return "/shared/root-helper"
	}
	// Fallback: next to the main executable
	exe, err := os.Executable()
	if err != nil {
		return "./root-helper"
	}
	dir := filepath.Dir(exe)
	return filepath.Join(dir, "root-helper")
}

// simulateStep simulates a step by sleeping for a random duration.
func simulateStep() error {
	duration := time.Duration(600+rand.Intn(1400)) * time.Millisecond
	time.Sleep(duration)
	return nil
}

func (a *App) emitProgress(label string, percent int, phase string) {
	progress := InstallationProgress{
		Label:   label,
		Percent: percent,
		Phase:   phase,
	}
	wailsRuntime.EventsEmit(a.ctx, EventInstallationProgress, progress)
}

func (a *App) emitError(label string, message string) {
	progress := InstallationProgress{
		Label:   label,
		Percent: 0,
		Phase:   "error",
	}
	wailsRuntime.EventsEmit(a.ctx, EventInstallationError, progress, message)
}

// installationStep defines a single step in the installation process.
type installationStep struct {
	label string
	run   func(diskID string, helper string, username string, password string, language string) error
}

var installationSteps = []installationStep{
	{
		label: "Formatting disk",
		run:   steps.FormatDisk,
	},
	{
		label: "Mounting filesystem",
		run:   steps.MountFilesystem,
	},
	{
		label: "Installing base system",
		run:   steps.InstallBase,
	},
	{
		label: "Configuring bootloader",
		run:   steps.ConfigureBootloader,
	},
	{
		label: "Setting up users",
		run:   steps.SetupUsers,
	},
	{
		label: "Installing packages",
		run:   steps.InstallPackages,
	},
	{
		label: "Configuring system",
		run:   steps.ConfigureSystem,
	},
}

package app

import (
	"fmt"
	"math/rand"
	"runtime"
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
	// Skip root check in dry-run mode
	if !a.dryRun && runtime.GOOS == "linux" {
		if !isRoot() {
			return fmt.Errorf("installation requires root privileges (run with sudo)")
		}
	}

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
		if a.dryRun {
			err = simulateStep()
		} else {
			err = step.run(a.diskSelection.DiskID, a.userAccount.Username, a.userAccount.Password, a.selectedLanguage)
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
	run   func(diskID string, username string, password string, language string) error
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

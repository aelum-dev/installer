package gui

import (
	"fmt"

	"aesthetic-os-installer/backend/core"
	"aesthetic-os-installer/backend/shared"
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

// StartInstallation begins the installation process using the shared Installer.
func (a *App) StartInstallation() error {
	// Safety check: verify root privileges one more time before starting
	rootCheck := a.CheckRoot()
	if !rootCheck.Authorized {
		wailsRuntime.LogError(a.ctx, fmt.Sprintf("Root check failed: %s", rootCheck.Message))
		return fmt.Errorf("root check failed: %s", rootCheck.Message)
	}

	helper := core.FindHelper()

	// Cleanup leftover mounts before starting
	if err := steps.CleanupMounts(helper); err != nil {
		wailsRuntime.LogWarning(a.ctx, fmt.Sprintf("Failed to cleanup mounts: %v", err))
	}

	// Use provided hostname or fall back to default
	hostname := a.hostname
	if hostname == "" {
		hostname = "aestheticos"
	}

	// Auto-determine layout based on filesystem type
	layout := a.layout
	if layout == "" {
		switch a.fileSystem {
		case "ext4":
			layout = shared.LayoutExt4
		case "btrfs":
			layout = shared.LayoutBtrfs
		default:
			layout = shared.LayoutExt4 // fallback
		}
	}

	installer := core.NewInstaller(
		a.diskSelection.DiskID,
		helper,
		a.userAccount.Username,
		a.userAccount.Password,
		hostname,
		a.region,
		a.fileSystem,
		layout,
		a.dryRun,
	)

	// Set up progress/error callbacks for UI
	installer.OnProgress = func(label string, percent int, phase string) {
		progress := InstallationProgress{
			Label:   label,
			Percent: percent,
			Phase:   phase,
		}
		wailsRuntime.EventsEmit(a.ctx, EventInstallationProgress, progress)
	}
	installer.OnError = func(label, message string) {
		logPath := WriteErrorLog(label, message)
		progress := InstallationProgress{
			Label:   label,
			Percent: 0,
			Phase:   "error",
		}
		wailsRuntime.EventsEmit(a.ctx, EventInstallationError, progress, message, logPath)
	}

	// Run installation in a goroutine so it doesn't block the UI
	go func() {
		err := installer.Install()
		if err != nil {
			return
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
	}()

	return nil
}

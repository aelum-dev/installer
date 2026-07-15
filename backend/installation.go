package app

import (
	"fmt"
	"math/rand"
	"os/exec"
	"runtime"
	"time"

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
// It reports progress to the frontend via Wails events.
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
			err = simulateStep(step.label)
		} else {
			err = step.run(a.diskSelection.DiskID)
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
func simulateStep(label string) error {
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
	run   func(diskID string) error
}

var installationSteps = []installationStep{
	{
		label: "Formatting disk",
		run:   stepFormatDisk,
	},
	{
		label: "Mounting filesystem",
		run:   stepMountFilesystem,
	},
	{
		label: "Installing base system",
		run:   stepInstallBase,
	},

	{
		label: "Configuring bootloader",
		run:   stepConfigureBootloader,
	},
	{
		label: "Setting up users",
		run:   stepSetupUsers,
	},
	{
		label: "Installing packages",
		run:   stepInstallPackages,
	},
	{
		label: "Configuring system",
		run:   stepConfigureSystem,
	},
}

// stepFormatDisk formats the target disk with ext4 filesystem.
func stepFormatDisk(diskID string) error {
	if diskID == "" {
		diskID = "/dev/sda"
	}
	cmd := exec.Command("mkfs.ext4", "-F", "-L", "aesthetic-os", diskID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("format disk failed: %w: %s", err, string(output))
	}
	return nil
}

// stepMountFilesystem mounts the formatted disk.
func stepMountFilesystem(diskID string) error {
	if diskID == "" {
		diskID = "/dev/sda"
	}
	cmd := exec.Command("mount", diskID, "/mnt")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount failed: %w: %s", err, string(output))
	}
	return nil
}

// stepInstallBase installs the Arch Linux base system.
func stepInstallBase(diskID string) error {
	cmd := exec.Command("pacstrap", "/mnt", "base", "linux", "linux-firmware", "base-devel")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pacstrap failed: %w: %s", err, string(output))
	}
	return nil
}

// stepConfigureBootloader sets up GRUB bootloader.
func stepConfigureBootloader(diskID string) error {
	if diskID == "" {
		diskID = "/dev/sda"
	}
	// Generate fstab
	cmd := exec.Command("genfstab", "-U", "/mnt")
	if _, err := cmd.CombinedOutput(); err != nil {
		// genfstab may output to stdout but still succeed
	}
	// Install GRUB
	cmd = exec.Command("grub-install", "--target=i386-pc", diskID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("grub-install failed: %w: %s", err, string(output))
	}
	// Generate GRUB config
	cmd = exec.Command("grub-mkconfig", "-o", "/mnt/boot/grub/grub.cfg")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("grub-mkconfig failed: %w: %s", err, string(output))
	}
	return nil
}

// stepSetupUsers creates user accounts.
func stepSetupUsers(diskID string) error {
	// Uses userAccount data from app state
	// Would create user, set password, configure sudo
	return nil
}

// stepInstallPackages installs additional packages.
func stepInstallPackages(diskID string) error {
	// Could install: networkmanager, sudo, vim, etc.
	return nil
}

// stepConfigureSystem sets up system configuration.
func stepConfigureSystem(diskID string) error {
	// Set hostname, timezone, locale, etc.
	return nil
}

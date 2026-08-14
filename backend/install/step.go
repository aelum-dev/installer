package install

import (
	"aestheticos-installer/backend/install/clearcache"
	"aestheticos-installer/backend/install/configure"
	"aestheticos-installer/backend/install/format"
	"aestheticos-installer/backend/install/installbase"
	"aestheticos-installer/backend/install/installdesktop"
	"aestheticos-installer/backend/install/mount"
	"aestheticos-installer/backend/install/users"
	"aestheticos-installer/backend/shared"
)

// Step defines a single installation step.
type Step interface {
	Name() string
	Label() string
	Run(ctx *shared.InstallContext) error
}

// stepFunc is a function that implements the Step interface.
type stepFunc struct {
	name  string
	label string
	fn    func(*shared.InstallContext) error
}

func (s *stepFunc) Name() string  { return s.name }
func (s *stepFunc) Label() string { return s.label }
func (s *stepFunc) Run(ctx *shared.InstallContext) error {
	return s.fn(ctx)
}

// DefaultSteps returns the canonical list of installation steps.
func Steps() []Step {
	return []Step{
		&stepFunc{"format_disk", "Formatting disk", format.Run},
		&stepFunc{"mount_filesystem", "Mounting filesystem", mount.Run},
		&stepFunc{"install_base", "Installing system", installbase.Run},
		&stepFunc{"install_desktop", "Installing desktop", installdesktop.Run},
		&stepFunc{"setup_users", "Setting up users", users.Run},
		&stepFunc{"configure_system", "Configuring system", configure.Run},
		&stepFunc{"clear_cache", "Clearing package cache", clearcache.Run},
	}
}

// stepWeights returns the canonical weight constants for installation steps.
func stepWeights() []int {
	return []int{
		shared.WeightFormatDisk,
		shared.WeightMountFilesystem,
		shared.WeightInstallBase,
		shared.WeightInstallDesktop,
		shared.WeightSetupUsers,
		shared.WeightConfigureSystem,
		shared.WeightClearCache,
	}
}

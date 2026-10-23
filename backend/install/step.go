package install

import (
	"aelum-installer/backend/install/clearcache"
	"aelum-installer/backend/install/configure"
	"aelum-installer/backend/install/format"
	"aelum-installer/backend/install/installbase"
	"aelum-installer/backend/install/installdesktop"
	"aelum-installer/backend/install/mount"
	"aelum-installer/backend/install/partition"
	"aelum-installer/backend/install/users"
	"aelum-installer/backend/install/verify"
	"aelum-installer/backend/shared"
)

// formatDisk runs the format step. In manual mode it delegates to
// partition.Apply, which only formats the user-designated Boot and System
// partitions; otherwise it runs the automatic format.Run.
func formatDisk(ctx *shared.InstallContext) error {
	if ctx.ManualMode && ctx.ManualLayout != nil {
		return partition.Apply(ctx, ctx.ManualLayout)
	}
	return format.Run(ctx)
}

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
		&stepFunc{"format_disk", "Formatting disk", formatDisk},
		&stepFunc{"mount_filesystem", "Mounting filesystem", mount.Run},
		&stepFunc{"install_base", "Installing system", installbase.Run},
		&stepFunc{"install_desktop", "Installing desktop", installdesktop.Run},
		&stepFunc{"setup_users", "Setting up users", users.Run},
		&stepFunc{"configure_system", "Configuring system", configure.Run},
		&stepFunc{"clear_cache", "Clearing package cache", clearcache.Run},
		&stepFunc{"verify_system", "Verifying system", verify.Run},
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
		shared.WeightVerifySystem,
	}
}

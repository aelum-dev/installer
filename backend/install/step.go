package install

import "aestheticos-installer/backend/shared"

// Step defines a single installation step.
type Step interface {
	Name() string
	Label() string
	Run(ctx *InstallContext) error
}

// --- Step implementations ---------------------------------------------------

type FormatDiskStep struct{}

func (s *FormatDiskStep) Name() string  { return "format_disk" }
func (s *FormatDiskStep) Label() string { return "Formatting disk" }
func (s *FormatDiskStep) Run(ctx *InstallContext) error {
	return FormatDisk(ctx)
}

type MountFilesystemStep struct{}

func (s *MountFilesystemStep) Name() string  { return "mount_filesystem" }
func (s *MountFilesystemStep) Label() string { return "Mounting filesystem" }
func (s *MountFilesystemStep) Run(ctx *InstallContext) error {
	return MountFilesystem(ctx)
}

type InstallBaseStep struct{}

func (s *InstallBaseStep) Name() string  { return "install_base" }
func (s *InstallBaseStep) Label() string { return "Installing system" }
func (s *InstallBaseStep) Run(ctx *InstallContext) error {
	return Install(ctx)
}

type InstallDesktopStep struct{}

func (s *InstallDesktopStep) Name() string  { return "install_desktop" }
func (s *InstallDesktopStep) Label() string { return "Installing desktop" }
func (s *InstallDesktopStep) Run(ctx *InstallContext) error {
	return InstallDesktop(ctx)
}

type SetupUsersStep struct{}

func (s *SetupUsersStep) Name() string  { return "setup_users" }
func (s *SetupUsersStep) Label() string { return "Setting up users" }
func (s *SetupUsersStep) Run(ctx *InstallContext) error {
	return SetupUsers(ctx)
}

type ConfigureSystemStep struct{}

func (s *ConfigureSystemStep) Name() string  { return "configure_system" }
func (s *ConfigureSystemStep) Label() string { return "Configuring system" }
func (s *ConfigureSystemStep) Run(ctx *InstallContext) error {
	return ConfigureSystem(ctx)
}

// DefaultSteps returns the canonical list of installation steps.
func Steps() []Step {
	return []Step{
		&FormatDiskStep{},
		&MountFilesystemStep{},
		&InstallBaseStep{},
		&InstallDesktopStep{},
		&SetupUsersStep{},
		&ConfigureSystemStep{},
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
	}
}

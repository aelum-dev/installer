package install


// Step defines a single installation step.
// Each step has a unique name, a human-readable label, and a Run method.
type Step interface {
	// Name returns the programmatic step name (e.g. "format_disk").
	Name() string
	// Label returns the human-readable step label (e.g. "Formatting disk").
	Label() string
	// Run executes the step with the given context.
	Run(ctx *InstallContext) error
}

// --- Step implementations ---------------------------------------------------

// FormatDiskStep formats and partitions the target disk.
type FormatDiskStep struct{}

func (s *FormatDiskStep) Name() string  { return "format_disk" }
func (s *FormatDiskStep) Label() string { return "Formatting disk" }
func (s *FormatDiskStep) Run(ctx *InstallContext) error {
	return FormatDisk(ctx)
}

// MountFilesystemStep mounts the formatted partitions.
type MountFilesystemStep struct{}

func (s *MountFilesystemStep) Name() string  { return "mount_filesystem" }
func (s *MountFilesystemStep) Label() string { return "Mounting filesystem" }
func (s *MountFilesystemStep) Run(ctx *InstallContext) error {
	return MountFilesystem(ctx)
}

// InstallBaseStep installs the base system packages via pacstrap.
type InstallBaseStep struct{}

func (s *InstallBaseStep) Name() string  { return "install_base" }
func (s *InstallBaseStep) Label() string { return "Installing base system" }
func (s *InstallBaseStep) Run(ctx *InstallContext) error {
	return Install(ctx)
}

// InstallDesktopStep installs the desktop environment packages via pacstrap.
type InstallDesktopStep struct{}

func (s *InstallDesktopStep) Name() string  { return "install_desktop" }
func (s *InstallDesktopStep) Label() string { return "Installing desktop environment" }
func (s *InstallDesktopStep) Run(ctx *InstallContext) error {
	return InstallDesktop(ctx)
}

// SetupUsersStep creates user accounts and copies ISO skel files.
type SetupUsersStep struct{}

func (s *SetupUsersStep) Name() string  { return "setup_users" }
func (s *SetupUsersStep) Label() string { return "Setting up users" }
func (s *SetupUsersStep) Run(ctx *InstallContext) error {
	return SetupUsers(ctx)
}

// ConfigureSystemStep configures fstab, bootloader, hostname, locale, greetd,
// Plymouth, zram, and systemd services.
type ConfigureSystemStep struct{}

func (s *ConfigureSystemStep) Name() string  { return "configure_system" }
func (s *ConfigureSystemStep) Label() string { return "Configuring system" }
func (s *ConfigureSystemStep) Run(ctx *InstallContext) error {
	return ConfigureSystem(ctx)
}

// DefaultSteps returns the canonical list of installation steps.
// Accessible without instantiating an Installer.
func DefaultSteps() []Step {
	return []Step{
		&FormatDiskStep{},
		&MountFilesystemStep{},
		&InstallBaseStep{},
		&InstallDesktopStep{},
		&SetupUsersStep{},
		&ConfigureSystemStep{},
	}
}

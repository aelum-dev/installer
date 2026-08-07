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
func (s *InstallBaseStep) Label() string { return "Installing base system" }
func (s *InstallBaseStep) Run(ctx *InstallContext) error {
	return Install(ctx)
}

type InstallDesktopStep struct{}

func (s *InstallDesktopStep) Name() string  { return "install_desktop" }
func (s *InstallDesktopStep) Label() string { return "Installing desktop environment" }
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

// ============================================================================
// Plan
// ============================================================================

// Plan defines an ordered list of installation steps with relative weights.
type Plan struct {
	Steps   []Step
	Weights []int
}

// DefaultPlan returns the canonical installation plan.
func DefaultPlan() *Plan {
	return &Plan{
		Steps: DefaultSteps(),
		Weights: []int{
			shared.WeightFormatDisk,
			shared.WeightMountFilesystem,
			shared.WeightInstallBase,
			shared.WeightInstallDesktop,
			shared.WeightSetupUsers,
			shared.WeightConfigureSystem,
		},
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

// cumulativePercentages calculates the cumulative percentage at each step boundary.
func (p *Plan) cumulativePercentages() []int {
	total := 0
	for _, w := range p.Weights {
		total += w
	}
	if total == 0 {
		return make([]int, len(p.Steps)+1)
	}
	percentages := make([]int, len(p.Steps)+1)
	cumulative := 0
	for i := 0; i < len(p.Steps); i++ {
		percentages[i] = cumulative * 100 / total
		cumulative += p.Weights[i]
	}
	percentages[len(p.Steps)] = 100
	return percentages
}

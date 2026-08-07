package install


import "fmt"

// ============================================================================
// Service Management
// Handles enabling systemd services in the installed system.
// ============================================================================

// Service names to enable in the installed system.
// These are essential for a functional desktop environment:
//   - NetworkManager: network connectivity
//   - greetd: login manager / display manager session
//   - bluetooth: Bluetooth stack
var servicesToEnable = []string{
	"NetworkManager",
	"greetd",
	"bluetooth",
}

// EnableServices enables all essential systemd services in the installed system.
// Runs via arch-chroot as root.
// Returns early on first error — partial enabling is left for resume support.
func EnableServices(ctx *InstallContext) error {
	if err := validateMountedSystem(); err != nil {
		return err
	}

	for _, svc := range servicesToEnable {
		if ctx.Logger != nil {
			ctx.Logger.Info("Enabling service: %s", svc)
		}
		output, err := runChroot("systemctl", "enable", svc)
		if err != nil {
			return fmt.Errorf("failed to enable service %q: %w: %s", svc, err, string(output))
		}
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("All services enabled successfully")
	}

	return nil
}

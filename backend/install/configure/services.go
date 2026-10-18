package configure

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// zram swap configuration
// ============================================================================

// ConfigureZram sets up zram-based compressed swap in the installed system.
func ConfigureZram(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}
	swapMiB := calculateZramSizeMiB()
	zramConf := fmt.Sprintf(`[zram0]
zram-size = %d
compression-algorithm = zstd
`, swapMiB)
	if ctx.Logger != nil {
		ctx.Logger.Info("Configuring zram swap: %d MiB", swapMiB)
	}
	if err := primitives.WriteFile("/etc/systemd/zram-generator.conf", zramConf); err != nil {
		return fmt.Errorf("failed to write zram-generator config: %w", err)
	}
	return nil
}

var meminfoKBRegex = regexp.MustCompile(`^MemTotal:\s+(\d+)\s+kB$`)

const zramSizeMaxMiB = 8192

func calculateZramSizeMiB() int {
	data, err := primitives.ReadFile("/proc/meminfo")
	if err != nil {
		return 2048
	}
	for _, line := range strings.Split(string(data), "\n") {
		matches := meminfoKBRegex.FindStringSubmatch(line)
		if matches != nil {
			kb, err := strconv.ParseInt(matches[1], 10, 64)
			if err != nil {
				continue
			}
			giB := int(kb / 1048576)
			switch {
			case giB <= 4:
				return 2048
			case giB <= 8:
				return 4096
			default:
				return zramSizeMaxMiB
			}
		}
	}
	return 2048
}

// ============================================================================
// Service management
// ============================================================================

// requiredServices must be enabled for a usable installed system: without
// NetworkManager there is no network, and without greetd there is no login.
var requiredServices = []string{"NetworkManager", "greetd"}

// optionalServices add functionality but must not abort an otherwise complete
// install when their unit is missing or their enable fails.
var optionalServices = []string{"bluetooth"}

// EnableServices enables all essential systemd services in the installed system.
func EnableServices(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}
	for _, svc := range requiredServices {
		if ctx.Logger != nil {
			ctx.Logger.Info("Enabling service: %s", svc)
		}
		output, err := primitives.RunChroot("systemctl", "enable", svc)
		if err != nil {
			return fmt.Errorf("failed to enable service %q: %w: %s", svc, err, string(output))
		}
	}
	for _, svc := range optionalServices {
		if ctx.Logger != nil {
			ctx.Logger.Info("Enabling service: %s", svc)
		}
		output, err := primitives.RunChroot("systemctl", "enable", svc)
		if err != nil && ctx.Logger != nil {
			ctx.Logger.Error("Warning: failed to enable optional service %q (non-fatal): %v: %s", svc, err, strings.TrimSpace(string(output)))
		}
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("All services enabled successfully")
	}
	return nil
}

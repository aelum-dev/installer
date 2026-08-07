package install

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// zramSizeMaxMiB is the maximum zram swap size in MiB (8 GiB).
const zramSizeMaxMiB = 8192

// meminfoKBRegex extracts the numeric value from a /proc/meminfo line.
var meminfoKBRegex = regexp.MustCompile(`^MemTotal:\s+(\d+)\s+kB$`)

// ConfigureZram sets up zram-based compressed swap in the installed system.
// Uses zram-generator with a fixed size based on installed RAM and zstd compression.
// The generator creates and manages the zram device automatically at boot.
func ConfigureZram(ctx *InstallContext) error {
	if err := validateMountedSystem(); err != nil {
		return err
	}

	// Calculate swap size from live ISO RAM using tiered mapping.
	swapMiB := calculateZramSizeMiB()

	zramConf := fmt.Sprintf(`[zram0]
zram-size = %d
compression-algorithm = zstd
`, swapMiB)
	if ctx.Logger != nil {
		ctx.Logger.Info("Configuring zram swap: %d MiB", swapMiB)
	}

	if err := writeFile("/etc/systemd/zram-generator.conf", zramConf); err != nil {
		return fmt.Errorf("failed to write zram-generator config: %w", err)
	}

	return nil
}

// calculateZramSizeMiB reads /proc/meminfo on the live ISO and returns
// the recommended zram size in MiB using tiered mapping:
//
//	<=4 GiB RAM  → 2 GiB swap
//	>4 GiB ≤8 GiB RAM  → 4 GiB swap
//	>8 GiB ≤16 GiB RAM → 8 GiB swap
//	>16 GiB RAM       → 8 GiB swap (capped)
func calculateZramSizeMiB() int {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 2048 // fallback to 2 GiB if we can't read
	}
	for _, line := range strings.Split(string(data), "\n") {
		matches := meminfoKBRegex.FindStringSubmatch(line)
		if matches != nil {
			kb, err := strconv.ParseInt(matches[1], 10, 64)
			if err != nil {
				continue
			}
			// Convert kB to GiB (1 GiB = 1048576 kB)
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



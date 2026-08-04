package install

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
)

// zramSizeMaxMiB is the maximum zram swap size in MiB (16 GiB).
const zramSizeMaxMiB = 16384

// meminfoKBRegex extracts the numeric value from a /proc/meminfo line.
var meminfoKBRegex = regexp.MustCompile(`^MemTotal:\s+(\d+)\s+kB$`)

// ConfigureZram sets up zram-based compressed swap in the installed system.
// Uses zram-generator with zram-size = min(ram/2, 16384 MiB) and zstd compression.
// The generator creates and manages the zram device automatically at boot.
func ConfigureZram(ctx *InstallContext) error {
	if err := validateMountedSystem(ctx); err != nil {
		return err
	}

	// Calculate swap size from live ISO RAM.
	// Rule: min(RAM/2, 16GiB), expressed in MiB for zram-generator.
	swapMiB := calculateZramSizeMiB()

	zramConf := fmt.Sprintf(`[zram0]
zram-size = %d
compression-algorithm = zstd
`, swapMiB)
	if ctx.Logger != nil {
		ctx.Logger.Info("Configuring zram swap: %d MiB", swapMiB)
	}

	if err := writeFile(ctx, "/etc/systemd/zram-generator.conf", zramConf); err != nil {
		return fmt.Errorf("failed to write zram-generator config: %w", err)
	}

	return nil
}

// calculateZramSizeMiB reads /proc/meminfo on the live ISO and returns
// the recommended zram size in MiB: min(totalRAMMiB / 2, 16384).
func calculateZramSizeMiB() int {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return zramSizeMaxMiB // fallback to max if we can't read
	}
	for _, line := range splitLines(string(data)) {
		matches := meminfoKBRegex.FindStringSubmatch(line)
		if matches != nil {
			kb, err := strconv.ParseInt(matches[1], 10, 64)
			if err != nil {
				continue
			}
			miB := int(kb / 1024)
			swap := miB / 2
			if swap > zramSizeMaxMiB {
				swap = zramSizeMaxMiB
			}
			return swap
		}
	}
	return zramSizeMaxMiB
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}



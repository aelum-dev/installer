package install

import (
	"fmt"
)

// ConfigureZram sets up zram-based compressed swap in the installed system.
// Uses zram-generator with zram-size = min(ram/2, 16384 MiB) and zstd compression.
// The generator creates and manages the zram device automatically at boot.
func ConfigureZram(ctx *InstallContext) error {
	if err := validateMountedSystem(ctx); err != nil {
		return err
	}

	// Write zram-generator config.
	// zram-size = min(ram / 2, 16384) gives: 8GB RAM→4G, 16GB→8G, 32GB→16G cap
	zramConf := `[zram0]
zram-size = min(ram / 2, 16384)
compression-algorithm = zstd
`
	if err := writeFile(ctx, "/etc/systemd/zram-generator.conf", zramConf); err != nil {
		return fmt.Errorf("failed to write zram-generator config: %w", err)
	}

	return nil
}



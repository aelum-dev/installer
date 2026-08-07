package install

import (
	"fmt"
	"strings"

	"aestheticos-installer/backend/shared"
)

// validateMountedSystem checks that /mnt is properly mounted with a root partition.
// This is a prerequisite for steps that operate on the installed system.
func validateMountedSystem() error {
	// Check if /mnt is mounted
	cmd := Run("findmnt", "-n", "-o", "SOURCE", shared.MountRoot)
	output, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(output)) == "" {
		return fmt.Errorf("/mnt is not mounted. Run --mount-filesystem <disk> first to mount partitions")
	}

	return nil
}

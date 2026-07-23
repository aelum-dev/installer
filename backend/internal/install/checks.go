package install

import (
	"fmt"
	"os/exec"
	"strings"
)

// validateMountedSystem checks that /mnt is properly mounted with a root partition.
// This is a prerequisite for steps that operate on the installed system.
func validateMountedSystem(ctx *InstallContext) error {
	helper := ctx.Helper
	if helper == "" {
		return fmt.Errorf("helper path not set")
	}

	// Check if /mnt is mounted
	cmd := exec.Command("sudo", helper, "run", "findmnt", "-n", "-o", "SOURCE", "/mnt")
	output, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(output)) == "" {
		return fmt.Errorf("/mnt is not mounted. Run --mount-filesystem <disk> first to mount partitions")
	}

	return nil
}

// validatePartitionsExist checks that partition info is available (from FormatDisk).
func validatePartitionsExist(ctx *InstallContext) error {
	if ctx.Partitions == nil {
		return fmt.Errorf("partition info not available. Run --format-disk <disk> first to create partitions")
	}
	return nil
}

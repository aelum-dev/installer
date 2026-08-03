package install

import (
	"fmt"
	"os/exec"
	"strings"

	"aestheticos-installer/backend/internal/shared"
)

// validateMountedSystem checks that /mnt is properly mounted with a root partition.
// This is a prerequisite for steps that operate on the installed system.
func validateMountedSystem(ctx *InstallContext) error {
	helper := ctx.Helper
	if helper == "" {
		return fmt.Errorf("helper path not set")
	}

	// Check if /mnt is mounted
	cmd := exec.Command("sudo", helper, HelperCmdQueryFindmnt, shared.MountRoot)
	output, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(output)) == "" {
		return fmt.Errorf("/mnt is not mounted. Run --mount-filesystem <disk> first to mount partitions")
	}

	return nil
}



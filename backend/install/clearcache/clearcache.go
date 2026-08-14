package clearcache

import (
	"fmt"
	"strings"

	"aestheticos-installer/backend/install/primitives"
	"aestheticos-installer/backend/shared"
)

// Run clears the pacman package cache in the installed system.
func Run(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}

	output, err := primitives.RunChroot("pacman", "-Scc", "--noconfirm")
	if err != nil {
		return fmt.Errorf("failed to clear pacman cache: %w: %s", err, strings.TrimSpace(string(output)))
	}

	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	return nil
}

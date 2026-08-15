package clearcache

import (
	"os"

	"aestheticos-installer/backend/install/primitives"
	"aestheticos-installer/backend/shared"
)

// Run clears the pacman package cache in the installed system.
// This is a best-effort cleanup — failures are logged but never fatal.
func Run(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}

	// Remove cached packages directly to avoid pacman's interactive
	// confirmation prompt and exit code 1 when cache is empty.
	pkgDir := shared.MountRoot + "/var/cache/pacman/pkg"
	if _, err := os.Stat(pkgDir); err == nil {
		if err := os.RemoveAll(pkgDir); err != nil {
			// Best-effort: log the failure but don't abort installation.
			if ctx.Logger != nil {
				ctx.Logger.Error("cache clear failed (non-fatal): %v", err)
			}
			return nil
		}
	}

	return nil
}

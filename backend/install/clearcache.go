package install

import (
	"fmt"
	"strings"
)

// ClearCacheStep clears the pacman package cache in the installed system.
type ClearCacheStep struct{}

func (s *ClearCacheStep) Name() string  { return "clear_cache" }
func (s *ClearCacheStep) Label() string { return "Clearing package cache" }

// ClearCache clears the pacman package cache in the installed system.
func ClearCache(ctx *InstallContext) error {
	if err := validateMountedSystem(); err != nil {
		return err
	}

	output, err := runChroot("pacman", "-Scc", "--noconfirm")
	if err != nil {
		return fmt.Errorf("failed to clear pacman cache: %w: %s", err, strings.TrimSpace(string(output)))
	}

	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	return nil
}

func (s *ClearCacheStep) Run(ctx *InstallContext) error {
	return ClearCache(ctx)
}

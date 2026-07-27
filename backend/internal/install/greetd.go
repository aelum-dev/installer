package install

import (
	_ "embed"

	"fmt"
)

//go:embed data/greetd/config.toml
var greetdConfigToml string

// ConfigureGreetd sets up greetd as the login manager with the DMS greeter.
// Runs inside the installed system via arch-chroot as root.
// Steps:
//
//	1. Create greeter user and dirs via systemd-sysusers/tmpfiles
//	2. Write /etc/greetd/config.toml (avoids dms greeter enable root check)
//
// Note: greetd.service is enabled by EnableServices() in services.go.
// Note: The greetd-dms-greeter-bin package is installed during the desktop
// environment step. ISO /etc/skel is copied to /mnt/etc/skel in SetupUsers
// before useradd, so DMS Hyprland configs are automatically copied to the
// user's home.
func ConfigureGreetd(ctx *InstallContext) error {
	if err := validateMountedSystem(ctx); err != nil {
		return err
	}

	// Step 1: Create greeter user and directories
	// The package ships /usr/lib/sysusers.d/dms-greeter.conf to create the greeter user.
	// systemd-sysusers only runs at boot, so we trigger it manually after install.
	// Also run systemd-tmpfiles to create /var/lib/greeter (home dir) and /var/cache/dms-greeter.
	if ctx.Logger != nil {
		ctx.Logger.Info("Ensuring greeter user exists")
	}
	output, err := runChroot(ctx, "systemd-sysusers", "--root=/", "--no-pager")
	if err != nil {
		return fmt.Errorf("failed to create greeter user via systemd-sysusers: %w: %s", err, string(output))
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Creating greeter directories")
	}
	output, err = runChroot(ctx, "systemd-tmpfiles", "--create", "--root=/")
	if err != nil {
		return fmt.Errorf("failed to create greeter directories via systemd-tmpfiles: %w: %s", err, string(output))
	}

	// Ensure greeter user owns its directories (tmpfiles may not set ownership correctly)
	if ctx.Logger != nil {
		ctx.Logger.Info("Setting greeter directory ownership")
	}
	// Create directories first in case systemd-tmpfiles didn't (e.g. package not yet installed)
	for _, dir := range []string{"/var/cache/dms-greeter", "/var/lib/greeter"} {
		output, err = runChroot(ctx, "mkdir", "-p", dir)
		if err != nil {
			return fmt.Errorf("failed to create directory %s: %w: %s", dir, err, string(output))
		}
	}
	output, err = runChroot(ctx, "chown", "-R", "greeter:greeter", "/var/cache/dms-greeter", "/var/lib/greeter")
	if err != nil {
		return fmt.Errorf("failed to set greeter directory ownership: %w: %s", err, string(output))
	}

	// Step 2: Write /etc/greetd/config.toml from embedded file
	// This is what `dms greeter enable` does internally, but we do it as root
	// to avoid the "should not be run as root" check in the dms CLI.
	if ctx.Logger != nil {
		ctx.Logger.Info("Configuring greetd")
	}

	if err := writeFile(ctx, "/etc/greetd/config.toml", greetdConfigToml); err != nil {
		return err
	}

	return nil
}

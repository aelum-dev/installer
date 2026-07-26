package install

import (
	_ "embed"

	"fmt"
	"os/exec"
)

//go:embed data/greetd/config.toml
var greetdConfigToml string

// GreetdPackage is the greetd + DMS greeter package.
// Must be available in the offline repo (built from AUR).
const GreetdPackage = "greetd-dms-greeter-bin"

// ConfigureGreetd sets up greetd as the login manager with the DMS greeter.
// Runs inside the installed system via arch-chroot as root.
// Steps:
//  1. Install greetd packages via pacstrap
//  2. Create greeter user and dirs via systemd-sysusers/tmpfiles
//  3. Write /etc/greetd/config.toml (avoids dms greeter enable root check)
//  4. Enable greetd.service
//
// Note: DMS Hyprland configs are deployed via /etc/skel on the ISO,
// so they're automatically copied to the user's home when useradd -m runs.
func ConfigureGreetd(ctx *InstallContext) error {
	if err := validateMountedSystem(ctx); err != nil {
		return err
	}

	// Step 1: Install greetd packages via pacstrap
	if ctx.Logger != nil {
		ctx.Logger.Info("Installing greetd package: %s", GreetdPackage)
	}

	pacstrapArgs := []string{PacstrapMountPoint, GreetdPackage}
	cmd := exec.Command("sudo", append([]string{ctx.Helper, HelperCmdRun, "pacstrap"}, pacstrapArgs...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pacstrap failed for greetd packages: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	// Step 2: Create greeter user and directories
	// The package ships /usr/lib/sysusers.d/dms-greeter.conf to create the greeter user.
	// systemd-sysusers only runs at boot, so we trigger it manually after pacstrap.
	// Also run systemd-tmpfiles to create /var/lib/greeter (home dir) and /var/cache/dms-greeter.
	if ctx.Logger != nil {
		ctx.Logger.Info("Ensuring greeter user exists")
	}
	output, err = runChroot(ctx, "systemd-sysusers", "--root=/", "--no-pager")
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

	// Step 3: Write /etc/greetd/config.toml from embedded file
	// This is what `dms greeter enable` does internally, but we do it as root
	// to avoid the "should not be run as root" check in the dms CLI.
	if ctx.Logger != nil {
		ctx.Logger.Info("Configuring greetd")
	}

	if err := writeFile(ctx, "/etc/greetd/config.toml", greetdConfigToml); err != nil {
		return err
	}

	// Step 4: Enable greetd.service
	if ctx.Logger != nil {
		ctx.Logger.Info("Enabling greetd service")
	}
	output, err = runChroot(ctx, "systemctl", "enable", "greetd")
	if err != nil {
		return fmt.Errorf("failed to enable greetd.service: %w: %s", err, string(output))
	}

	return nil
}

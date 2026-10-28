// Package greetd provides greetd login manager configuration with Dux Greeter support.
package greetd

import (
	"fmt"

	"aelum-installer/backend/shared"
)

// Executor abstracts command execution and file operations for greetd setup.
// Implemented by the install package.
type Executor interface {
	// RunChroot executes a command inside the installed system via arch-chroot.
	RunChroot(args ...string) ([]byte, error)
	// WriteFile writes content to a file inside the installed system.
	WriteFile(path, content string) error
	// Logger returns the logger for this installation.
	Logger() shared.Logger
}

// InstallContext holds the data needed for greetd configuration.
type InstallContext struct {
	Username string
	Logger   shared.Logger
}

// ConfigureGreetd sets up greetd as the login manager with the Dux Greeter.
// Runs inside the installed system via arch-chroot as root.
func ConfigureGreetd(exec Executor, ctx *InstallContext) error {
	// Step 1: Create greeter user and directories
	output, err := exec.RunChroot("systemd-sysusers", "--root=/", "--no-pager")
	if err != nil {
		return fmt.Errorf("failed to create greeter user via systemd-sysusers: %w: %s", err, string(output))
	}

	output, err = exec.RunChroot("systemd-tmpfiles", "--create", "--root=/")
	if err != nil {
		return fmt.Errorf("failed to create greeter directories via systemd-tmpfiles: %w: %s", err, string(output))
	}

	for _, dir := range []string{"/var/cache/dms-greeter", "/var/lib/greeter"} {
		output, err = exec.RunChroot("mkdir", "-p", dir)
		if err != nil {
			return fmt.Errorf("failed to create directory %s: %w: %s", dir, err, string(output))
		}
	}
	output, err = exec.RunChroot("chown", "-R", "greeter:greeter", "/var/cache/dms-greeter", "/var/lib/greeter")
	if err != nil {
		return fmt.Errorf("failed to set greeter directory ownership: %w: %s", err, string(output))
	}

	for _, dir := range []string{"/var/cache/dms-greeter", "/var/lib/greeter"} {
		output, err = exec.RunChroot("chmod", "2770", dir)
		if err != nil {
			return fmt.Errorf("failed to set permissions on %s: %w: %s", dir, err, string(output))
		}
	}

	// Step 2: Write /etc/greetd/config.toml
	if err := exec.WriteFile("/etc/greetd/config.toml", ConfigToml()); err != nil {
		return err
	}

	// Step 3: Sync Dux Greeter.
	// Fail loudly if this errors: a silently broken greeter (missing ACLs, home
	// not ready, etc.) leaves the user at a login screen with no working theme,
	// which is worse than a clear install failure they can act on.
	if ctx.Username != "" {
		if err := SyncDmsGreeter(exec, ctx); err != nil {
			return fmt.Errorf("failed to sync Dux Greeter: %w", err)
		}
	}

	return nil
}

// Package greetd provides greetd login manager configuration with DMS greeter support.
package greetd

import "fmt"

// Executor abstracts command execution and file operations for greetd setup.
// Implemented by the install package.
type Executor interface {
	// RunChroot executes a command inside the installed system via arch-chroot.
	RunChroot(args ...string) ([]byte, error)
	// WriteFile writes content to a file inside the installed system.
	WriteFile(path, content string) error
	// Logger returns the logger for this installation.
	Logger() Logger
}

// Logger is the logging interface used by installation steps.
type Logger interface {
	Error(format string, args ...interface{})
}

// InstallContext holds the data needed for greetd configuration.
type InstallContext struct {
	Username string
	Logger   Logger
}

// ConfigureGreetd sets up greetd as the login manager with the DMS greeter.
// Runs inside the installed system via arch-chroot as root.
func ConfigureGreetd(exec Executor, ctx *InstallContext) error {
	// Ensure the greeter system group exists. systemd-sysusers below also
	// creates it, but we add it explicitly to match the packaged dms-greeter
	// behaviour and to be robust about package ordering.
	if _, err := exec.RunChroot("groupadd", "-r", "greeter"); err != nil {
		// Ignore errors (e.g. group already exists); systemd-sysusers ensures
		// the group as a fallback.
		ctx.Logger.Error("could not ensure greeter group exists (ignoring): %v", err)
	}

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

	// Step 3: Sync DMS greeter.
	// Fail loudly if this errors: a silently broken greeter (missing ACLs, home
	// not ready, etc.) leaves the user at a login screen with no working theme,
	// which is worse than a clear install failure they can act on.
	if ctx.Username != "" {
		if err := SyncDmsGreeter(exec, ctx); err != nil {
			return fmt.Errorf("failed to sync DMS greeter: %w", err)
		}
	}

	return nil
}

package install

import (
	_ "embed"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed data/greetd/config.toml
var greetdConfigToml string

//go:embed data/greetd/memory.json
var defaultMemoryJson string

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

	// Set 2770 permissions (setgid + rwxrwx---) on cache dirs per dms-greeter spec.
	// The setgid bit ensures new files inherit the greeter group.
	if ctx.Logger != nil {
		ctx.Logger.Info("Setting greeter directory permissions (2770)")
	}
	for _, dir := range []string{"/var/cache/dms-greeter", "/var/lib/greeter"} {
		output, err = runChroot(ctx, "chmod", "2770", dir)
		if err != nil {
			return fmt.Errorf("failed to set permissions on %s: %w: %s", dir, err, string(output))
		}
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

	// Step 2.5: Copy hypr.lua from live ISO to installed system
	// The live ISO has the greeter's Hyprland config at /etc/greetd/hypr.lua
	if ctx.Logger != nil {
		ctx.Logger.Info("Copying hypr.lua from live ISO")
	}
	cmd := exec.Command("sudo", ctx.Helper, HelperCmdRun, "bash", "-c", "cp /etc/greetd/hypr.lua /mnt/etc/greetd/hypr.lua")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy hypr.lua: %w: %s", err, string(output))
	}

	// Step 3: Sync DMS greeter with the user's config (wallpaper, theme, colors)
	// This replicates what `dms greeter sync` does, but runs as root during install
	// so the user sees their distro's default wallpaper immediately at first login.
	// The wallpaper and DMS configs are shipped in ISO /etc/skel and copied to the
	// user's home by useradd -m in SetupUsers.
	if ctx.Username != "" {
		if err := syncDmsGreeter(ctx); err != nil {
			// Log but don't fail the install — greeter sync is nice-to-have.
			// The greeter will still work, just without the themed wallpaper.
			if ctx.Logger != nil {
				ctx.Logger.Error("Failed to sync DMS greeter: %v (user can run 'dms greeter sync' later)", err)
			}
		}
	}

	return nil
}

// syncDmsGreeter sets up the greeter cache so the DMS greeter displays the
// user's wallpaper, theme, colors, and profile immediately at login.
//
// This replicates `dms greeter sync` (the full sync, not --profile) so it
// works as root during installation. The user's DMS config files are already
// in their home directory (copied from ISO /etc/skel by useradd -m).
//
// Steps:
//
//	1. Add user to the 'greeter' group
//	2. Set ACLs on parent dirs (~, .config, .local, .cache, .local/state)
//	3. Set group ownership on DMS config dirs
//	4. Create symlinks from /var/cache/dms-greeter/ → user's config files
//	5. Copy per-user slot to /var/cache/dms-greeter/users/<username>/
func syncDmsGreeter(ctx *InstallContext) error {
	username := ctx.Username
	home := filepath.Join("/home", username)

	// 1. Add user and greeter daemon to the greeter group
	// Group membership lets the greeter process read synced config files via
	// group ACLs (g:greeter:rX) on the user's home directories.
	if ctx.Logger != nil {
		ctx.Logger.Info("Adding user '%s' to greeter group", username)
	}
	output, err := runChroot(ctx, "usermod", "-aG", "greeter", username)
	if err != nil {
		return fmt.Errorf("failed to add %s to greeter group: %w: %s", username, err, string(output))
	}
	// Also add the greeter daemon user so the greeter process can read synced files
	if ctx.Logger != nil {
		ctx.Logger.Info("Adding greeter daemon to greeter group")
	}
	output, err = runChroot(ctx, "usermod", "-aG", "greeter", "greeter")
	if err != nil {
		return fmt.Errorf("failed to add greeter user to greeter group: %w: %s", err, string(output))
	}

	// 2. Set ACLs on parent directories so the greeter user can traverse and read
	// The greeter needs 'x' (execute/traverse) on parent dirs and 'r' on config files.
	// We use setfacl to grant the greeter user access without changing ownership.
	// First ensure the directory structure exists (useradd -m doesn't create .local etc.)
	if ctx.Logger != nil {
		ctx.Logger.Info("Setting ACLs for greeter on user home directories")
	}
	aclDirs := []string{home, filepath.Join(home, ".config"), filepath.Join(home, ".local"),
		filepath.Join(home, ".cache"), filepath.Join(home, ".local", "state"),
		filepath.Join(home, ".local", "share")}
	for _, dir := range aclDirs {
		// Create dir if it doesn't exist
		if _, err := runChroot(ctx, "mkdir", "-p", dir); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		// Use group-based ACL (g:greeter:rX) — covers the greeter daemon user
		// regardless of its username (e.g. greetd on Fedora vs greeter on Arch).
		output, err = runChroot(ctx, "setfacl", "-m", "g:greeter:rX", dir)
		if err != nil {
			return fmt.Errorf("failed to set ACL on %s: %w: %s", dir, err, string(output))
		}
	}

	// 3. Set group ownership on DMS config directories so greeter can read them
	// The DMS config dirs need group 'greeter' with read+execute permissions.
	if ctx.Logger != nil {
		ctx.Logger.Info("Setting DMS config group permissions")
	}
	dmsConfigDirs := []string{
		filepath.Join(home, ".config", "DankMaterialShell"),
		filepath.Join(home, ".local", "state", "DankMaterialShell"),
		filepath.Join(home, ".cache", "DankMaterialShell"),
		filepath.Join(home, ".local", "share", "color-schemes"),
	}
	for _, dir := range dmsConfigDirs {
		// chgrp -R greeter <dir> && chmod -R g+rX <dir>
		output, err = runChroot(ctx, "chgrp", "-R", "greeter", dir)
		if err != nil {
			// Check if dir actually exists before skipping
			if _, testErr := runChroot(ctx, "test", "-d", dir); testErr != nil {
				// Directory doesn't exist — that's ok, skip it
				if ctx.Logger != nil {
					ctx.Logger.Info("DMS config dir %s not found, skipping", dir)
				}
				continue
			}
			return fmt.Errorf("failed to chgrp %s: %w: %s", dir, err, string(output))
		}
		output, err = runChroot(ctx, "chmod", "-R", "g+rX", dir)
		if err != nil {
			return fmt.Errorf("failed to set group perms on %s: %w: %s", dir, err, string(output))
		}
	}

	// 4. Create greeter cache runtime subdirs
	// These are required by dms-greeter's EnsureGreeterCacheDir and used for:
	//   - .local/state/memory.json: auto-login state (last user, last session)
	//   - .local/share: runtime data
	//   - .cache: runtime cache
	if ctx.Logger != nil {
		ctx.Logger.Info("Creating greeter cache runtime subdirs")
	}
	cacheDir := "/var/cache/dms-greeter"
	runtimeSubdirs := []string{
		filepath.Join(cacheDir, ".local"),
		filepath.Join(cacheDir, ".local", "state"),
		filepath.Join(cacheDir, ".local", "share"),
		filepath.Join(cacheDir, ".cache"),
		filepath.Join(cacheDir, "users"),
	}
	for _, subdir := range runtimeSubdirs {
		output, err = runChroot(ctx, "mkdir", "-p", subdir)
		if err != nil {
			return fmt.Errorf("failed to create cache subdir %s: %w: %s", subdir, err, string(output))
		}
		// chown + chmod 2770 to match dms-greeter's EnsureGreeterCacheDir behavior
		output, err = runChroot(ctx, "chown", "greeter:greeter", subdir)
		if err != nil {
			return fmt.Errorf("failed to chown %s: %w: %s", subdir, err, string(output))
		}
		output, err = runChroot(ctx, "chmod", "2770", subdir)
		if err != nil {
			return fmt.Errorf("failed to chmod %s: %w: %s", subdir, err, string(output))
		}
	}

	// 6. Create symlinks from /var/cache/dms-greeter/ → user's DMS config files
	// Configs are shipped in ISO /etc/skel, so they already exist in user's home.
	// These symlinks let the greeter (running as greeter user) read them.
	if ctx.Logger != nil {
		ctx.Logger.Info("Creating greeter cache symlinks")
	}
	symlinks := map[string]string{
		filepath.Join(cacheDir, "settings.json"): filepath.Join(home, ".config", "DankMaterialShell", "settings.json"),
		filepath.Join(cacheDir, "session.json"):  filepath.Join(home, ".local", "state", "DankMaterialShell", "session.json"),
		filepath.Join(cacheDir, "colors.json"):   filepath.Join(home, ".cache", "DankMaterialShell", "dms-colors.json"),
	}
	for link, target := range symlinks {
		// Remove existing symlink/file first
		if _, err := runChroot(ctx, "rm", "-f", link); err != nil {
			return fmt.Errorf("failed to remove existing file %s: %w", link, err)
		}

		// Create the symlink
		output, err = runChroot(ctx, "ln", "-sf", target, link)
		if err != nil {
			return fmt.Errorf("failed to create symlink %s -> %s: %w: %s", link, target, err, string(output))
		}
	}

	// 7. Write memory.json for session memory (last user + last session)
	// This lets the greeter pre-select the user and session in the picker,
	// but does NOT enable auto-login (no autoLoginEnabled field).
	if ctx.Logger != nil {
		ctx.Logger.Info("Writing greeter memory.json")
	}
	stateMemoryPath := filepath.Join(cacheDir, ".local", "state", "memory.json")
	memoryContent := strings.ReplaceAll(defaultMemoryJson, "{{USERNAME}}", username)
	if err := writeFile(ctx, stateMemoryPath, memoryContent); err != nil {
		return fmt.Errorf("failed to write memory.json: %w", err)
	}
	output, err = runChroot(ctx, "chown", "greeter:greeter", stateMemoryPath)
	if err != nil {
		return fmt.Errorf("failed to chown memory.json: %w: %s", err, string(output))
	}
	output, err = runChroot(ctx, "chmod", "0664", stateMemoryPath)
	if err != nil {
		return fmt.Errorf("failed to chmod memory.json: %w: %s", err, string(output))
	}
	// Legacy compat symlink: memory.json -> .local/state/memory.json
	// Older dms-greeter versions stored memory.json at the cache root.
	legacyMemoryPath := filepath.Join(cacheDir, "memory.json")
	output, err = runChroot(ctx, "ln", "-sfn", stateMemoryPath, legacyMemoryPath)
	if err != nil {
		return fmt.Errorf("failed to create legacy memory symlink: %w: %s", err, string(output))
	}

	// 8. Copy per-user slot to /var/cache/dms-greeter/users/<username>/
	// This enables multi-account picker with the user's avatar and theme preview.
	if ctx.Logger != nil {
		ctx.Logger.Info("Setting up per-user greeter slot")
	}
	userSlot := filepath.Join(cacheDir, "users", username)
	output, err = runChroot(ctx, "mkdir", "-p", userSlot)
	if err != nil {
		return fmt.Errorf("failed to create user slot dir: %w: %s", err, string(output))
	}

	// Copy the user's DMS config files into their slot for the login picker
	userSlotFiles := []string{
		filepath.Join(home, ".config", "DankMaterialShell", "settings.json"),
		filepath.Join(home, ".local", "state", "DankMaterialShell", "session.json"),
		filepath.Join(home, ".cache", "DankMaterialShell", "dms-colors.json"),
	}
	for _, source := range userSlotFiles {
		filename := filepath.Base(source)
		_, err = runChroot(ctx, "cp", "-f", source, filepath.Join(userSlot, filename))
		if err != nil {
			if ctx.Logger != nil {
				ctx.Logger.Error("Failed to copy %s to user slot: %v", filename, err)
			}
		}
	}

	// Set ownership of the user slot to greeter
	output, err = runChroot(ctx, "chown", "-R", "greeter:greeter", userSlot)
	if err != nil {
		return fmt.Errorf("failed to set user slot ownership: %w: %s", err, string(output))
	}

	return nil
}

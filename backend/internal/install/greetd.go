package install

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"
)

//go:embed data/greetd/config.toml
var greetdConfigToml string

//go:embed data/greetd/session.json
var sessionJsonTemplate string

//go:embed data/greetd/settings.json
var defaultSettingsJson string

//go:embed data/greetd/dms-colors.json
var defaultColorsJson string

//go:embed data/greetd/wallpaper.jpg
var defaultWallpaperJpg string

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

	// 1. Add user to the greeter group
	if ctx.Logger != nil {
		ctx.Logger.Info("Adding user '%s' to greeter group", username)
	}
	output, err := runChroot(ctx, "usermod", "-aG", "greeter", username)
	if err != nil {
		return fmt.Errorf("failed to add %s to greeter group: %w: %s", username, err, string(output))
	}

	// 2. Set ACLs on parent directories so the greeter user can traverse and read
	// The greeter needs 'x' (execute/traverse) on parent dirs and 'r' on config files.
	// We use setfacl to grant the greeter user access without changing ownership.
	// First ensure the directory structure exists (useradd -m doesn't create .local etc.)
	if ctx.Logger != nil {
		ctx.Logger.Info("Setting ACLs for greeter on user home directories")
	}
	aclDirs := []string{home, filepath.Join(home, ".config"), filepath.Join(home, ".local"),
		filepath.Join(home, ".cache"), filepath.Join(home, ".local", "state")}
	for _, dir := range aclDirs {
		// Create dir if it doesn't exist
		if _, err := runChroot(ctx, "mkdir", "-p", dir); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		output, err = runChroot(ctx, "setfacl", "-m", fmt.Sprintf("u:greeter:rx"), dir)
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
		filepath.Join(home, ".cache", "quickshell"),
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

	// 4. Write embedded wallpaper to greeter cache
	// The wallpaper is embedded in the installer binary, not shipped in ISO skel.
	if ctx.Logger != nil {
		ctx.Logger.Info("Writing embedded wallpaper to greeter cache")
	}
	cacheDir := "/var/cache/dms-greeter"
	cacheWallpaperPath := filepath.Join(cacheDir, "wallpaper.jpg")

	// Write embedded wallpaper directly to cache
	if err := writeFile(ctx, cacheWallpaperPath, defaultWallpaperJpg); err != nil {
		return fmt.Errorf("failed to write wallpaper to cache: %w", err)
	}

	// Write session.json to user's home with the cache wallpaper path
	if ctx.Logger != nil {
		ctx.Logger.Info("Writing session.json to user home")
	}
	sessionContent := strings.ReplaceAll(sessionJsonTemplate, "{{WALLPAPER_PATH}}", cacheWallpaperPath)
	sessionPath := filepath.Join(home, ".local", "state", "DankMaterialShell", "session.json")
	if err := writeFile(ctx, sessionPath, sessionContent); err != nil {
		return fmt.Errorf("failed to write session.json: %w", err)
	}

	// Fix ownership: user must own their config files (not root)
	if ctx.Logger != nil {
		ctx.Logger.Info("Fixing ownership of user config files")
	}
	if err := fixUserConfigOwnership(ctx, username, home); err != nil {
		return err
	}

	// 5. Create runtime subdirs in cache (.local/state for memory.json, .local/share, .cache)
	// These are required by dms-greeter's EnsureGreeterCacheDir and used for:
	//   - .local/state/memory.json: auto-login state (last user, last session)
	//   - .local/share: runtime data
	//   - .cache: runtime cache
	if ctx.Logger != nil {
		ctx.Logger.Info("Creating greeter cache runtime subdirs")
	}
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
	// These are the main config files the greeter reads:
	//   settings.json  — theme, clock, weather, fonts
	//   session.json   — wallpaper configuration (symlink to user's home)
	//   colors.json    — color scheme (from DMS cache)
	// If source files don't exist, create empty {} (like dms-greeter sync does).
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

		// Create source file if it doesn't exist (use embedded defaults)
		output, err = runChroot(ctx, "test", "-f", target)
		if err != nil {
			// File doesn't exist, create from embedded default
			var defaultContent string
			switch target {
			case filepath.Join(home, ".config", "DankMaterialShell", "settings.json"):
				defaultContent = defaultSettingsJson
			case filepath.Join(home, ".cache", "DankMaterialShell", "dms-colors.json"):
				defaultContent = defaultColorsJson
			default:
				defaultContent = "{}"
			}
			if err := writeFile(ctx, target, defaultContent); err != nil {
				return fmt.Errorf("failed to create default file %s: %w", target, err)
			}
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
	memoryPath := filepath.Join(cacheDir, ".local", "state", "memory.json")
	memoryContent := strings.ReplaceAll(defaultMemoryJson, "{{USERNAME}}", username)
	if err := writeFile(ctx, memoryPath, memoryContent); err != nil {
		return fmt.Errorf("failed to write memory.json: %w", err)
	}
	output, err = runChroot(ctx, "chown", "greeter:greeter", memoryPath)
	if err != nil {
		return fmt.Errorf("failed to chown memory.json: %w: %s", err, string(output))
	}
	output, err = runChroot(ctx, "chmod", "0664", memoryPath)
	if err != nil {
		return fmt.Errorf("failed to chmod memory.json: %w: %s", err, string(output))
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

	// Copy the user's DMS config files into their slot
	// wallpaper.jpg is copied to cache root; also copy to user slot for picker
	userSlotFiles := map[string]string{
		"settings.json": filepath.Join(home, ".config", "DankMaterialShell", "settings.json"),
		"session.json":  sessionPath,
		"wallpaper.jpg": cacheWallpaperPath,
		"colors.json":   filepath.Join(home, ".cache", "DankMaterialShell", "dms-colors.json"),
	}
	for slotFile, source := range userSlotFiles {
		output, err = runChroot(ctx, "cp", "-f", source, filepath.Join(userSlot, slotFile))
		if err != nil {
			if ctx.Logger != nil {
				ctx.Logger.Error("Failed to copy %s to user slot: %v", slotFile, err)
			}
		}
	}

	// Also copy profile photo if it exists in skel
	for _, ext := range []string{"jpg", "jpeg", "png", "webp"} {
		photoSrc := filepath.Join(home, ".config", "DankMaterialShell", fmt.Sprintf("profile.%s", ext))
		photoDst := filepath.Join(userSlot, "profile.jpg")
		if _, err := runChroot(ctx, "test", "-f", photoSrc); err == nil {
			if _, err := runChroot(ctx, "cp", "-f", photoSrc, photoDst); err != nil {
				if ctx.Logger != nil {
					ctx.Logger.Error("Failed to copy profile photo: %v", err)
				}
			}
			break
		}
	}

	// Set ownership of the user slot to greeter
	output, err = runChroot(ctx, "chown", "-R", "greeter:greeter", userSlot)
	if err != nil {
		return fmt.Errorf("failed to set user slot ownership: %w: %s", err, string(output))
	}

	// Fix ownership of any config files created by writeFile in this function
	// (writeFile runs as root via tee; must chown to user after creation)
	if err := fixUserConfigOwnership(ctx, username, home); err != nil {
		return err
	}

	return nil
}

// fixUserConfigOwnership ensures the user owns their .config, .local, and .cache
// directories recursively. Used after writeFile calls (which run as root via tee)
// to transfer ownership to the target user.
func fixUserConfigOwnership(ctx *InstallContext, username, home string) error {
	dirs := []string{
		filepath.Join(home, ".config"),
		filepath.Join(home, ".local"),
		filepath.Join(home, ".cache"),
	}
	for _, dir := range dirs {
		output, err := runChroot(ctx, "chown", "-R", username+":"+username, dir)
		if err != nil {
			return fmt.Errorf("failed to chown %s to %s: %w: %s", dir, username, err, string(output))
		}
	}
	return nil
}

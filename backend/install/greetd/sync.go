package greetd

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SyncDmsGreeter sets up the greeter cache so the DMS greeter displays the
// user's wallpaper, theme, colors, and profile immediately at login.
func SyncDmsGreeter(exec Executor, ctx *InstallContext) error {
	username := ctx.Username
	home := filepath.Join("/home", username)

	// 1. Add user and greeter daemon to the greeter group
	output, err := exec.RunChroot("usermod", "-aG", "greeter", username)
	if err != nil {
		return fmt.Errorf("failed to add %s to greeter group: %w: %s", username, err, string(output))
	}
	output, err = exec.RunChroot("usermod", "-aG", "greeter", "greeter")
	if err != nil {
		return fmt.Errorf("failed to add greeter user to greeter group: %w: %s", err, string(output))
	}

	// 2. Set ACLs on parent directories
	aclDirs := []string{home, filepath.Join(home, ".config"), filepath.Join(home, ".local"),
		filepath.Join(home, ".cache"), filepath.Join(home, ".local", "state"),
		filepath.Join(home, ".local", "share")}
	for _, dir := range aclDirs {
		if _, err := exec.RunChroot("mkdir", "-p", dir); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		output, err = exec.RunChroot("setfacl", "-m", "g:greeter:rX", dir)
		if err != nil {
			return fmt.Errorf("failed to set ACL on %s: %w: %s", dir, err, string(output))
		}
	}

	// 3. Set group ownership on DMS config directories
	dmsConfigDirs := []string{
		filepath.Join(home, ".config", "DankMaterialShell"),
		filepath.Join(home, ".local", "state", "DankMaterialShell"),
		filepath.Join(home, ".cache", "DankMaterialShell"),
		filepath.Join(home, ".local", "share", "color-schemes"),
	}
	for _, dir := range dmsConfigDirs {
		output, err = exec.RunChroot("chgrp", "-R", "greeter", dir)
		if err != nil {
			if _, testErr := exec.RunChroot("test", "-d", dir); testErr != nil {
				continue
			}
			return fmt.Errorf("failed to chgrp %s: %w: %s", dir, err, string(output))
		}
		output, err = exec.RunChroot("chmod", "-R", "g+rX", dir)
		if err != nil {
			return fmt.Errorf("failed to set group perms on %s: %w: %s", dir, err, string(output))
		}
	}

	// 4. Create greeter cache runtime subdirs
	cacheDir := "/var/cache/dms-greeter"
	runtimeSubdirs := []string{
		filepath.Join(cacheDir, ".local"),
		filepath.Join(cacheDir, ".local", "state"),
		filepath.Join(cacheDir, ".local", "share"),
		filepath.Join(cacheDir, ".cache"),
		filepath.Join(cacheDir, "users"),
	}
	for _, subdir := range runtimeSubdirs {
		output, err = exec.RunChroot("mkdir", "-p", subdir)
		if err != nil {
			return fmt.Errorf("failed to create cache subdir %s: %w: %s", subdir, err, string(output))
		}
		output, err = exec.RunChroot("chown", "greeter:greeter", subdir)
		if err != nil {
			return fmt.Errorf("failed to chown %s: %w: %s", subdir, err, string(output))
		}
		output, err = exec.RunChroot("chmod", "2770", subdir)
		if err != nil {
			return fmt.Errorf("failed to chmod %s: %w: %s", subdir, err, string(output))
		}
	}

	// 5. Create per-user slot with symlinks to the user's live config files.
	// Using symlinks (not static copies) ensures the greeter always reads
	// up-to-date settings — including wallpaper changes made after install.
	// Each user gets their own slot so multi-user systems work correctly.
	userSlot := filepath.Join(cacheDir, "users", username)
	if _, err := exec.RunChroot("mkdir", "-p", userSlot); err != nil {
		return fmt.Errorf("failed to create user slot dir: %w", err)
	}

	userSlotSymlinks := map[string]string{
		filepath.Join(userSlot, "settings.json"): filepath.Join(home, ".config", "DankMaterialShell", "settings.json"),
		filepath.Join(userSlot, "session.json"):  filepath.Join(home, ".local", "state", "DankMaterialShell", "session.json"),
		filepath.Join(userSlot, "colors.json"):   filepath.Join(home, ".cache", "DankMaterialShell", "dms-colors.json"),
	}
	for link, target := range userSlotSymlinks {
		if _, err := exec.RunChroot("rm", "-f", link); err != nil {
			return fmt.Errorf("failed to remove existing file %s: %w", link, err)
		}
		output, err = exec.RunChroot("ln", "-sf", target, link)
		if err != nil {
			return fmt.Errorf("failed to create symlink %s -> %s: %w: %s", link, target, err, string(output))
		}
	}

	if _, err := exec.RunChroot("chown", "-h", "greeter:greeter", userSlot); err != nil {
		return fmt.Errorf("failed to set user slot ownership: %w", err)
	}

	// 6. Write memory.json to the root cache dir (shared across users).
	stateMemoryPath := filepath.Join(cacheDir, ".local", "state", "memory.json")
	memoryContent := strings.ReplaceAll(MemoryJSON(), "{{USERNAME}}", username)
	if err := exec.WriteFile(stateMemoryPath, memoryContent); err != nil {
		return fmt.Errorf("failed to write memory.json: %w", err)
	}
	output, err = exec.RunChroot("chown", "greeter:greeter", stateMemoryPath)
	if err != nil {
		return fmt.Errorf("failed to chown memory.json: %w: %s", err, string(output))
	}
	output, err = exec.RunChroot("chmod", "0664", stateMemoryPath)
	if err != nil {
		return fmt.Errorf("failed to chmod memory.json: %w: %s", err, string(output))
	}
	legacyMemoryPath := filepath.Join(cacheDir, "memory.json")
	output, err = exec.RunChroot("ln", "-sfn", stateMemoryPath, legacyMemoryPath)
	if err != nil {
		return fmt.Errorf("failed to create legacy memory symlink: %w: %s", err, string(output))
	}

	return nil
}

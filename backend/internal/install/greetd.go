package install

import (
	"embed"
	"fmt"
	"os/exec"
	"strings"

	"aestheticos-installer/backend/internal/shared"
)

//go:embed data/dms-hypr/*
var dmsHyprFiles embed.FS

//go:embed data/greetd/config.toml
var greetdConfigToml string

// GreetdPackages lists the packages needed for greetd + DMS greeter.
// greetd-dms-greeter-bin must be available in the offline repo (built from AUR).
var GreetdPackages = []string{
	"greetd-dms-greeter-bin",
}

// ConfigureGreetd sets up greetd as the login manager with the DMS greeter.
// Runs inside the installed system via arch-chroot as root.
// Steps:
//  1. Install greetd packages via pacstrap
//  2. Ensure greeter user exists via systemd-sysusers
//  3. Deploy DMS Hyprland configs to user's home directory
//  4. Write /etc/greetd/config.toml manually (avoids dms greeter enable root check)
//  5. Disable conflicting display managers
//  6. Enable greetd.service
func ConfigureGreetd(ctx *InstallContext) error {
	if err := validateMountedSystem(ctx); err != nil {
		return err
	}

	helper := ctx.Helper

	// Step 1: Install greetd packages via pacstrap
	if ctx.Logger != nil {
		ctx.Logger.Info("Installing greetd packages: %s", strings.Join(GreetdPackages, ", "))
	}

	pacstrapArgs := []string{PacstrapMountPoint}
	pacstrapArgs = append(pacstrapArgs, GreetdPackages...)
	cmd := exec.Command("sudo", append([]string{helper, HelperCmdRun, "pacstrap"}, pacstrapArgs...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pacstrap failed for greetd packages: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	// Step 2: Ensure greeter user exists (created by systemd-sysusers from package's sysusers.conf)
	// The package ships /usr/lib/sysusers.d/dms-greeter.conf but sysusers only runs at boot.
	// We must trigger it manually after pacstrap.
	if ctx.Logger != nil {
		ctx.Logger.Info("Ensuring greeter user exists")
	}
	cmd = exec.Command("sudo", helper, HelperCmdRun, ChrootCommand, shared.MountRoot,
		"systemd-sysusers", "--root=/", "--no-pager")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create greeter user via systemd-sysusers: %w: %s", err, string(output))
	}

	// Also create the greeter cache directories via tmpfiles
	cmd = exec.Command("sudo", helper, HelperCmdRun, ChrootCommand, shared.MountRoot,
		"systemd-tmpfiles", "--create", "--root=/")
	output, err = cmd.CombinedOutput()
	if err != nil {
		if ctx.Logger != nil {
			ctx.Logger.Info("Note: systemd-tmpfiles failed (non-fatal): %v", err)
		}
	}

	// Step 3: Deploy DMS Hyprland configs to user's home directory
	// hyprland.lua goes in ~/.config/hypr/ (main config)
	// DMS modules go in ~/.config/hypr/dms/ (theme, keybinds, window rules, etc.)
	if ctx.Logger != nil {
		ctx.Logger.Info("Deploying DMS Hyprland configs")
	}

	// Create directories
	cmd = exec.Command("sudo", helper, HelperCmdRun, ChrootCommand, shared.MountRoot,
		"bash", "-c", fmt.Sprintf(`mkdir -p /home/%s/.config/hypr/dms`, ctx.Username))
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create hypr config directories: %w: %s", err, string(output))
	}

	// Embed and copy each DMS Hyprland Lua file
	dmsFiles := []string{
		"colors.lua",
		"outputs.lua",
		"layout.lua",
		"cursor.lua",
		"binds.lua",
		"binds-user.lua",
		"windowrules.lua",
	}
	for _, filename := range dmsFiles {
		content, err := dmsHyprFiles.ReadFile("data/dms-hypr/" + filename)
		if err != nil {
			return fmt.Errorf("failed to read embedded file %s: %w", filename, err)
		}

		// Write DMS module files to ~/.config/hypr/dms/
		cmd = exec.Command("sudo", helper, HelperCmdRun, ChrootCommand, shared.MountRoot,
			"bash", "-c", fmt.Sprintf(`cat > /home/%s/.config/hypr/dms/%s << 'DMS_EOF'
%s
DMS_EOF`, ctx.Username, filename, string(content)))
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to write %s: %w: %s", filename, err, string(output))
		}
	}

	// Write main hyprland.lua to ~/.config/hypr/ (parent directory)
	content, err := dmsHyprFiles.ReadFile("data/dms-hypr/hyprland.lua")
	if err != nil {
		return fmt.Errorf("failed to read embedded file hyprland.lua: %w", err)
	}
	cmd = exec.Command("sudo", helper, HelperCmdRun, ChrootCommand, shared.MountRoot,
		"bash", "-c", fmt.Sprintf(`cat > /home/%s/.config/hypr/hyprland.lua << 'DMS_EOF'
%s
DMS_EOF`, ctx.Username, string(content)))
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to write hyprland.lua: %w: %s", err, string(output))
	}

	// Step 4: Write /etc/greetd/config.toml from embedded file
	// This is what `dms greeter enable` does internally, but we do it as root
	// to avoid the "should not be run as root" check in the dms CLI.
	if ctx.Logger != nil {
		ctx.Logger.Info("Configuring greetd")
	}
	cmd = exec.Command("sudo", helper, HelperCmdRun, ChrootCommand, shared.MountRoot,
		"bash", "-c", fmt.Sprintf(`mkdir -p /etc/greetd && cat > /etc/greetd/config.toml << 'CONFIGEOF'
%s
CONFIGEOF`, strings.TrimRight(greetdConfigToml, "\n")))
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to configure greetd: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	// Step 5: Disable conflicting display managers
	if ctx.Logger != nil {
		ctx.Logger.Info("Disabling conflicting display managers")
	}
	cmd = exec.Command("sudo", helper, HelperCmdRun, ChrootCommand, shared.MountRoot,
		"systemctl", "disable", "gdm", "lightdm", "sddm", "--quiet")
	output, err = cmd.CombinedOutput()
	if err != nil {
		// Non-fatal: these services may not exist on a minimal install
		if ctx.Logger != nil {
			ctx.Logger.Info("Note: some display managers may not be present: %v", err)
		}
	}

	// Step 7: Enable greetd.service
	if ctx.Logger != nil {
		ctx.Logger.Info("Enabling greetd service")
	}
	cmd = exec.Command("sudo", helper, HelperCmdRun, ChrootCommand, shared.MountRoot,
		"systemctl", "enable", "greetd")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to enable greetd.service: %w: %s", err, string(output))
	}

	return nil
}

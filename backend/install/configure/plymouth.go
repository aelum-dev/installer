package configure

import (
	"fmt"
	"path/filepath"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// Plymouth configuration
// ============================================================================

// aelumThemeISOPath is the Aelum LUKS unlock Plymouth theme as shipped on the
// live ISO. The installer copies it verbatim into the installed system so the
// early-boot (dm-crypt unlock) splash matches the live ISO exactly.
const aelumThemeISOPath = "/usr/share/plymouth/themes/aelum"

// ConfigurePlymouth sets up the Aelum LUKS unlock Plymouth theme for the
// installed system, matching the live ISO. It copies the branded theme files
// from the live ISO, points plymouthd.conf at the aelum theme, creates the
// default->aelum symlink, and regenerates the initramfs so the theme is baked
// in for early-boot unlock.
func ConfigurePlymouth(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}

	// 1. Copy the Aelum LUKS unlock theme from the live ISO into the target.
	if err := installAelumTheme(ctx); err != nil {
		return err
	}

	// 2. Point plymouthd.conf at the aelum theme. This mirrors the live ISO;
	//    UseFirmwareBackground=false keeps the dark boot background.
	plymouthdConf := `[Daemon]
Theme=aelum
ShowDelay=0
UseFirmwareBackground=false
# DeviceScale=1  # Uncomment and adjust for HiDPI
`
	if err := primitives.WriteFile("/etc/plymouth/plymouthd.conf", plymouthdConf); err != nil {
		return err
	}

	// 3. Create the default->aelum symlink. plymouth-set-default-theme does not
	//    manage the no-extension symlink the live ISO ships, so create it here.
	dstLink := shared.MountRoot + "/usr/share/plymouth/themes/default"
	if err := primitives.Run("rm", "-f", dstLink).Run(); err != nil {
		return fmt.Errorf("failed to clear existing plymouth default symlink: %w", err)
	}
	if err := primitives.Run("ln", "-s", "aelum", dstLink).Run(); err != nil {
		return fmt.Errorf("failed to create plymouth default symlink: %w", err)
	}

	// 4. Regenerate the initramfs so plymouth bakes the aelum theme in for
	//    early-boot. plymouth-set-default-theme only rewrites the Theme line in
	//    place, so the keys written in step 2 are preserved.
	output, err := primitives.RunChroot("plymouth-set-default-theme", "-R", "aelum")
	if err != nil {
		return fmt.Errorf("failed to set Plymouth aelum theme: %w: %s", err, string(output))
	}

	if ctx.Username != "" {
		if err := createHushlogin(ctx.Username); err != nil {
			if ctx.Logger != nil {
				ctx.Logger.Error("Warning: failed to create .hushlogin: %v", err)
			}
		}
	}
	return nil
}

// installAelumTheme copies the Aelum LUKS unlock Plymouth theme from the live
// ISO into the installed system. It fails loudly if the theme is missing from
// the ISO: the early-boot unlock splash depends on these files.
func installAelumTheme(ctx *shared.InstallContext) error {
	dstDir := shared.MountRoot + "/usr/share/plymouth/themes/aelum"

	if err := primitives.Run("test", "-d", aelumThemeISOPath).Run(); err != nil {
		return fmt.Errorf("cannot configure early-boot unlock splash: Aelum plymouth theme not found on ISO at %s (rebuild the ISO with the theme)", aelumThemeISOPath)
	}

	if err := primitives.Run("mkdir", "-p", dstDir).Run(); err != nil {
		return fmt.Errorf("failed to create plymouth theme dir: %w", err)
	}
	// Copy the theme contents (the .plymouth/.script and PNG assets).
	out, err := primitives.Run("cp", "-a", aelumThemeISOPath+"/.", dstDir+"/").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy Aelum plymouth theme from ISO: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("Installed Aelum LUKS unlock Plymouth theme")
	}
	return nil
}

// createHushlogin creates the user's .hushlogin so the first login shell
// stays quiet (no motd/banner) after the unlock splash.
func createHushlogin(username string) error {
	home := shared.MountRoot + filepath.Join("/home", username)
	cmd := primitives.Run("touch", filepath.Join(home, ".hushlogin"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create .hushlogin: %w: %s", err, string(output))
	}
	_, err = primitives.RunChroot("chown", username+":"+username, filepath.Join("/home", username, ".hushlogin"))
	if err != nil {
		return fmt.Errorf("failed to set .hushlogin ownership: %w", err)
	}
	return nil
}

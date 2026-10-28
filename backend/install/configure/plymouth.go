package configure

import (
	"fmt"
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

// aelumBootProgressUnit is the installed-system twin of the live ISO's
// aelum-live-progress.service. It sends the theme's final milestone right
// before plymouth-quit dismisses the splash, so the installed-system bar
// finishes at 100% instead of stalling wherever the stopwatch happened to be
// when the greeter came up.
const aelumBootProgressUnit = `# SPDX-License-Identifier: GPL-3.0
#
# Last installed-system boot milestone: push Aelum's Plymouth theme to 100%
# just before plymouth-quit dismisses the splash.
#
# Plymouth ships two units as static symlinks into multi-user.target.wants:
#
#   plymouth-quit.service       runs "plymouth quit"  -> dismisses the splash
#   plymouth-quit-wait.service  runs "plymouth --wait" -> holds boot until quit
#
# Neither is ordered against this unit, so pin it into the last window in which
# the splash is still on screen: after systemd-user-sessions, before BOTH quit
# units -- exactly as the live ISO's aelum-live-progress.service does. The
# theme's timed bar cannot otherwise know when boot ends, so without this it
# stops short of 100%.

[Unit]
Description=Aelum boot progress: finish the Plymouth bar before the greeter
After=plymouth-start.service systemd-user-sessions.service
Before=plymouth-quit.service plymouth-quit-wait.service

[Service]
Type=oneshot
ExecStart=-/usr/bin/plymouth update --status=boot:complete

# The status above only puts a message on plymouthd's socket; the theme still
# has to be redrawn for the 100% frame to exist on screen. plymouth-quit.service
# is ordered After= this unit, so holding here keeps the splash up long enough
# for that frame to be composited. The theme runs Plymouth.SetRefreshRate(50),
# i.e. one frame every 20 ms, so 0.1 s is five frames of headroom -- without it
# the two run a couple of frames apart and the bar can stop just short of 100%.
ExecStartPost=/usr/bin/sleep 0.1

[Install]
WantedBy=multi-user.target
`

// bootProgressUnitPath is where the unit is written inside the installed system.
const bootProgressUnitPath = "/etc/systemd/system/aelum-boot-progress.service"

// ConfigurePlymouth sets up the Aelum LUKS unlock Plymouth theme for the
// installed system, matching the live ISO. It copies the branded theme files
// from the live ISO, points plymouthd.conf at the aelum theme, creates the
// default->aelum symlink, regenerates the initramfs so the theme is baked in
// for early-boot unlock, and installs aelum-boot-progress.service so the boot
// bar finishes at 100% right before the greeter.
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

	// 5. Install + enable the bootstrap unit that finishes the splash bar at
	//    100% right before the greeter (the live ISO does the same with
	//    aelum-live-progress.service).
	if err := installBootProgressUnit(ctx); err != nil {
		return err
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

// installBootProgressUnit writes and enables aelum-boot-progress.service in the
// installed system. It is the installed-system equivalent of the live ISO's
// aelum-live-progress.service: a single oneshot that tells the aelum Plymouth
// theme to fill its bar to 100% in the last moment before plymouth-quit
// dismisses the splash. Best-effort: a missing plymouth binary or an
// enable failure must not fail the whole install (the theme's timed bar still
// works, just without the guaranteed finish).
func installBootProgressUnit(ctx *shared.InstallContext) error {
	if err := primitives.WriteFile(bootProgressUnitPath, aelumBootProgressUnit); err != nil {
		return fmt.Errorf("failed to write boot progress unit: %w", err)
	}

	output, err := primitives.RunChroot("systemctl", "enable", "aelum-boot-progress.service")
	if err != nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("Warning: failed to enable aelum-boot-progress.service: %v: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("Installed aelum-boot-progress.service (finishes the boot splash bar)")
	}
	return nil
}

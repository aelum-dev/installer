package install

import (
	"fmt"
	"path/filepath"

	"aestheticos-installer/backend/internal/shared"
)

// ConfigurePlymouth sets up Plymouth BGRT theme for silent boot in the installed system.
// Runs inside the installed system via arch-chroot as root.
//
// Steps:
//  1. Write /etc/plymouth/plymouthd.conf (Theme=bgrt, UseFirmwareBackground=true)
//  2. Run plymouth-set-default-theme -R bgrt (regenerates initramfs)
//  4. Enable plymouth-wait-for-animation.service (for fast boots)
//  5. Create .hushlogin for the user (suppresses "Last login" message)
func ConfigurePlymouth(ctx *InstallContext) error {
	if err := validateMountedSystem(ctx); err != nil {
		return err
	}

	// Step 1: Write /etc/plymouth/plymouthd.conf
	plymouthdConf := `[Daemon]
Theme=bgrt
ShowDelay=0
UseFirmwareBackground=true
`
	if err := writeFile(ctx, "/etc/plymouth/plymouthd.conf", plymouthdConf); err != nil {
		return err
	}

	// Step 2: Set BGRT theme and regenerate initramfs
	output, err := runChroot(ctx, "plymouth-set-default-theme", "-R", "bgrt")
	if err != nil {
		return fmt.Errorf("failed to set Plymouth BGRT theme: %w: %s", err, string(output))
	}

	// Step 4: Enable plymouth-wait-for-animation.service (for fast boots)
	_, err = runChroot(ctx, "systemctl", "enable", "plymouth-wait-for-animation.service")
	if err != nil {
		// Non-fatal — animation delay is optional
		if ctx.Logger != nil {
			ctx.Logger.Error("Warning: failed to enable plymouth-wait-for-animation: %v", err)
		}
	}

	// Step 5: Create .hushlogin for the user
	if ctx.Username != "" {
		if err := createHushlogin(ctx, ctx.Username); err != nil {
			// Non-fatal — .hushlogin is nice-to-have
			if ctx.Logger != nil {
				ctx.Logger.Error("Warning: failed to create .hushlogin: %v", err)
			}
		}
	}

	return nil
}

// createHushlogin creates a .hushlogin file in the user's home directory
// to suppress the "Last login" message on TTY login.
func createHushlogin(ctx *InstallContext, username string) error {
	home := shared.MountRoot + filepath.Join("/home", username)

	// Create the file (empty, just needs to exist)
	cmd := ctx.run(HelperCmdRun, "touch", filepath.Join(home, ".hushlogin"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create .hushlogin: %w: %s", err, string(output))
	}

	// Set correct ownership (must run inside chroot since user may not exist on host)
	_, err = runChroot(ctx, "chown", username+":"+username, filepath.Join("/home", username, ".hushlogin"))
	if err != nil {
		return fmt.Errorf("failed to set .hushlogin ownership: %w", err)
	}

	return nil
}

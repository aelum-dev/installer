package configure

import (
	"aelum-installer/backend/shared"
)

// ============================================================================
// Configuration orchestrator
// ============================================================================

// Run merges the former separate configuration steps into a single
// "Configuring system" step. It runs in order:
//
//  1. Generate fstab
//  2. Configure bootloader
//  3. System configuration (hostname, timezone, locale, keymap, NTP, mirrors)
//  4. greetd + DMS greeter setup
//  5. Aelum LUKS unlock Plymouth theme (matches the live ISO)
//  6. zram swap configuration
//  7. Enable systemd services
func Run(ctx *shared.InstallContext) error {
	if err := GenerateFstab(ctx); err != nil {
		return err
	}
	if err := ConfigureBootloader(ctx); err != nil {
		return err
	}
	if err := ConfigureSystemBase(ctx); err != nil {
		return err
	}
	// Configure the initramfs HOOKS: plymouth for the boot splash (always, since
	// ConfigurePlymouth runs unconditionally), and, when LUKS is enabled, the
	// sd-encrypt hook so the initramfs unlocks the root before mounting it. This
	// runs before greetd/plymouth so the later plymouth regen preserves the hook.
	if err := configureInitramfs(ctx); err != nil {
		return err
	}
	if err := ConfigureGreetd(ctx); err != nil {
		return err
	}
	if err := ConfigurePlymouth(ctx); err != nil {
		return err
	}
	if err := ConfigureZram(ctx); err != nil {
		return err
	}
	return EnableServices(ctx)
}

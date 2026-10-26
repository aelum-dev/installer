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
//  2. System configuration (hostname, timezone, locale, keymap, NTP, mirrors)
//  3. Configure the initramfs HOOKS and regenerate the initramfs images
//  4. Configure bootloader (must run after the initramfs exists, since the boot
//     entry references the kernel + initrd it produces)
//  5. greetd + DMS greeter setup
//  6. Aelum LUKS unlock Plymouth theme (matches the live ISO)
//  7. zram swap configuration
//  8. Snapper configs for the btrfs layout (btrfs only)
//  9. Enable systemd services
func Run(ctx *shared.InstallContext) error {
	if err := GenerateFstab(ctx); err != nil {
		return err
	}
	if err := ConfigureSystemBase(ctx); err != nil {
		return err
	}
	// Configure the initramfs HOOKS: plymouth for the boot splash (always, since
	// ConfigurePlymouth runs unconditionally), and, when LUKS is enabled, the
	// sd-encrypt hook so the initramfs unlocks the root before mounting it. This
	// runs before greetd/plymouth so the later plymouth regen preserves the hook.
	// It also runs before the bootloader so the entry it writes always references
	// an initrd that has already been generated.
	if err := configureInitramfs(ctx); err != nil {
		return err
	}
	if err := ConfigureBootloader(ctx); err != nil {
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
	if err := ConfigureSnapper(ctx); err != nil {
		return err
	}
	return EnableServices(ctx)
}

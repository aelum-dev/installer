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
//  8. Point /etc/resolv.conf at /run (before the services and before the
//     snapshot, so the factory snapshot already carries the link)
//  9. Enable systemd services
//  10. Snapper for the btrfs layout (btrfs only): configs, pacman transaction
//     hooks, the boot-menu entries for the snapshots, the state mounts a snapshot
//     boot needs, and the "after installation" snapshot - last, because it is the
//     finished system and because a snapshot only contains what was written before
//     it was taken
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
	// Before the services are enabled and before the snapshot is taken: the point
	// is that the factory snapshot — the state a broken system gets restored to —
	// already carries the link, so a snapshot boot resolves on the network it is
	// running on rather than the one its root was installed on.
	if err := ConfigureNetworkDNS(ctx); err != nil {
		return err
	}
	if err := EnableServices(ctx); err != nil {
		return err
	}
	// Last, and it has to stay last: the "after installation" snapshot is the state
	// a user restores to, so anything enabled after it is missing from it. Measured
	// on a real install where this ran before the services were enabled: the factory
	// snapshot had no display-manager.service and no NetworkManager, and booting it
	// came up at a text login instead of the greeter the install itself boots into.
	return ConfigureSnapper(ctx)
}

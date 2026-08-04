package install

// ConfigureSystem merges the former separate configuration steps into a single
// "Configuring system" step. It runs in order:
//
//	1. Generate fstab
//	2. Configure bootloader
//	3. System configuration (hostname, timezone, locale, keymap, NTP, mirrors)
//	4. greetd + DMS greeter setup
//	5. Plymouth BGRT silent boot
//	6. zram swap configuration
//	7. Enable systemd services
func ConfigureSystem(ctx *InstallContext) error {
	if err := GenerateFstab(ctx); err != nil {
		return err
	}
	if err := ConfigureBootloader(ctx); err != nil {
		return err
	}
	if err := ConfigureSystemBase(ctx); err != nil {
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

package install

// basePackages are the essential system packages installed during the base
// system step. Kept minimal so this step finishes quickly and the progress
// bar moves smoothly.
var basePackages = []string{
	// Base system
	"base",
	"linux-lts",
	"linux-firmware",
	"mkinitcpio",
	"arch-install-scripts",
	"mkinitcpio-archiso",
	"qemu-guest-agent",
	"edk2-shell",
	"memtest86+-efi",

	// Utilities
	"util-linux",
	"gptfdisk",
	"parted",
	"dosfstools",
	"e2fsprogs",
	"btrfs-progs",
	"pv",

	// System
	"sudo",
	"networkmanager",
	"bluez",

	// Editor
	"vim",
	"nano",
}

// Install installs the Arch Linux base system via pacstrap.
func Install(ctx *InstallContext) error {
	return installPackages(ctx, basePackages, "base system")
}

package installdesktop

import (
	"aelum-installer/backend/install/installbase"
	"aelum-installer/backend/shared"
)

// desktopPackages are user-space packages installed during the desktop
// environment step. Split from basePackages so the install progress bar
// moves smoothly instead of stalling during the long pacstrap call.
var DesktopPackages = []string{
	// Boot splash
	"plymouth",

	// Login manager
	"greetd",
	"greetd-agreety",
	"dux-greeter-bin",
	"uwsm",

	// Desktop environment
	"hyprland",
	"dux-shell",
	"cava",
	"dux-gop",
	"matugen",
	"qtengine",
	"nautilus",
	"firefox",
	"mpv",
	"imv",
	"gnome-text-editor",

	// Terminal & UI
	"kitty",
	"nautilus-open-any-terminal",
	"adwaita-cursors",
	"adw-gtk-theme",
	"fluent-icon-theme",
	"breeze",

	// Utilities
	"btop",
	"fastfetch",
	"brightnessctl",
	"ddcutil",

	// Fonts
	"noto-fonts",
	"noto-fonts-cjk",
	"noto-fonts-emoji",
	"terminus-font",
	"ttf-jetbrains-mono",
	"ttf-jetbrains-mono-nerd",

	// GPU drivers
	"vulkan-radeon",
	"vulkan-intel",
	"vulkan-nouveau",

	// Audio
	"pipewire",
	"pipewire-pulse",
	"pipewire-alsa",
	"pipewire-jack",
	"wireplumber",
}

// BtrfsOnlyDesktopPackages extend the desktop set only when the install chose
// btrfs. btrfs-assistant is the front-end for that layout: subvolumes, snapper
// snapshots, scrub and balance. On ext4 there is no layout for it to manage, and
// snapper is not installed either (see installbase.btrfsOnlyBasePackages).
var BtrfsOnlyDesktopPackages = []string{
	"btrfs-assistant",
}

// Run installs the desktop environment packages via pacstrap.
func Run(ctx *shared.InstallContext) error {
	packages := installbase.AppendBtrfsOnly(ctx, DesktopPackages, BtrfsOnlyDesktopPackages)
	return installbase.InstallPackages(ctx, packages, "desktop environment")
}

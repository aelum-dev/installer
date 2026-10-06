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
	"greetd-dms-greeter-bin",
	"uwsm",

	// Desktop environment
	"hyprland",
	"dms-shell",
	"cava",
	"dgop",
	"matugen",
	"qtengine",
	"nautilus",
	"firefox",
	"mpv",
	"imv",
	"gnome-text-editor",

	// Terminal & UI
	"kitty",
	"adwaita-cursors",
	"webkit2gtk-4.1",
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

// Run installs desktop environment packages via pacstrap.
func Run(ctx *shared.InstallContext) error {
	return installbase.InstallPackages(ctx, DesktopPackages, "desktop environment")
}

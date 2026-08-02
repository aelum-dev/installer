package install

// desktopPackages are user-space packages installed during the desktop
// environment step. Split from basePackages so the install progress bar
// moves smoothly instead of stalling during the long pacstrap call.
var desktopPackages = []string{
	// Login manager
	"greetd",
	"greetd-agreety",
	"greetd-dms-greeter-git",
	"uwsm",

	// Desktop environment
	"hyprland",
	"quickshell",
	"dms-shell-hyprland",
	"cava",
	"dgop",
	"matugen",
	"qt6ct-kde",
	"nautilus",
	"firefox",
	"mpv",
	"imv",
	"gnome-text-editor",

	// Terminal & UI
	"alacritty",
	"adwaita-cursors",
	"webkit2gtk-4.1",
	"adw-gtk-theme",
	"fluent-icon-theme-git",
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

// InstallDesktop installs desktop environment packages via pacstrap.
func InstallDesktop(ctx *InstallContext) error {
	return installPackages(ctx, desktopPackages, "desktop environment")
}

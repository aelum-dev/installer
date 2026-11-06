package installdesktop

import (
	"aelum-installer/backend/install/installbase"
	"aelum-installer/backend/shared"
)

// DesktopPackages are user-space packages installed during the desktop
// environment step. Split from basePackages so the install progress bar
// moves smoothly instead of stalling during the long pacstrap call.
//
// This list is also read by the ISO build (docker/scripts/build-iso.sh runs
// `go run ./backend/scripts/pkg-list.go`): the offline mirror embedded in the
// image is the union of these lists and the live profile's package list, so a
// package added here is installable without network by every install of that
// image. It is deliberately not repeated in live-iso/packages.x86_64 — the
// live session is a host that boots, displays and installs, not a copy of the
// desktop it writes.
var DesktopPackages = []string{
	// Boot splash
	"plymouth",

	// Login manager
	"greetd",
	"greetd-agreety",
	"dux-greeter-bin",
	"uwsm",

	// Desktop environment. dux-shell is DMS 2.0 (renamed from dms-shell) and
	// dux-gop is Dux GOP (renamed from dgop); both Provides/Replaces/Conflicts
	// the old names, so an existing install upgrades in place.
	"hyprland",
	"dux-shell",
	"cava",
	"dux-gop",
	"matugen",
	"qtengine",
	"nautilus",
	"firefox",
	"mpv",
	// Aelum's own media player and package manager: resolved from the [aelum]
	// repo by the ISO build and baked into the offline mirror, so this pacstrap
	// needs no network.
	"aelum-video-player",
	"aelum-package-installer",
	"imv",
	"gnome-text-editor",

	// Terminal & UI
	"kitty",
	"nautilus-open-any-terminal",
	"adwaita-cursors",
	"adw-gtk-theme",
	"fluent-icon-theme",
	"breeze",

	// Dux Shell's other optional dependencies are deliberately not shipped:
	//   cups-pk-helper  - no CUPS in this image, so there is no printer to manage
	//   i2c-tools       - external-monitor brightness already goes through ddcutil
	//   qt6ct           - Qt theming goes through qtengine (QT_QPA_PLATFORMTHEME)
	//   qt6-multimedia  - shell sound effects; not part of the Aelum desktop
	//   wtype           - 2.0 pastes the clipboard through zwp_virtual_keyboard_v1

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

package installdesktop

import (
	"aelum-installer/backend/install/installbase"
	"aelum-installer/backend/install/primitives"
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
	// Video thumbnails for the file manager. ffmpegthumbnailer installs the
	// freedesktop entries in /usr/share/thumbnailers that gnome-desktop (and so
	// Nautilus) runs; ffmpegthumbs is the KF6 thumbcreator plugin the KIO/Qt side
	// loads. Both depend on ffmpeg, which mpv already brings in.
	"ffmpegthumbnailer",
	"ffmpegthumbs",
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

	// Input method framework. The core is on every install, like the GPU set
	// is never empty: the seeded Hyprland env lines and the aelum-fcitx5
	// session unit (live-ISO source) always point at something, and "add an
	// input method later" is a plain package install — a first-run fcitx5
	// registers whatever engine appears. The per-language engines are chosen by
	// inputmethods.go and are deliberately not here.
	"fcitx5",
	"fcitx5-gtk",
	"fcitx5-qt",
	"fcitx5-configtool",

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

	// GPU drivers are deliberately not here: the set depends on the hardware,
	// and a fixed list is what made `pacman -S steam` ask between 13 providers
	// for lib32-vulkan-driver. drivers.go picks the set for the detected GPU
	// at the start of Run, and it always provides both Vulkan virtuals.

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

// Run installs the desktop environment packages via pacstrap. The GPU driver
// set is chosen first (drivers.go): the hardware decides which vendor's stack
// ships, the set is never empty, and every set provides both the vulkan-driver
// and lib32-vulkan-driver virtuals — so an installed system never gets the
// provider question. The chosen set is recorded on ctx.DriverPackages for the
// verify step to check. The input method addons are chosen the same way
// (inputmethods.go): the language decides which engines ship, getting none is
// a normal outcome, and the choice is recorded on ctx.IMPackages.
func Run(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}

	drivers := ChooseDriverPackages(ctx)
	imAddons := ChooseIMEAddons(ctx)
	base := installbase.AppendBtrfsOnly(ctx, DesktopPackages, BtrfsOnlyDesktopPackages)
	packages := make([]string, 0, len(base)+len(drivers)+len(imAddons))
	packages = append(packages, base...)
	packages = append(packages, drivers...)
	packages = append(packages, imAddons...)
	return installbase.InstallPackages(ctx, packages, "desktop environment")
}

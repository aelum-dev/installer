package install

import (
	"bufio"
	"fmt"
	"os/exec"

	"aestheticos-installer/backend/install/progress"
	"aestheticos-installer/backend/shared"
)

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
	"power-profiles-daemon",
	"yay-bin",
	"zram-generator",

	// Editor
	"vim",
	"nano",
}

// desktopPackages are user-space packages installed during the desktop
// environment step. Split from basePackages so the install progress bar
// moves smoothly instead of stalling during the long pacstrap call.
var desktopPackages = []string{
	// Boot splash
	"plymouth",

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

// installPackages runs pacstrap with the given package list.
// Streams stdout to parse real progress from pacstrap's output lines.
func installPackages(ctx *InstallContext, packages []string, description string) error {
	if err := validateMountedSystem(); err != nil {
		return err
	}

	if len(packages) == 0 {
		return fmt.Errorf("no packages to install")
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Installing %d %s packages via pacstrap", len(packages), description)
	}

	args := []string{"-K", shared.MountRoot}
	args = append(args, packages...)
	cmd := exec.Command("pacstrap", args...)

	// Stream stdout to parse real progress from pacstrap's lines.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start pacstrap: %w", err)
	}

	// Track progress across pacstrap's multiple phases.
	tracker := progress.NewPacstrapProgressTracker()

	// Large buffer for the "Packages (N) pkg1 pkg2 ..." line which can exceed
	// the default 64KB scanner buffer when installing many packages.
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	// Run scanner in a goroutine so we can select on ctx.cancel for clean
	// interrupt handling. Without this, SIGINT would leave the scanner
	// blocked on Read() until pacstrap exits on its own.
	scanDone := make(chan struct{})
	var scanErr error
	go func() {
		for scanner.Scan() {
			line := scanner.Text()

			// Process all line types: Packages count, downloading, verifying,
			// installing, and (N/M) hook progress.
			pct := tracker.ProcessLine(line)
			if ctx.ProgressCallback != nil {
				ctx.ProgressCallback(pct)
			}

			// Command output appends to the progress line via => prefix.
			if ctx.Logger != nil {
				ctx.Logger.Command(line)
			}
		}
		scanErr = scanner.Err()
		close(scanDone)
	}()

	select {
	case <-scanDone:
		if scanErr != nil {
			return fmt.Errorf("pacstrap stdout scan: %w", scanErr)
		}
		if err := cmd.Wait(); err != nil {
			return fmt.Errorf("pacstrap failed: %w", err)
		}
		return nil
	case <-ctx.Cancel:
		// Kill the child process so the scanner goroutine unblocks and exits.
		// Cleanup is handled by the Installer's defer, not here.
		_ = cmd.Process.Kill()
		<-scanDone
		return fmt.Errorf("interrupted")
	}
}

// Install installs the Arch Linux base system via pacstrap.
func Install(ctx *InstallContext) error {
	return installPackages(ctx, basePackages, "base system")
}

// InstallDesktop installs desktop environment packages via pacstrap.
func InstallDesktop(ctx *InstallContext) error {
	return installPackages(ctx, desktopPackages, "desktop environment")
}

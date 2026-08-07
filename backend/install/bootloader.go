package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aestheticos-installer/backend/shared"
)

// ConfigureBootloader installs systemd-boot and writes the boot entry configuration.
// When ctx.Partitions is nil, it auto-discovers the ESP and root partition from the live system.
//
// Steps performed:
//  1. Auto-discover ESP and root partition if not provided via FormatDisk
//  2. Run bootctl install --esp-path=<ESP> to install systemd-boot to the ESP
//  3. Discover kernel/initrd filenames from /mnt/boot/ (flavor-agnostic)
//  4. Write a Type #2 boot entry to <ESP>/loader/entries/aesthetic-os.conf
//  5. Set it as the default entry via bootctl set-default
//  6. Verify installation with bootctl status
//
// The boot entry uses the root partition UUID captured during formatting (or discovered
// on-the-fly), and kernel/initrd paths discovered from the installed system — so it works
// regardless of filesystem type (ext4, btrfs, etc.) or kernel flavor (linux, linux-lts, etc.).
func ConfigureBootloader(ctx *InstallContext) error {
	esp := ctx.ESP
	if esp == "" {
		esp = shared.BootMountPoint
	}

	// Check that /mnt is mounted (system must be installed and mounted)
	if err := validateMountedSystem(); err != nil {
		return err
	}

	// Validate preconditions
	if ctx.DiskID == "" {
		return fmt.Errorf("disk ID not set. Run --format-disk <disk> first")
	}
	if ctx.Partitions == nil {
		return fmt.Errorf("partitions not initialized. Run --format-disk <disk> first")
	}
	if ctx.FileSystem == "" {
		return fmt.Errorf("filesystem not set. Run --format-disk <disk> first")
	}
	if ctx.Partitions.RootUUID == "" {
		return fmt.Errorf("root UUID not set. FormatDisk may have failed")
	}

	// Step 1: Install systemd-boot to the ESP
	if err := installSystemdBoot(esp); err != nil {
		return err
	}

	// Step 2: Discover kernel and initrd filenames from the installed system
	kernel, initrd, err := discoverKernelFiles()
	if err != nil {
		return err
	}

	// Step 3: Write boot entry configuration
	if err := writeBootEntry(esp, ctx.Partitions.RootUUID, kernel, initrd, ctx.FileSystem); err != nil {
		return err
	}

	// Step 4: Set as default boot entry
	if err := setDefaultBootEntry(esp, shared.BootEntryFileName+".conf"); err != nil {
		return err
	}

	// Step 5: Clean up orphaned files from the ESP (leftover from previous installations)
	if err := cleanupEsp(esp); err != nil {
		return err
	}

	// Step 6: Generate random seed for the new system's entropy pool
	// This is best-effort — failure is not fatal
	if err := generateRandomSeed(esp); err != nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("Warning: random seed generation failed (non-fatal): %v", err)
		}
	}

	// Step 7: Verify installation
	return verifyBootloaderInstallation(esp)
}

// installSystemdBoot runs bootctl install to place systemd-boot on the ESP.
// Per the bootctl man page: "Installs systemd-boot into the EFI system partition."
//
// Note: running outside chroot, so --graceful is NOT implicitly enabled.
// We add it explicitly to make install robust against edge cases:
//   - ESP not found
//   - EFI variables can't be written
//   - Different/newer boot loader already installed
//
// Uses a custom boot option description so entries can be identified later.
func installSystemdBoot(esp string) error {
	cmd := Run("bootctl", "install", "--esp-path="+esp,
		"--graceful",
		"--efi-boot-option-description="+shared.BootOptionDescription,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bootctl install failed: %w: %s", err, string(output))
	}
	return nil
}

// discoverKernelFiles scans /mnt/boot/ for the kernel and initrd files.
// Returns the filenames (e.g. "vmlinuz-linux-lts", "initramfs-linux-lts.img").
// Works regardless of kernel flavor (linux, linux-lts, linux-zen, etc.).
func discoverKernelFiles() (kernel, initrd string, err error) {
	// List files in /mnt/boot/ matching vmlinuz-* using find (no shell glob needed)
	cmd := Run("find", shared.BootMountPoint, "-maxdepth", "1", "-name", shared.KernelFilePrefix+"*")
	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to list kernel files in %s: %w", shared.BootMountPoint, err)
	}

	files := strings.TrimSpace(string(output))
	if files == "" {
		return "", "", fmt.Errorf("no %s* files found in %s", shared.KernelFilePrefix, shared.BootMountPoint)
	}

	// Take the first (and typically only) match
	kernel = filepath.Base(strings.Split(files, "\n")[0])

	// Find matching initrd — same base name with .img extension
	base := strings.TrimPrefix(kernel, shared.KernelFilePrefix)
	initrd = shared.InitrdFilePrefix + base + shared.InitrdFileExtension

	// Verify initrd exists; if not, try the fallback initrd (initramfs-<base>-fallback.img)
	cmd = Run("test", "-f", shared.BootMountPoint+"/"+initrd)
	if err := cmd.Run(); err != nil {
		initrdFallback := shared.InitrdFilePrefix + base + "-fallback" + shared.InitrdFileExtension
		cmd = Run("test", "-f", shared.BootMountPoint+"/"+initrdFallback)
		if err := cmd.Run(); err != nil {
			return "", "", fmt.Errorf("no initrd found in %s for kernel %s (tried %s and %s)", shared.BootMountPoint, kernel, initrd, initrdFallback)
		}
		initrd = initrdFallback
	}

	return kernel, initrd, nil
}

// writeBootEntry creates a Type #2 boot loader entry file on the ESP.
// Per the Boot Loader Specification, Type #2 entries go in $ESP/loader/entries/.
// The entry references the root partition by UUID. For btrfs subvolume layouts,
// rootflags=subvol=@ is required so the kernel mounts the @ subvolume as root.
// Includes silent boot kernel parameters: quiet splash loglevel=3 rd.udev.log_level=3 vt.global_cursor_default=0
func writeBootEntry(esp, rootUUID, kernel, initrd, fsType string) error {
	options := "root=UUID=" + rootUUID + " rw quiet splash loglevel=3 rd.udev.log_level=3 vt.global_cursor_default=0"
	if fsType == shared.FileSystemBtrfs {
		options += " rootflags=subvol=@"
	}
	entryContent := fmt.Sprintf("title   %s\nlinux   /%s\ninitrd  /%s\noptions %s\n", shared.BootEntryTitle, kernel, initrd, options)

	entryPath := esp + "/" + shared.BootLoaderEntriesDir + "/" + shared.BootEntryFileName + ".conf"

	// Create parent directory
	cmd := Run("mkdir", "-p", esp+"/"+shared.BootLoaderEntriesDir)
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create boot entry dir: %w", err)
	}

	// Write boot entry
	if err := writeFile(esp+"/"+shared.BootLoaderEntriesDir+"/"+shared.BootEntryFileName+".conf", entryContent); err != nil {
		return fmt.Errorf("failed to write boot entry to %s: %w", entryPath, err)
	}

	return nil
}

// cleanupEsp removes orphaned files from the ESP and XBOOTLDR partitions that
// belong to the entry token but are not referenced by any boot loader entries.
// This cleans up leftover kernels, initrds, and entry files from previous installations.
// Per the bootctl man page: "Removes files from the ESP and XBOOTLDR partitions that
// belong to the entry token but are not referenced in any boot loader entries."
func cleanupEsp(esp string) error {
	cmd := Run("bootctl", "cleanup", "--esp-path="+esp,
		"--graceful",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bootctl cleanup failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// generateRandomSeed generates a random seed for the new system's entropy pool.
// Per the bootctl man page: "Generates a random seed and stores it in the EFI System
// Partition (ESP), for use by the systemd-boot boot loader."
// The seed is credited to the kernel's entropy pool during early boot.
func generateRandomSeed(esp string) error {
	cmd := Run("bootctl", "random-seed", "--esp-path="+esp)
	if err := cmd.Run(); err != nil {
		// Not fatal — entropy generation is a best-effort improvement
		return fmt.Errorf("failed to generate random seed (non-fatal): %w", err)
	}
	return nil
}

// setDefaultBootEntry sets the default boot entry using bootctl.
// Per the bootctl man page: "set-default ID — Sets the default boot loader entry."
func setDefaultBootEntry(esp, entryID string) error {
	cmd := Run("bootctl", "set-default", "--esp-path="+esp, entryID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bootctl set-default failed: %w: %s", err, string(output))
	}
	return nil
}

// verifyBootloaderInstallation checks that systemd-boot is installed and the boot entry exists.
// Returns detailed error messages if anything is missing.
// Informational output (entry contents, ESP files, bootctl status) is collected separately
// and only included in error messages — successful verification returns nil.
func verifyBootloaderInstallation(esp string) error {
	var issues []string
	var info []string

	// Check 1: bootctl is-installed
	cmd := Run("bootctl", "is-installed", "--esp-path="+esp)
	output, err := cmd.CombinedOutput()
	if err != nil {
		issues = append(issues, fmt.Sprintf("bootctl is-installed failed: %s", strings.TrimSpace(string(output))))
	}

	// Check 2: loader.conf exists
	cmd = Run("test", "-f", esp+"/loader/loader.conf")
	if err := cmd.Run(); err != nil {
		issues = append(issues, "loader.conf is MISSING")
	}

	// Check 3: boot entry file exists
	entryPath := esp + "/" + shared.BootLoaderEntriesDir + "/" + shared.BootEntryFileName + ".conf"
	cmd = Run("test", "-f", entryPath)
	if err := cmd.Run(); err != nil {
		issues = append(issues, "boot entry file is MISSING: "+entryPath)
	} else {
		// Capture entry contents as informational output (not an error)
		data, err := os.ReadFile(entryPath)
		if err == nil {
			info = append(info, "boot entry contents: "+strings.TrimSpace(string(data)))
		}
	}

	// Check 4: loader directory structure
	cmd = Run("find", esp, "-name", "*.conf", "-o", "-name", "*.efi", "-o", "-name", "loader.conf")
	output, err = cmd.Output()
	if err == nil {
		info = append(info, "ESP files: "+strings.TrimSpace(string(output)))
	}

	// Check 5: bootctl status
	cmd = Run("bootctl", "status", "--esp-path="+esp)
	output, err = cmd.CombinedOutput()
	if err == nil {
		info = append(info, "bootctl status:\n"+strings.TrimSpace(string(output)))
	}

	if len(issues) > 0 {
		// Include informational output in error for debugging
		if len(info) > 0 {
			issues = append(issues, "---", "--- Info ---")
			issues = append(issues, info...)
		}
		return fmt.Errorf("bootloader verification failed:\n  - %s", strings.Join(issues, "\n  - "))
	}

	return nil
}

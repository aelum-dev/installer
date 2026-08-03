package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"aestheticos-installer/backend/internal/shared"
)

// ConfigureBootloader installs systemd-boot and writes the boot entry configuration.
// When ctx.Partitions is nil, it auto-discovers the ESP and root partition from the live system.
//
// Steps performed:
//   1. Auto-discover ESP and root partition if not provided via FormatDisk
//   2. Run bootctl install --esp-path=<ESP> to install systemd-boot to the ESP
//   3. Discover kernel/initrd filenames from /mnt/boot/ (flavor-agnostic)
//   4. Write a Type #2 boot entry to <ESP>/loader/entries/aesthetic-os.conf
//   5. Set it as the default entry via bootctl set-default
//   6. Verify installation with bootctl status
//
// The boot entry uses the root partition UUID captured during formatting (or discovered
// on-the-fly), and kernel/initrd paths discovered from the installed system — so it works
// regardless of filesystem type (ext4, btrfs, etc.) or kernel flavor (linux, linux-lts, etc.).
func ConfigureBootloader(ctx *InstallContext) error {
	esp := ctx.ESP
	if esp == "" {
		esp = BootMountPoint
	}

	// Check that /mnt is mounted (system must be installed and mounted)
	if err := validateMountedSystem(ctx); err != nil {
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

	helper := ctx.Helper

	// Step 1: Install systemd-boot to the ESP
	if ctx.Logger != nil {
	}
	if err := installSystemdBoot(helper, esp); err != nil {
		return err
	}

	// Step 2: Discover kernel and initrd filenames from the installed system
	kernel, initrd, err := discoverKernelFiles(helper)
	if err != nil {
		return err
	}
	if ctx.Logger != nil {
	}

	// Step 3: Write boot entry configuration
	if ctx.Logger != nil {
	}
	if err := writeBootEntry(helper, esp, ctx.Partitions.RootUUID, kernel, initrd, ctx.FileSystem); err != nil {
		return err
	}

	// Step 4: Set as default boot entry
	if ctx.Logger != nil {
	}
	if err := setDefaultBootEntry(helper, BootEntryFileName+".conf"); err != nil {
		return err
	}

	// Step 5: Clean up orphaned files from the ESP (leftover from previous installations)
	if ctx.Logger != nil {
	}
	if err := cleanupEsp(helper, esp); err != nil {
		return err
	}

	// Step 6: Generate random seed for the new system's entropy pool
	// This is best-effort — failure is not fatal
	if ctx.Logger != nil {
	}
	if err := generateRandomSeed(helper, esp); err != nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("Warning: random seed generation failed (non-fatal): %v", err)
		}
	}

	// Step 7: Verify installation
	if ctx.Logger != nil {
	}
	return verifyBootloaderInstallation(helper, esp)
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
func installSystemdBoot(helper, esp string) error {
	cmd := exec.Command("sudo", helper, HelperCmdBootctlInstall, esp,
		BootctlGracefulFlag,
		"--efi-boot-option-description="+BootOptionDescription,
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
func discoverKernelFiles(helper string) (kernel, initrd string, err error) {
	// List files in /mnt/boot/ matching vmlinuz-* using find (no shell glob needed)
	cmd := exec.Command("sudo", helper, HelperCmdQueryFind, BootMountPoint, "-maxdepth", "1", "-name", KernelFilePrefix+"*")
	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to list kernel files in %s: %w", BootMountPoint, err)
	}

	files := strings.TrimSpace(string(output))
	if files == "" {
		return "", "", fmt.Errorf("no %s* files found in %s", KernelFilePrefix, BootMountPoint)
	}

	// Take the first (and typically only) match
	kernel = filepath.Base(strings.Split(files, "\n")[0])

	// Find matching initrd — same base name with .img extension
	base := strings.TrimPrefix(kernel, KernelFilePrefix)
	initrd = InitrdFilePrefix + base + InitrdFileExtension

	// Verify initrd exists; if not, try the fallback initrd (initramfs-<base>-fallback.img)
	cmd = exec.Command("sudo", helper, HelperCmdQueryTest, "-f", BootMountPoint+"/"+initrd)
	if err := cmd.Run(); err != nil {
		initrdFallback := InitrdFilePrefix + base + "-fallback" + InitrdFileExtension
		cmd = exec.Command("sudo", helper, HelperCmdQueryTest, "-f", BootMountPoint+"/"+initrdFallback)
		if err := cmd.Run(); err != nil {
			return "", "", fmt.Errorf("no initrd found in %s for kernel %s (tried %s and %s)", BootMountPoint, kernel, initrd, initrdFallback)
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
func writeBootEntry(helper, esp, rootUUID, kernel, initrd, fsType string) error {
	options := "root=UUID=" + rootUUID + " rw quiet splash loglevel=3 rd.udev.log_level=3 vt.global_cursor_default=0"
	if fsType == shared.FileSystemBtrfs {
		options += " rootflags=subvol=@"
	}
	entryContent := fmt.Sprintf("title   %s\nlinux   /%s\ninitrd  /%s\noptions %s\n", BootEntryTitle, kernel, initrd, options)

	entryPath := esp + "/" + BootLoaderEntriesDir + "/" + BootEntryFileName + ".conf"

	// Create parent directory
	cmd := exec.Command("sudo", helper, HelperCmdMkdir, esp+"/"+BootLoaderEntriesDir)
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create boot entry dir: %w", err)
	}

	// Write boot entry using helper's write-file command directly
	cmd = exec.Command("sudo", helper, HelperCmdWriteFile, entryPath)
	cmd.Stdin = strings.NewReader(entryContent)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to write boot entry to %s: %w: %s", entryPath, err, string(output))
	}

	return nil
}

// cleanupEsp removes orphaned files from the ESP and XBOOTLDR partitions that
// belong to the entry token but are not referenced by any boot loader entries.
// This cleans up leftover kernels, initrds, and entry files from previous installations.
// Per the bootctl man page: "Removes files from the ESP and XBOOTLDR partitions that
// belong to the entry token but are not referenced in any boot loader entries."
func cleanupEsp(helper, esp string) error {
	cmd := exec.Command("sudo", helper, HelperCmdBootctlCleanup, esp,
		BootctlGracefulFlag,
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
func generateRandomSeed(helper, esp string) error {
	cmd := exec.Command("sudo", helper, HelperCmdBootctlRandomSeed, esp)
	if err := cmd.Run(); err != nil {
		// Not fatal — entropy generation is a best-effort improvement
		fmt.Fprintf(os.Stderr, "Warning: failed to generate random seed: %v\n", err)
		return fmt.Errorf("failed to generate random seed (non-fatal): %w", err)
	}
	return nil
}

// setDefaultBootEntry sets the default boot entry using bootctl.
// Per the bootctl man page: "set-default ID — Sets the default boot loader entry."
func setDefaultBootEntry(helper, entryID string) error {
	cmd := exec.Command("sudo", helper, HelperCmdBootctlSetDefault, entryID)
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
func verifyBootloaderInstallation(helper, esp string) error {
	var issues []string
	var info []string

	// Check 1: bootctl is-installed
	cmd := exec.Command("sudo", helper, HelperCmdBootctlIsInstalled, esp)
	output, err := cmd.CombinedOutput()
	if err != nil {
		issues = append(issues, fmt.Sprintf("bootctl is-installed failed: %s", strings.TrimSpace(string(output))))
	}

	// Check 2: loader.conf exists
	cmd = exec.Command("sudo", helper, HelperCmdQueryTest, "-f", esp+"/loader/loader.conf")
	if err := cmd.Run(); err != nil {
		issues = append(issues, "loader.conf is MISSING")
	}

	// Check 3: boot entry file exists
	entryPath := esp + "/" + BootLoaderEntriesDir + "/" + BootEntryFileName + ".conf"
	cmd = exec.Command("sudo", helper, HelperCmdQueryTest, "-f", entryPath)
	if err := cmd.Run(); err != nil {
		issues = append(issues, "boot entry file is MISSING: "+entryPath)
	} else {
		// Capture entry contents as informational output (not an error)
		cmd = exec.Command("sudo", helper, HelperCmdReadFile, entryPath)
		output, err := cmd.Output()
		if err == nil {
			info = append(info, "boot entry contents: "+strings.TrimSpace(string(output)))
		}
	}

	// Check 4: loader directory structure
	cmd = exec.Command("sudo", helper, HelperCmdQueryFind, esp, "-name", "*.conf", "-o", "-name", "*.efi", "-o", "-name", "loader.conf")
	output, err = cmd.Output()
	if err == nil {
		info = append(info, "ESP files: "+strings.TrimSpace(string(output)))
	}

	// Check 5: bootctl status
	cmd = exec.Command("sudo", helper, HelperCmdBootctlStatus, esp)
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



package steps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ConfigureBootloader installs systemd-boot and writes the boot entry configuration.
// It receives known partition data from FormatDisk — no on-the-fly detection.
//
// Steps performed:
//   1. Run bootctl install --esp-path=<ESP> to install systemd-boot to the ESP
//   2. Discover kernel/initrd filenames from /mnt/boot/ (flavor-agnostic)
//   3. Write a Type #2 boot entry to <ESP>/loader/entries/aesthetic-os.conf
//   4. Set it as the default entry via bootctl set-default
//   5. Verify installation with bootctl status
//
// The boot entry uses the root partition UUID captured during formatting,
// and kernel/initrd paths discovered from the installed system — so it works
// regardless of filesystem type (ext4, btrfs, etc.) or kernel flavor (linux, linux-lts, etc.).
func ConfigureBootloader(ctx *InstallContext) error {
	p := ctx.Partitions
	if p == nil {
		return fmt.Errorf("partition info not available (FormatDisk must run first)")
	}

	esp := ctx.ESP
	if esp == "" {
		esp = "/mnt/boot"
	}

	helper := ctx.Helper

	// Step 1: Install systemd-boot to the ESP
	if err := installSystemdBoot(helper, esp); err != nil {
		return err
	}

	// Step 2: Discover kernel and initrd filenames from the installed system
	kernel, initrd, err := discoverKernelFiles(helper)
	if err != nil {
		return err
	}

	// Step 3: Write boot entry configuration
	if err := writeBootEntry(helper, esp, p.RootUUID, kernel, initrd); err != nil {
		return err
	}

	// Step 4: Set as default boot entry
	if err := setDefaultBootEntry(helper, "aesthetic-os.conf"); err != nil {
		return err
	}

	// Step 5: Clean up orphaned files from the ESP (leftover from previous installations)
	if err := cleanupEsp(helper, esp); err != nil {
		return err
	}

	// Step 6: Generate random seed for the new system's entropy pool
	// This is best-effort — failure is not fatal
	if err := generateRandomSeed(helper, esp); err != nil {
		return err
	}

	// Step 7: Verify installation
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
	cmd := exec.Command("sudo", helper, "run", "bootctl", "install",
		"--esp-path="+esp,
		"--graceful",
		"--efi-boot-option-description=Aesthetic OS",
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
	// List files in /mnt/boot/ matching vmlinuz-* and initramfs-*
	cmd := exec.Command("sudo", helper, "run", "bash", "-c", "ls /mnt/boot/vmlinuz-* 2>/dev/null")
	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to list kernel files in /mnt/boot: %w", err)
	}

	files := strings.TrimSpace(string(output))
	if files == "" {
		return "", "", fmt.Errorf("no vmlinuz-* files found in /mnt/boot/")
	}

	// Take the first (and typically only) match
	kernel = filepath.Base(strings.Split(files, "\n")[0])

	// Find matching initrd — same base name with .img extension
	base := strings.TrimPrefix(kernel, "vmlinuz-")
	initrd = "initramfs-" + base + ".img"

	// Verify initrd exists
	cmd = exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf("test -f /mnt/boot/%s", initrd))
	if err := cmd.Run(); err != nil {
		// Try the fallback naming: initramfs-<kernel>.img
		initrd = "initramfs-" + kernel + ".img"
		cmd = exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf("test -f /mnt/boot/%s", initrd))
		if err := cmd.Run(); err != nil {
			return "", "", fmt.Errorf("no initrd found in /mnt/boot/ for kernel %s", kernel)
		}
	}

	return kernel, initrd, nil
}

// writeBootEntry creates a Type #2 boot loader entry file on the ESP.
// Per the Boot Loader Specification, Type #2 entries go in $ESP/loader/entries/.
// The entry references the root partition by UUID, so it is filesystem-agnostic.
func writeBootEntry(helper, esp, rootUUID, kernel, initrd string) error {
	entryContent := fmt.Sprintf(`title   Aesthetic OS
linux   /%s
initrd  /%s
options root=UUID=%s rw
`, kernel, initrd, rootUUID)

	entryPath := esp + "/loader/entries/aesthetic-os.conf"

	cmd := exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf("mkdir -p %s/loader/entries && cat > %s << 'BOOTEOF'\n%s\nBOOTEOF", esp, entryPath, entryContent))
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
	cmd := exec.Command("sudo", helper, "run", "bootctl", "cleanup",
		"--esp-path="+esp,
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
func generateRandomSeed(helper, esp string) error {
	cmd := exec.Command("sudo", helper, "run", "bootctl", "random-seed",
		"--esp-path="+esp,
	)
	if err := cmd.Run(); err != nil {
		// Not fatal — entropy generation is a best-effort improvement
		fmt.Fprintf(os.Stderr, "Warning: failed to generate random seed: %v\n", err)
		return nil
	}
	return nil
}

// setDefaultBootEntry sets the default boot entry using bootctl.
// Per the bootctl man page: "set-default ID — Sets the default boot loader entry."
func setDefaultBootEntry(helper, entryID string) error {
	cmd := exec.Command("sudo", helper, "run", "bootctl", "set-default", entryID)
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
	cmd := exec.Command("sudo", helper, "run", "bootctl", "is-installed", "--esp-path="+esp)
	output, err := cmd.CombinedOutput()
	if err != nil {
		issues = append(issues, fmt.Sprintf("bootctl is-installed failed: %s", strings.TrimSpace(string(output))))
	}

	// Check 2: loader.conf exists
	cmd = exec.Command("sudo", helper, "run", "bash", "-c", "test -f "+esp+"/loader/loader.conf && echo OK || echo MISSING")
	output, err = cmd.Output()
	if strings.TrimSpace(string(output)) != "OK" {
		issues = append(issues, "loader.conf is MISSING")
	}

	// Check 3: boot entry file exists
	entryPath := esp + "/loader/entries/aesthetic-os.conf"
	cmd = exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf("test -f %s && echo OK || echo MISSING", entryPath))
	output, err = cmd.Output()
	if strings.TrimSpace(string(output)) != "OK" {
		issues = append(issues, "boot entry file is MISSING: "+entryPath)
	} else {
		// Capture entry contents as informational output (not an error)
		cmd = exec.Command("sudo", helper, "run", "cat", entryPath)
		output, err = cmd.Output()
		if err == nil {
			info = append(info, "boot entry contents: "+strings.TrimSpace(string(output)))
		}
	}

	// Check 4: loader directory structure
	cmd = exec.Command("sudo", helper, "run", "find", esp, "-name", "*.conf", "-o", "-name", "*.efi", "-o", "-name", "loader.conf")
	output, err = cmd.Output()
	if err == nil {
		info = append(info, "ESP files: "+strings.TrimSpace(string(output)))
	}

	// Check 5: bootctl status
	cmd = exec.Command("sudo", helper, "run", "bootctl", "status", "--esp-path="+esp)
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

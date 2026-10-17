package configure

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// Bootloader configuration
// ============================================================================

// ConfigureBootloader installs systemd-boot and writes the boot entry configuration.
func ConfigureBootloader(ctx *shared.InstallContext) error {
	esp := ctx.ESP
	if esp == "" {
		esp = shared.BootMountPoint
	}
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}
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

	if err := installSystemdBoot(esp); err != nil {
		return err
	}
	kernel, initrd, err := discoverKernelFiles()
	if err != nil {
		return err
	}
	if err := writeBootEntry(esp, ctx, kernel, initrd); err != nil {
		return err
	}
	if err := setDefaultBootEntry(esp, shared.BootEntryFileName+".conf"); err != nil {
		return err
	}
	if err := cleanupEsp(esp); err != nil {
		return err
	}
	if err := generateRandomSeed(esp); err != nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("Warning: random seed generation failed (non-fatal): %v", err)
		}
	}
	return verifyBootloaderInstallation(esp)
}

func installSystemdBoot(esp string) error {
	cmd := primitives.Run("bootctl", "install", "--esp-path="+esp, "--graceful", "--efi-boot-option-description="+shared.BootOptionDescription)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bootctl install failed: %w: %s", err, string(output))
	}
	return nil
}

func discoverKernelFiles() (kernel, initrd string, err error) {
	cmd := primitives.Run("find", shared.BootMountPoint, "-maxdepth", "1", "-name", shared.KernelFilePrefix+"*")
	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to list kernel files in %s: %w", shared.BootMountPoint, err)
	}
	files := strings.TrimSpace(string(output))
	if files == "" {
		return "", "", fmt.Errorf("no %s* files found in %s", shared.KernelFilePrefix, shared.BootMountPoint)
	}
	kernel = filepath.Base(strings.Split(files, "\n")[0])
	base := strings.TrimPrefix(kernel, shared.KernelFilePrefix)
	initrd = shared.InitrdFilePrefix + base + shared.InitrdFileExtension
	cmd = primitives.Run("test", "-f", shared.BootMountPoint+"/"+initrd)
	if err := cmd.Run(); err != nil {
		initrdFallback := shared.InitrdFilePrefix + base + "-fallback" + shared.InitrdFileExtension
		cmd = primitives.Run("test", "-f", shared.BootMountPoint+"/"+initrdFallback)
		if err := cmd.Run(); err != nil {
			return "", "", fmt.Errorf("no initrd found in %s for kernel %s (tried %s and %s)", shared.BootMountPoint, kernel, initrd, initrdFallback)
		}
		initrd = initrdFallback
	}
	return kernel, initrd, nil
}

// writeBootEntry writes the systemd-boot entry. The root device depends on the
// layout: the root LV when LVM is enabled, otherwise the LUKS mapper (encrypted)
// or the root partition UUID. When LUKS is enabled the rd.luks.name= argument
// maps the header to the "aelum" mapper (consumed by sd-encrypt).
func writeBootEntry(esp string, ctx *shared.InstallContext, kernel, initrd string) error {
	options := bootEntryOptions(ctx)
	entryContent := fmt.Sprintf("title   %s\nlinux   /%s\ninitrd  /%s\noptions %s\n", shared.BootEntryTitle, kernel, initrd, options)
	entryPath := esp + "/" + shared.BootLoaderEntriesDir + "/" + shared.BootEntryFileName + ".conf"
	cmd := primitives.Run("mkdir", "-p", esp+"/"+shared.BootLoaderEntriesDir)
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create boot entry dir: %w", err)
	}
	if err := primitives.WriteFile(esp+"/"+shared.BootLoaderEntriesDir+"/"+shared.BootEntryFileName+".conf", entryContent); err != nil {
		return fmt.Errorf("failed to write boot entry to %s: %w", entryPath, err)
	}
	return nil
}

// bootEntryOptions builds the kernel command line for the systemd-boot entry.
// The root device is the root LV (LVM), the decrypted LUKS mapper (encrypted),
// or the root filesystem UUID. LUKS entries carry rd.luks.name= so sd-encrypt
// maps the header to the "aelum" mapper before the root is mounted.
func bootEntryOptions(ctx *shared.InstallContext) string {
	root := "UUID=" + ctx.Partitions.RootUUID
	if ctx.Lvm {
		root = shared.LvmRootLvPath
	}

	var options string
	if ctx.Luks {
		// `splash` is required for Plymouth to take over the console and show the
		// LUKS unlock theme (without it Plymouth logs to /var/log/boot.log and you
		// get the raw cryptsetup text prompt). `quiet` + lowered log levels keep
		// boot silent behind the splash.
		options = fmt.Sprintf("rd.luks.name=%s=%s root=%s rw quiet splash loglevel=3 rd.udev.log_level=3 vt.global_cursor_default=0", ctx.Partitions.LuksUUID, shared.LuksMapperName, root)
	} else {
		options = "root=" + root + " rw quiet splash loglevel=3 rd.udev.log_level=3 vt.global_cursor_default=0"
	}
	if ctx.FileSystem == shared.FileSystemBtrfs {
		options += " rootflags=subvol=@"
	}
	return options
}

func cleanupEsp(esp string) error {
	cmd := primitives.Run("bootctl", "cleanup", "--esp-path="+esp, "--graceful")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bootctl cleanup failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func generateRandomSeed(esp string) error {
	cmd := primitives.Run("bootctl", "random-seed", "--esp-path="+esp)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate random seed (non-fatal): %w", err)
	}
	return nil
}

func setDefaultBootEntry(esp, entryID string) error {
	// Write the default= line directly to loader.conf instead of using
	// `bootctl set-default`, which fails when the live ISO was not booted
	// with systemd-boot (e.g. Ventoy/GRUB). The file write is all that's
	// needed — systemd-boot reads it at boot time.
	confPath := esp + "/loader/loader.conf"
	data, err := primitives.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to read loader.conf: %w", err)
	}
	content := applyLoaderConfDefaults(string(data), entryID)
	if err := primitives.WriteFile(confPath, content); err != nil {
		return fmt.Errorf("failed to write default entry to loader.conf: %w", err)
	}
	return nil
}

// applyLoaderConfDefaults returns loader.conf with the systemd-boot default
// entry and selection timeout set, replacing the existing lines when present or
// appending them when absent. A timeout is important because systemd-boot
// defaults to 0 (no menu) when it is unset, so the user can still pick an entry.
func applyLoaderConfDefaults(content, entryID string) string {
	re := regexp.MustCompile(`(?m)^default .*$`)
	if re.MatchString(content) {
		content = re.ReplaceAllString(content, "default "+entryID)
	} else {
		content += "default " + entryID + "\n"
	}

	timeRe := regexp.MustCompile(`(?m)^timeout .*$`)
	if timeRe.MatchString(content) {
		content = timeRe.ReplaceAllString(content, "timeout "+strconv.Itoa(shared.BootLoaderTimeout))
	} else {
		content += "timeout " + strconv.Itoa(shared.BootLoaderTimeout) + "\n"
	}
	return content
}

func verifyBootloaderInstallation(esp string) error {
	var issues []string
	var info []string

	cmd := primitives.Run("bootctl", "is-installed", "--esp-path="+esp)
	output, err := cmd.CombinedOutput()
	if err != nil {
		issues = append(issues, fmt.Sprintf("bootctl is-installed failed: %s", strings.TrimSpace(string(output))))
	}

	cmd = primitives.Run("test", "-f", esp+"/loader/loader.conf")
	if err := cmd.Run(); err != nil {
		issues = append(issues, "loader.conf is MISSING")
	}

	entryPath := esp + "/" + shared.BootLoaderEntriesDir + "/" + shared.BootEntryFileName + ".conf"
	cmd = primitives.Run("test", "-f", entryPath)
	if err := cmd.Run(); err != nil {
		issues = append(issues, "boot entry file is MISSING: "+entryPath)
	} else {
		data, err := primitives.ReadFile(entryPath)
		if err == nil {
			info = append(info, "boot entry contents: "+strings.TrimSpace(string(data)))
		}
	}

	cmd = primitives.Run("find", esp, "-name", "*.conf", "-o", "-name", "*.efi", "-o", "-name", "loader.conf")
	output, err = cmd.Output()
	if err == nil {
		info = append(info, "ESP files: "+strings.TrimSpace(string(output)))
	}

	cmd = primitives.Run("bootctl", "status", "--esp-path="+esp)
	output, err = cmd.CombinedOutput()
	if err == nil {
		info = append(info, "bootctl status:\n"+strings.TrimSpace(string(output)))
	}

	if len(issues) > 0 {
		if len(info) > 0 {
			issues = append(issues, "---", "--- Info ---")
			issues = append(issues, info...)
		}
		return fmt.Errorf("bootloader verification failed:\n  - %s", strings.Join(issues, "\n  - "))
	}
	return nil
}

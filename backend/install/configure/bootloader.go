package configure

import (
	"fmt"
	"strconv"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// Bootloader configuration (rEFInd)
// ============================================================================

// ConfigureBootloader installs rEFInd onto the ESP and writes its configuration.
//
// The whole rEFInd asset tree is copied from the package installed in the live
// ISO (/usr/share/refind), so nothing is downloaded. rEFInd replaces
// systemd-boot for every install (automatic and manual) and auto-detects other
// OS boot loaders on a shared ESP, which is what makes a dualboot menu work.
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

	if err := installRefind(esp); err != nil {
		return err
	}
	kernel, initrd, err := discoverKernelFiles()
	if err != nil {
		return err
	}
	if err := writeRefindConfig(esp, ctx, kernel, initrd); err != nil {
		return err
	}
	// NVRAM registration is best-effort, mirroring systemd-boot's --graceful:
	// some firmware (and some virtual machines) reject efibootmgr writes, but
	// the installed system is still bootable via the firmware boot menu. Log the
	// failure instead of failing an otherwise complete install.
	if err := registerRefindBootEntry(ctx); err != nil && ctx.Logger != nil {
		ctx.Logger.Error("Warning: could not register the rEFInd NVRAM boot entry (non-fatal): %v", err)
	}
	return verifyBootloaderInstallation(esp)
}

// installRefind copies the packaged rEFInd tree (binary, icons, drivers, tools,
// keys, sample config) onto the ESP. The destination directory is created
// explicitly: `cp -a` only creates the final component of a missing path, and
// EFI/ may not exist on a reused ESP assumed from another OS.
func installRefind(esp string) error {
	dest := esp + "/" + shared.RefindDir
	if err := primitives.Run("mkdir", "-p", dest).Run(); err != nil {
		return fmt.Errorf("failed to create rEFInd directory %s: %w", dest, err)
	}
	out, err := primitives.Run("cp", "-r", shared.RefindSourceDir+"/.", dest+"/").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy rEFInd from %s: %w: %s", shared.RefindSourceDir, err, strings.TrimSpace(string(out)))
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
	// Prefer the kernel this installer actually installed. A reused ESP (dualboot)
	// can also hold another distribution's kernel, and find(1) lists entries in
	// filesystem order — picking the wrong one writes a boot entry that points
	// Aelum's root flags at an unrelated kernel.
	kernel = shared.PickKernelFile(strings.Split(files, "\n"))
	if kernel == "" {
		return "", "", fmt.Errorf("no %s* files found in %s", shared.KernelFilePrefix, shared.BootMountPoint)
	}
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

// writeRefindConfig writes <ESP>/EFI/refind/refind.conf with a single Aelum
// menu entry. Paths inside the entry are ESP-relative, matching where pacstrap
// puts the kernel and initrd (the ESP is mounted at /boot).
func writeRefindConfig(esp string, ctx *shared.InstallContext, kernel, initrd string) error {
	path := esp + "/" + shared.RefindConfigPath
	if err := primitives.WriteFile(path, refindConfig(ctx, kernel, initrd)); err != nil {
		return fmt.Errorf("failed to write rEFInd config to %s: %w", path, err)
	}
	return nil
}

// refindConfig renders refind.conf. The kernel command line is the same one the
// old systemd-boot entry used (bootEntryOptions), so LUKS, LVM, and btrfs root
// handling is unchanged.
//
// scan_all_linux_kernels is disabled so rEFInd shows exactly one Aelum entry
// instead of also auto-listing every kernel on a shared ESP. EFI loaders from
// other OSes (Windows Boot Manager, GRUB) are still auto-detected. The
// dont_scan_* lines suppress leftovers from a previous systemd-boot install
// without deleting any files that may belong to another OS.
func refindConfig(ctx *shared.InstallContext, kernel, initrd string) string {
	var b strings.Builder
	b.WriteString("# Generated by the Aelum installer. Do not edit by hand.\n")
	fmt.Fprintf(&b, "timeout %d\n", shared.RefindTimeoutSeconds)
	fmt.Fprintf(&b, "default_selection %s\n", shared.BootEntryTitle)
	b.WriteString("scan_all_linux_kernels false\n")
	b.WriteString("dont_scan_dirs EFI/systemd\n")
	b.WriteString("dont_scan_files systemd-bootx64.efi,BOOTX64.EFI,bootx64.efi\n")
	b.WriteString("\n")
	fmt.Fprintf(&b, "menuentry %q {\n", shared.BootEntryTitle)
	fmt.Fprintf(&b, "    icon /%s\n", shared.RefindIconPath)
	fmt.Fprintf(&b, "    loader /%s\n", kernel)
	fmt.Fprintf(&b, "    initrd /%s\n", initrd)
	// The options string must be quoted: rEFInd tokenizes refind.conf and
	// without quotes it keeps only the first whitespace-separated word (and
	// rewrites unquoted slashes). That would drop `rw`, `splash`, and the LUKS
	// rd.luks.name= argument, producing an unbootable entry.
	fmt.Fprintf(&b, "    options %q\n", bootEntryOptions(ctx))
	b.WriteString("}\n")
	return b.String()
}

// bootEntryOptions builds the kernel command line for the rEFInd entry. The
// root device is the root LV (LVM), the decrypted LUKS mapper (encrypted), or
// the root filesystem UUID. LUKS entries carry rd.luks.name= so sd-encrypt maps
// the header to the "aelum" mapper before the root is mounted.
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

// registerRefindBootEntry adds (or leaves alone) the firmware boot entry for
// rEFInd. efibootmgr runs in the live environment and writes the host's NVRAM,
// exactly as archiso and bootctl do; it is not run inside arch-chroot.
func registerRefindBootEntry(ctx *shared.InstallContext) error {
	if ctx.Partitions == nil || ctx.Partitions.EFIPart == "" {
		return fmt.Errorf("EFI partition path not set")
	}
	if refindNvramEntryExists(ctx.Partitions.EFIPart) {
		return nil
	}
	partNum := partitionNumber(ctx.DiskID, ctx.Partitions.EFIPart)
	if partNum <= 0 {
		return fmt.Errorf("could not derive the ESP partition number from %s", ctx.Partitions.EFIPart)
	}
	out, err := primitives.Run(
		"efibootmgr",
		"--create",
		"--disk", ctx.DiskID,
		"--part", strconv.Itoa(partNum),
		"--loader", shared.RefindNvramLoader,
		"--label", shared.BootEntryTitle,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("efibootmgr --create failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// refindNvramEntryExists reports whether the firmware already has a boot entry
// pointing at this ESP's rEFInd binary. Re-creating it on every reinstall would
// pile up duplicate entries. The loader path alone is not enough to identify the
// entry: a dualboot machine may have another rEFInd on a different ESP with the
// same \EFI\refind\refind_x64.efi path. When the ESP's PARTUUID is readable,
// require the existing entry to reference it as well.
func refindNvramEntryExists(espPart string) bool {
	out, err := primitives.Run("efibootmgr", "-v").Output()
	if err != nil {
		return false
	}
	entries := strings.ToLower(string(out))
	if !strings.Contains(entries, strings.ToLower(shared.RefindNvramLoader)) {
		return false
	}
	partUUID, err := primitives.Run("blkid", "-s", "PARTUUID", "-o", "value", espPart).Output()
	if err != nil || strings.TrimSpace(string(partUUID)) == "" {
		// Can't identify the ESP; the loader path is the best signal available.
		return true
	}
	return strings.Contains(entries, strings.ToLower(strings.TrimSpace(string(partUUID))))
}

// partitionNumber extracts the partition number from an existing device path by
// stripping the disk prefix and the optional "p" separator used by NVMe/mmcblk
// devices: /dev/vda1 → 1, /dev/nvme0n1p1 → 1.
func partitionNumber(diskID, partPath string) int {
	if diskID == "" || partPath == "" {
		return 0
	}
	suffix := strings.TrimPrefix(partPath, diskID)
	if suffix == partPath {
		return 0
	}
	suffix = strings.TrimPrefix(suffix, "p")
	n, err := strconv.Atoi(suffix)
	if err != nil {
		return 0
	}
	return n
}

func verifyBootloaderInstallation(esp string) error {
	var issues []string
	var info []string

	binaryPath := esp + "/" + shared.RefindBinaryPath
	if err := primitives.Run("test", "-f", binaryPath).Run(); err != nil {
		issues = append(issues, "rEFInd binary is MISSING: "+binaryPath)
	} else {
		info = append(info, "rEFInd binary: "+binaryPath)
	}

	confPath := esp + "/" + shared.RefindConfigPath
	if err := primitives.Run("test", "-f", confPath).Run(); err != nil {
		issues = append(issues, "rEFInd config is MISSING: "+confPath)
	} else if data, err := primitives.ReadFile(confPath); err != nil {
		issues = append(issues, "failed to read rEFInd config "+confPath)
	} else {
		content := string(data)
		if !strings.Contains(content, fmt.Sprintf("menuentry %q", shared.BootEntryTitle)) {
			issues = append(issues, "rEFInd config is missing the "+shared.BootEntryTitle+" menu entry")
		} else {
			info = append(info, "rEFInd config contents: "+strings.TrimSpace(content))
		}
	}

	cmd := primitives.Run("find", esp+"/"+shared.RefindDir, "-maxdepth", "1", "-type", "f")
	if output, err := cmd.Output(); err == nil {
		info = append(info, "rEFInd files: "+strings.TrimSpace(string(output)))
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

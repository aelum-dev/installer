package configure

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// hiDPIMinWidth mirrors rEFInd's HIDPI_MIN (global.h). At this width and
// above, rEFInd doubles icon sizes; the theme then also needs its 2x font.
const hiDPIMinWidth = 1921

// ============================================================================
// Bootloader configuration (rEFInd)
// ============================================================================

// ConfigureBootloader installs rEFInd onto the ESP and writes its configuration.
//
// The whole rEFInd asset tree is copied from the package installed in the live
// ISO (/usr/share/refind), so nothing is downloaded. rEFInd replaces
// systemd-boot for every install (automatic and manual). The ESP belongs to
// this install and is always formatted, so its menu contains only Aelum's own
// entry plus any other OSes rEFInd finds on their own ESPs.
func ConfigureBootloader(ctx *shared.InstallContext) error {
	esp := ctx.ESP
	if esp == "" {
		esp = shared.ESPMountPoint
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
	if err := installRefindTheme(esp); err != nil {
		return err
	}
	kernel, initrd, err := discoverKernelFiles()
	if err != nil {
		return err
	}
	// Use the live display's preferred mode for the installed boot menu. An
	// explicit mode avoids rEFInd's "max" heuristic, which only grows when both
	// dimensions increase; QEMU's VESA mode list otherwise makes it pick a
	// 4:3 or 1:1 mode. Pick the font to match the selected mode.
	mode, haveMode := preferredDisplayMode("/sys/class/drm")
	hiDPI := haveMode && mode.Width >= hiDPIMinWidth
	if !haveMode {
		hiDPI = displayIsHiDPI("/sys/class/drm")
	}
	if err := writeRefindConfig(esp, ctx, kernel, initrd, mode, haveMode, hiDPI); err != nil {
		return err
	}
	// NVRAM registration is best-effort, mirroring systemd-boot's --graceful:
	// some firmware (and some virtual machines) reject efibootmgr writes, but
	// the installed system is still bootable via the firmware boot menu. Log the
	// failure instead of failing an otherwise complete install.
	if err := registerRefindBootEntry(ctx); err != nil && ctx.Logger != nil {
		ctx.Logger.Error("Warning: could not register the rEFInd NVRAM boot entry (non-fatal): %v", err)
	}
	return verifyBootloaderInstallation(esp, ctx)
}

// installRefind copies the packaged rEFInd tree (binary, icons, drivers, tools,
// keys, sample config) onto the freshly formatted ESP. The destination
// directory is created explicitly: `cp -a` only creates the final component of
// a missing path, and EFI/ may not exist on the new filesystem.
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

// installRefindTheme copies the branded Aelum theme (background, font, icons,
// selection highlight) from the live ISO onto the ESP. It is installed next to
// rEFInd rather than into the package's own tree so an rEFInd package upgrade
// cannot overwrite it. A theme shipped with the ISO is a hard requirement: the
// installer and the theme are baked into the same ISO, so a missing source
// means the ISO was built without it.
func installRefindTheme(esp string) error {
	if err := primitives.Run("test", "-d", shared.RefindThemeSourceDir).Run(); err != nil {
		return fmt.Errorf("rEFInd theme not found on ISO at %s (rebuild the ISO with the theme)", shared.RefindThemeSourceDir)
	}
	dest := esp + "/" + shared.RefindThemeDir
	if err := primitives.Run("mkdir", "-p", dest).Run(); err != nil {
		return fmt.Errorf("failed to create rEFInd theme directory %s: %w", dest, err)
	}
	out, err := primitives.Run("cp", "-r", shared.RefindThemeSourceDir+"/.", dest+"/").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy Aelum rEFInd theme from %s: %w: %s", shared.RefindThemeSourceDir, err, strings.TrimSpace(string(out)))
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
	// Prefer the kernel this installer actually installed. The bind-mounted
	// /boot is this installation's own directory, but it can still hold more
	// than one kernel (for example a fallback image added later), and find(1)
	// lists entries in filesystem order.
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

// displayMode is a display resolution in pixels.
type displayMode struct {
	Width  int
	Height int
}

// parseDisplayMode parses a DRM mode line ("1920x1080").
func parseDisplayMode(line string) (displayMode, bool) {
	width, height, ok := strings.Cut(strings.TrimSpace(line), "x")
	if !ok {
		return displayMode{}, false
	}
	w, err := strconv.Atoi(width)
	if err != nil || w <= 0 {
		return displayMode{}, false
	}
	h, err := strconv.Atoi(height)
	if err != nil || h <= 0 {
		return displayMode{}, false
	}
	return displayMode{Width: w, Height: h}, true
}

// preferredDisplayMode returns the preferred (first advertised) mode of the
// largest enabled, connected DRM connector. The live ISO runs on the same
// monitor and firmware as the installed rEFInd, so this is the mode the boot
// menu should use. "Preferred" is the monitor's native mode, not the largest
// mode: a 4K panel's list also contains 1080p/720p fallbacks.
func preferredDisplayMode(drmRoot string) (displayMode, bool) {
	paths, err := filepath.Glob(filepath.Join(drmRoot, "card*-*/modes"))
	if err != nil {
		return displayMode{}, false
	}

	var enabledMode, connectedMode displayMode
	var haveEnabled, haveConnected bool

	for _, modesPath := range paths {
		connector := filepath.Dir(modesPath)

		// A disconnected output can still have a stale modes file; ignore it.
		status, err := os.ReadFile(filepath.Join(connector, "status"))
		if err == nil && strings.TrimSpace(string(status)) != "connected" {
			continue
		}

		data, err := os.ReadFile(modesPath)
		if err != nil {
			continue
		}
		var mode displayMode
		var ok bool
		for _, line := range strings.Split(string(data), "\n") {
			if mode, ok = parseDisplayMode(line); ok {
				break
			}
		}
		if !ok {
			continue
		}

		// A connected-but-disabled output (lid closed, dock output off) is a
		// weaker candidate than an enabled one. Missing "enabled" files are
		// treated as enabled (some virtual connectors omit it).
		enabled, err := os.ReadFile(filepath.Join(connector, "enabled"))
		isEnabled := err != nil || strings.TrimSpace(string(enabled)) == "enabled"
		if isEnabled {
			if !haveEnabled || mode.Width*mode.Height > enabledMode.Width*enabledMode.Height {
				enabledMode, haveEnabled = mode, true
			}
		} else if !haveConnected || mode.Width*mode.Height > connectedMode.Width*connectedMode.Height {
			connectedMode, haveConnected = mode, true
		}
	}

	if haveEnabled {
		return enabledMode, true
	}
	return connectedMode, haveConnected
}

// displayIsHiDPI is the font-selection fallback when no preferred mode could be
// detected and rEFInd will use "resolution max". It reports whether any DRM
// connector advertises a mode at least as wide as rEFInd's HIDPI_MIN.
func displayIsHiDPI(drmRoot string) bool {
	paths, err := filepath.Glob(filepath.Join(drmRoot, "card*-*/modes"))
	if err != nil {
		return false
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if mode, ok := parseDisplayMode(line); ok && mode.Width >= hiDPIMinWidth {
				return true
			}
		}
	}
	return false
}

// writeRefindConfig writes the global refind.conf and this installation's menu
// fragment. Each installation has its own fragment under EFI/refind/aelum/, and
// the global file includes every fragment it finds, so one install never
// overwrites another's menu entry. Kernel and initramfs paths likewise live in
// a per-install directory on the ESP, so fragments cannot point at each other's
// images.
//
// A btrfs install gets a second fragment for its snapshots. It is written empty
// here and filled in by the installed system (see refind_snapshots.go), but it has
// to exist before the include list is collected: refind.conf can only include a
// file that is there, and the installer never runs again.
func writeRefindConfig(esp string, ctx *shared.InstallContext, kernel, initrd string, mode displayMode, modeOK, hiDPI bool) error {
	id := shared.InstallID(ctx.Partitions.RootUUID)

	entryPath := esp + "/" + shared.RefindEntryPath(id)
	if err := primitives.WriteFile(entryPath, refindEntry(ctx, id, kernel, initrd)); err != nil {
		return fmt.Errorf("failed to write rEFInd menu entry to %s: %w", entryPath, err)
	}

	if ctx.FileSystem == shared.FileSystemBtrfs {
		snapshotPath := esp + "/" + shared.RefindSnapshotEntryPath(id)
		if err := primitives.WriteFile(snapshotPath, refindSnapshotPlaceholder()); err != nil {
			return fmt.Errorf("failed to write the snapshot menu entry to %s: %w", snapshotPath, err)
		}
	}

	includes, err := refindEntryIncludes(esp)
	if err != nil {
		return err
	}

	path := esp + "/" + shared.RefindConfigPath
	if err := primitives.WriteFile(path, refindConfig(ctx, mode, modeOK, hiDPI, includes)); err != nil {
		return fmt.Errorf("failed to write rEFInd config to %s: %w", path, err)
	}
	return nil
}

// refindEntryIncludes lists the menu fragments present on the ESP and returns
// the include lines refind.conf needs. rEFInd resolves includes relative to its
// own directory, so the values omit the EFI/refind prefix.
func refindEntryIncludes(esp string) ([]string, error) {
	dir := esp + "/" + shared.RefindEntriesDir
	names, err := primitives.ReadDirNames(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list rEFInd menu entries in %s: %w", dir, err)
	}
	sort.Strings(names)

	includes := make([]string, 0, len(names))
	for _, name := range names {
		if strings.EqualFold(filepath.Ext(name), ".conf") {
			includes = append(includes, shared.RefindEntriesDirName+"/"+name)
		}
	}
	return includes, nil
}

// refindEntry renders one installation's menu fragment. Loader and initrd paths
// are ESP-relative and include this installation's boot directory
// (aelum/<id>/), which the installed system bind-mounts at /boot. Two installs
// on one ESP therefore boot their own kernel images.
func refindEntry(ctx *shared.InstallContext, id, kernel, initrd string) string {
	var b strings.Builder
	b.WriteString("# Generated by the Aelum installer. Do not edit by hand.\n")
	fmt.Fprintf(&b, "# Root partition: %s\n", ctx.Partitions.RootPart)
	fmt.Fprintf(&b, "menuentry %q {\n", shared.RefindEntryTitle(ctx.Partitions.RootPart))
	fmt.Fprintf(&b, "    icon /%s\n", shared.RefindIconPath)
	fmt.Fprintf(&b, "    loader /%s\n", shared.InstallBootFilePath(id, kernel))
	fmt.Fprintf(&b, "    initrd /%s\n", shared.InstallBootFilePath(id, initrd))
	// The options string must be quoted: rEFInd tokenizes refind.conf and
	// without quotes it keeps only the first whitespace-separated word (and
	// rewrites unquoted slashes). That would drop `rw`, `splash`, and the LUKS
	// rd.luks.name= argument, producing an unbootable entry.
	fmt.Fprintf(&b, "    options %q\n", bootEntryOptions(ctx))
	b.WriteString("}\n")
	return b.String()
}

// refindConfig renders the global refind.conf. The kernel command line lives in
// each installation's fragment, so this file only carries global options, the
// branded theme, and the include lines for every Aelum install on the ESP.
//
// scan_all_linux_kernels is disabled so rEFInd shows exactly one Aelum entry
// instead of also auto-listing every kernel it finds. Other OS loaders
// (Windows Boot Manager, systemd-boot, GRUB) on their own ESPs are
// auto-detected: the fallback BOOTX64.EFI copies are still suppressed because
// they duplicate the vendor-directory loader on the same ESP. The "+" prefix
// matters: without it rEFInd would replace its default tool/shim exclusion list
// instead of adding to it.
func refindConfig(ctx *shared.InstallContext, mode displayMode, modeOK, hiDPI bool, includes []string) string {
	var b strings.Builder
	b.WriteString("# Generated by the Aelum installer. Do not edit by hand.\n")
	// Use rEFInd's "max" to pick the largest resolution the UEFI GOP driver
	// actually supports. Some firmwares advertise a native mode (e.g. 2560x1440)
	// via the DRM subsystem that the GOP driver cannot set, causing rEFInd to
	// print "Error setting graphics mode …; using default mode!" and fall back.
	// "max" avoids this by letting rEFInd choose the highest supported mode.
	b.WriteString("resolution max\n")
	fmt.Fprintf(&b, "timeout %d\n", shared.RefindTimeoutSeconds)
	fmt.Fprintf(&b, "default_selection %q\n", shared.RefindEntryTitle(ctx.Partitions.RootPart))
	b.WriteString("scan_all_linux_kernels false\n")
	b.WriteString("dont_scan_files + BOOTX64.EFI,bootx64.efi\n")
	// Keep the firmware in graphics mode when launching Linux. Otherwise rEFInd
	// switches to text mode and prints its "Booting ..." / "Using load options
	// ..." diagnostic over a black screen before the kernel starts. osx is
	// rEFInd's default, so listing it preserves macOS behaviour.
	b.WriteString("use_graphics_for osx,linux\n")
	b.WriteString("\n")
	// The branded theme is a separate file so refind.conf stays readable and
	// the theme can be swapped without touching the boot entries. rEFInd resolves
	// this path relative to EFI/refind.
	fmt.Fprintf(&b, "# Branded Aelum boot-menu theme (background, font, icons, selection).\n")
	fmt.Fprintf(&b, "include %s\n", shared.RefindThemeInclude)
	// theme.conf ships the 1x font. rEFInd does not scale custom fonts, so
	// override it on HiDPI screens with the matching 2x sheet. A later "font"
	// token wins because rEFInd processes includes inline.
	if hiDPI {
		fmt.Fprintf(&b, "font %s\n", shared.RefindThemeFont2x)
	}
	b.WriteString("\n")
	// One include per fragment present on the ESP. Includes are processed
	// inline, so a fragment added outside this install is still honored.
	for _, include := range includes {
		fmt.Fprintf(&b, "include %s\n", include)
	}
	return b.String()
}

// bootEntryOptions builds the kernel command line for the rEFInd entry. The
// root device is the decrypted LUKS mapper (encrypted) or the root filesystem
// UUID. LUKS entries carry rd.luks.name= so sd-encrypt maps the header to the
// "aelum" mapper before the root is mounted.
func bootEntryOptions(ctx *shared.InstallContext) string {
	root := "UUID=" + ctx.Partitions.RootUUID

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
		"--label", shared.RefindEntryTitle(ctx.Partitions.RootPart),
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

func verifyBootloaderInstallation(esp string, ctx *shared.InstallContext) error {
	var issues []string
	var info []string

	binaryPath := esp + "/" + shared.RefindBinaryPath
	if err := primitives.Run("test", "-f", binaryPath).Run(); err != nil {
		issues = append(issues, "rEFInd binary is MISSING: "+binaryPath)
	} else {
		info = append(info, "rEFInd binary: "+binaryPath)
	}

	id := shared.InstallID(ctx.Partitions.RootUUID)
	entryPath := esp + "/" + shared.RefindEntryPath(id)
	title := shared.RefindEntryTitle(ctx.Partitions.RootPart)
	if data, err := primitives.ReadFile(entryPath); err != nil {
		issues = append(issues, "rEFInd menu entry is MISSING: "+entryPath)
	} else if !strings.Contains(string(data), fmt.Sprintf("menuentry %q", title)) {
		issues = append(issues, "rEFInd menu entry "+entryPath+" does not contain "+title)
	} else {
		info = append(info, "rEFInd menu entry: "+entryPath)
	}

	confPath := esp + "/" + shared.RefindConfigPath
	if data, err := primitives.ReadFile(confPath); err != nil {
		issues = append(issues, "rEFInd config is MISSING: "+confPath)
	} else {
		content := string(data)
		if !strings.Contains(content, "include "+shared.RefindEntryInclude(id)) {
			issues = append(issues, "rEFInd config does not include "+shared.RefindEntryInclude(id))
		}
		if !strings.Contains(content, "include "+shared.RefindThemeInclude) {
			issues = append(issues, "rEFInd config does not include the Aelum theme ("+shared.RefindThemeInclude+")")
		}
		// A btrfs install's snapshots are bootable through a second fragment. It is
		// empty until the first snapshot exists, but the include has to be in place
		// for the installed system to ever put anything in it.
		if ctx.FileSystem == shared.FileSystemBtrfs {
			snapshotPath := esp + "/" + shared.RefindSnapshotEntryPath(id)
			if _, err := primitives.ReadFile(snapshotPath); err != nil {
				issues = append(issues, "snapshot menu entry is MISSING: "+snapshotPath)
			} else if !strings.Contains(content, "include "+shared.RefindSnapshotEntryInclude(id)) {
				issues = append(issues, "rEFInd config does not include "+shared.RefindSnapshotEntryInclude(id))
			} else {
				info = append(info, "snapshot menu entry: "+snapshotPath)
			}
		}
		info = append(info, "rEFInd config contents: "+strings.TrimSpace(content))
	}

	// The theme is what makes the menu Aelum-branded; confirm the files the
	// include line depends on actually made it onto the ESP.
	for _, rel := range []string{
		shared.RefindThemeConfigPath,
		shared.RefindThemeBackgroundPath,
		shared.RefindIconPath,
		shared.RefindThemeDir + "/font.png",
		shared.RefindThemeDir + "/font-2x.png",
	} {
		path := esp + "/" + rel
		if err := primitives.Run("test", "-f", path).Run(); err != nil {
			issues = append(issues, "Aelum rEFInd theme file is MISSING: "+path)
		}
	}
	if len(issues) == 0 {
		info = append(info, "Aelum rEFInd theme installed at "+esp+"/"+shared.RefindThemeDir)
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

package configure

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"aelum-installer/backend/install/greetd"
	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// Configuration orchestrator
// ============================================================================

// Run merges the former separate configuration steps into a single
// "Configuring system" step. It runs in order:
//
//  1. Generate fstab
//  2. Configure bootloader
//  3. System configuration (hostname, timezone, locale, keymap, NTP, mirrors)
//  4. greetd + DMS greeter setup
//  5. Aelum LUKS unlock Plymouth theme (matches the live ISO)
//  6. zram swap configuration
//  7. Enable systemd services
func Run(ctx *shared.InstallContext) error {
	if err := GenerateFstab(ctx); err != nil {
		return err
	}
	if err := ConfigureBootloader(ctx); err != nil {
		return err
	}
	if err := ConfigureSystemBase(ctx); err != nil {
		return err
	}
	// Configure the initramfs HOOKS: plymouth for the boot splash (always, since
	// ConfigurePlymouth runs unconditionally), and, when LUKS is enabled, the
	// sd-encrypt hook so the initramfs unlocks the root before mounting it. This
	// runs before greetd/plymouth so the later plymouth regen preserves the hook.
	if err := configureInitramfs(ctx); err != nil {
		return err
	}
	if err := ConfigureGreetd(ctx); err != nil {
		return err
	}
	if err := ConfigurePlymouth(ctx); err != nil {
		return err
	}
	if err := ConfigureZram(ctx); err != nil {
		return err
	}
	return EnableServices(ctx)
}

// ============================================================================
// fstab generation
// ============================================================================

// GenerateFstab generates the fstab file for the installed system.
func GenerateFstab(ctx *shared.InstallContext) error {
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
	if ctx.Partitions.EFIUUID == "" || ctx.Partitions.RootUUID == "" {
		return fmt.Errorf("partition UUIDs not set. FormatDisk may have failed")
	}

	p := ctx.Partitions

	if ctx.Lvm {
		return writeLvmFstab(ctx)
	}
	if ctx.FileSystem == shared.FileSystemBtrfs {
		return writeBtrfsFstab(ctx)
	}

	fstabContent := generateExt4Fstab(ctx)
	if err := primitives.WriteFile("/etc/fstab", fstabContent); err != nil {
		return err
	}
	return verifyFstab(p.RootUUID, p.EFIUUID)
}

func generateBtrfsFstab(ctx *shared.InstallContext) string {
	p := ctx.Partitions
	subvolumes := shared.GetBtrfsSubvolumes()

	var lines []string
	lines = append(lines, "# <file system>                             <mount point>  <type>  <options>               <dump>  <pass>")
	lines = append(lines, fmt.Sprintf("UUID=%s  /boot  vfat    %s  0  2", p.EFIUUID, shared.EfiPartitionUmask))

	for _, sv := range subvolumes {
		opts := primitives.BuildMountOpts(&sv)
		lines = append(lines, fmt.Sprintf("UUID=%s  %s  btrfs   %s  0 0", p.RootUUID, sv.MountPoint, opts))
	}

	return strings.Join(lines, "\n") + "\n"
}

func generateExt4Fstab(ctx *shared.InstallContext) string {
	p := ctx.Partitions
	rootFstype := "ext4"
	if ctx.FileSystem != "" {
		rootFstype = ctx.FileSystem
	}
	return fmt.Sprintf(`# <file system>                             <mount point>  <type>  <options>       <dump>  <pass>
UUID=%s  /              %s    %s        0  1
UUID=%s  /boot          vfat    %s 0 2
`, p.RootUUID, rootFstype, shared.DefaultMountOptions, p.EFIUUID, shared.EfiPartitionUmask)
}

func writeBtrfsFstab(ctx *shared.InstallContext) error {
	fstabContent := generateBtrfsFstab(ctx)
	if err := primitives.WriteFile("/etc/fstab", fstabContent); err != nil {
		return err
	}
	return verifyFstab(ctx.Partitions.RootUUID, ctx.Partitions.EFIUUID)
}

// generateLvmFstab builds the fstab for an LVM layout. For btrfs it is the
// subvolume layout with /home split off onto the home logical volume: every
// subvolume except @home lives on the root LV (RootUUID), @home lives on the
// home LV (HomeUUID). For ext4 the root and home LVs are mounted directly at /
// and /home.
func generateLvmFstab(ctx *shared.InstallContext) string {
	if ctx.FileSystem != shared.FileSystemBtrfs {
		return generateLvmExt4Fstab(ctx)
	}

	p := ctx.Partitions

	var lines []string
	lines = append(lines, "# <file system>                             <mount point>  <type>  <options>               <dump>  <pass>")
	lines = append(lines, fmt.Sprintf("UUID=%s  /boot  vfat    %s  0  2", p.EFIUUID, shared.EfiPartitionUmask))

	for _, sv := range shared.GetLvmBtrfsSubvolumes() {
		opts := primitives.BuildMountOpts(&sv)
		lines = append(lines, fmt.Sprintf("UUID=%s  %s  btrfs   %s  0 0", p.RootUUID, sv.MountPoint, opts))
	}
	for _, sv := range shared.GetLvmHomeSubvolumes() {
		opts := primitives.BuildMountOpts(&sv)
		lines = append(lines, fmt.Sprintf("UUID=%s  %s  btrfs   %s  0 0", p.HomeUUID, sv.MountPoint, opts))
	}

	return strings.Join(lines, "\n") + "\n"
}

// generateLvmExt4Fstab builds the fstab for an ext4 LVM layout: the root LV is
// mounted at / and the home LV at /home, both ext4.
func generateLvmExt4Fstab(ctx *shared.InstallContext) string {
	p := ctx.Partitions
	return fmt.Sprintf(`# <file system>                             <mount point>  <type>  <options>       <dump>  <pass>
UUID=%s  /              ext4    %s        0  1
UUID=%s  /home          ext4    %s        0  2
UUID=%s  /boot          vfat    %s 0 2
`, p.RootUUID, shared.DefaultMountOptions, p.HomeUUID, shared.DefaultMountOptions, p.EFIUUID, shared.EfiPartitionUmask)
}

func writeLvmFstab(ctx *shared.InstallContext) error {
	fstabContent := generateLvmFstab(ctx)
	if err := primitives.WriteFile("/etc/fstab", fstabContent); err != nil {
		return err
	}
	return verifyFstab(ctx.Partitions.RootUUID, ctx.Partitions.EFIUUID)
}

func verifyFstab(rootUUID, efiUUID string) error {
	if rootUUID == "" {
		return fmt.Errorf("root UUID is empty — cannot verify fstab")
	}
	if efiUUID == "" {
		return fmt.Errorf("EFI UUID is empty — cannot verify fstab")
	}

	data, err := os.ReadFile(shared.MountRoot + "/etc/fstab")
	if err != nil {
		return fmt.Errorf("failed to read fstab: %w", err)
	}
	content := string(data)
	if len(content) == 0 || len(content) < shared.MinFstabContentLengthBytes {
		return fmt.Errorf("fstab is empty or too short (%d bytes)", len(content))
	}
	if !strings.Contains(content, rootUUID) {
		return fmt.Errorf("fstab missing root UUID %s (content length: %d bytes)", rootUUID, len(content))
	}
	if !strings.Contains(content, efiUUID) {
		return fmt.Errorf("fstab missing EFI UUID %s (content length: %d bytes)", efiUUID, len(content))
	}
	if !strings.Contains(content, "/ ") && !strings.Contains(content, "/\t") {
		return fmt.Errorf("fstab missing / mount point")
	}
	return nil
}

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
	data, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to read loader.conf: %w", err)
	}
	content := string(data)
	// Replace or append the default line (space-separated, no =).
	re := regexp.MustCompile(`(?m)^default .*$`)
	if re.MatchString(content) {
		content = re.ReplaceAllString(content, "default "+entryID)
	} else {
		content += "default " + entryID + "\n"
	}
	// Ensure a boot-selection timeout so the user can pick an entry at boot.
	// systemd-boot defaults to 0 (no menu) when timeout is unset, so set it
	// explicitly. Uses the shared BootLoaderTimeout constant.
	timeRe := regexp.MustCompile(`(?m)^timeout .*$`)
	if timeRe.MatchString(content) {
		content = timeRe.ReplaceAllString(content, "timeout "+strconv.Itoa(shared.BootLoaderTimeout))
	} else {
		content += "timeout " + strconv.Itoa(shared.BootLoaderTimeout) + "\n"
	}
	if err := primitives.WriteFile(confPath, content); err != nil {
		return fmt.Errorf("failed to write default entry to loader.conf: %w", err)
	}
	return nil
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
		data, err := os.ReadFile(entryPath)
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

// ============================================================================
// System configuration
// ============================================================================

// ConfigureSystemBase sets up system configuration in the installed system via chroot.
func ConfigureSystemBase(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}
	if ctx.Username == "" {
		return fmt.Errorf("username not set")
	}
	if ctx.Region.Code == "" {
		return fmt.Errorf("region not set")
	}
	region := ctx.Region

	if ctx.Hostname == "" {
		return fmt.Errorf("hostname not set")
	}
	if err := primitives.WriteFile("/etc/hostname", ctx.Hostname+"\n"); err != nil {
		return err
	}

	cmd := primitives.Run("ln", "-sf", fmt.Sprintf("/usr/share/zoneinfo/%s", region.Timezone), shared.MountRoot+"/etc/localtime")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set timezone: %w: %s", err, string(output))
	}

	localeGenPath := shared.MountRoot + "/etc/locale.gen"
	data, err := os.ReadFile(localeGenPath)
	if err != nil {
		return fmt.Errorf("failed to read locale.gen: %w", err)
	}
	localeGenContent := strings.Replace(string(data), "#"+region.Locale, region.Locale, 1)
	if localeGenContent == string(data) {
		return fmt.Errorf("locale %q not found (commented out) in /etc/locale.gen; cannot generate it", region.Locale)
	}
	if err := primitives.WriteFile("/etc/locale.gen", localeGenContent); err != nil {
		return err
	}

	output, err = primitives.RunChroot("locale-gen")
	if err != nil {
		return fmt.Errorf("failed to generate locale: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	if err := primitives.WriteFile("/etc/locale.conf", "LANG="+region.Locale+"\n"); err != nil {
		return err
	}

	if err := primitives.WriteFile("/etc/vconsole.conf", "KEYMAP="+region.Keymap+"\n"); err != nil {
		return err
	}
	if err := primitives.AppendFile("/etc/vconsole.conf", "FONT=ter-132n\n"); err != nil {
		return err
	}

	output, err = primitives.RunChroot("systemctl", "enable", "systemd-timesyncd")
	if err != nil {
		return fmt.Errorf("failed to enable NTP: %w: %s", err, string(output))
	}

	return copyMirrorList(region.Code)
}

func copyMirrorList(regionCode string) error {
	rawContent, err := shared.GetEmbeddedMirrorList(regionCode)
	if err != nil {
		return fmt.Errorf("no mirror list available for region %q: %w", regionCode, err)
	}
	content := uncommentMirrors(string(rawContent))
	return primitives.WriteFile("/etc/pacman.d/mirrorlist", content)
}

// configureInitramfs writes the installed system's mkinitcpio HOOKS line so the
// boot splash (plymouth) shows and, when LUKS/LVM is enabled, the initramfs
// unlocks the root / activates the volume group before mounting it. Aelum
// supports a systemd-based initramfs only, so the order is:
// systemd ... plymouth ... [sd-encrypt] ... [lvm2] ... filesystems.
func configureInitramfs(ctx *shared.InstallContext) error {
	// Edit the target system's config, not the live ISO's.
	confPath := shared.MountRoot + "/etc/mkinitcpio.conf"
	data, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", confPath, err)
	}
	content := string(data)

	// plymouth must come after systemd and before sd-encrypt (dm-crypt).
	hooks := []string{"base", "systemd", "plymouth", "autodetect", "microcode", "modconf", "kms", "keyboard", "sd-vconsole", "block"}
	if ctx.Luks {
		hooks = append(hooks, "sd-encrypt")
	}
	if ctx.Lvm {
		// Activate the volume group in the initramfs before the root filesystem
		// is mounted. Order is sd-encrypt (unlock, when enabled) → lvm2 →
		// filesystems. The hook is `lvm2`, not `sd-lvm2`.
		hooks = append(hooks, shared.LvmInitramfsHook)
	}
	hooks = append(hooks, "filesystems", "fsck")

	// Replace the HOOKS line in place, preserving MODULES/COMPRESSION/etc.
	re := regexp.MustCompile(`(?ms)^HOOKS=\(.*?\)`)
	newBlock := "HOOKS=(" + strings.Join(hooks, " ") + ")"
	if !re.MatchString(content) {
		return fmt.Errorf("HOOKS not found in %s", confPath)
	}
	content = re.ReplaceAllString(content, newBlock)

	if err := primitives.WriteFile(confPath, content); err != nil {
		return err
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("initramfs HOOKS configured: %s", newBlock)
	}

	// Regenerate initramfs images so the new hooks take effect.
	output, err := primitives.RunChroot("mkinitcpio", "-P")
	if err != nil {
		return fmt.Errorf("failed to regenerate initramfs: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}
	return nil
}

func uncommentMirrors(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, shared.MirrorListCommentedServerPrefix) {
			lines[i] = shared.MirrorListServerPrefix + strings.TrimPrefix(line, shared.MirrorListCommentedServerPrefix)
		}
	}
	return strings.Join(lines, "\n")
}

// ============================================================================
// greetd configuration
// ============================================================================

// ConfigureGreetd sets up greetd as the login manager with the DMS greeter.
func ConfigureGreetd(ctx *shared.InstallContext) error {
	exec := &greetdExecutor{ctx: ctx}
	gctx := &greetd.InstallContext{
		Username: ctx.Username,
		Logger:   ctx.Logger,
	}
	return greetd.ConfigureGreetd(exec, gctx)
}

type greetdExecutor struct {
	ctx *shared.InstallContext
}

func (e *greetdExecutor) RunChroot(args ...string) ([]byte, error) {
	return primitives.RunChroot(args...)
}

func (e *greetdExecutor) WriteFile(path, content string) error {
	return primitives.WriteFile(path, content)
}

func (e *greetdExecutor) Logger() greetd.Logger {
	return e.ctx.Logger
}

// ============================================================================
// Plymouth configuration
// ============================================================================

// aelumThemeISOPath is the Aelum LUKS unlock Plymouth theme as shipped on the
// live ISO. The installer copies it verbatim into the installed system so the
// early-boot (dm-crypt unlock) splash matches the live ISO exactly.
const aelumThemeISOPath = "/usr/share/plymouth/themes/aelum"

// ConfigurePlymouth sets up the Aelum LUKS unlock Plymouth theme for the
// installed system, matching the live ISO. It copies the branded theme files
// from the live ISO, points plymouthd.conf at the aelum theme, creates the
// default->aelum symlink, and regenerates the initramfs so the theme is baked
// in for early-boot unlock.
func ConfigurePlymouth(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}

	// 1. Copy the Aelum LUKS unlock theme from the live ISO into the target.
	if err := installAelumTheme(ctx); err != nil {
		return err
	}

	// 2. Point plymouthd.conf at the aelum theme. This mirrors the live ISO;
	//    UseFirmwareBackground=false keeps the dark boot background.
	plymouthdConf := `[Daemon]
Theme=aelum
ShowDelay=0
UseFirmwareBackground=false
# DeviceScale=1  # Uncomment and adjust for HiDPI
`
	if err := primitives.WriteFile("/etc/plymouth/plymouthd.conf", plymouthdConf); err != nil {
		return err
	}

	// 3. Create the default->aelum symlink. plymouth-set-default-theme does not
	//    manage the no-extension symlink the live ISO ships, so create it here.
	dstLink := shared.MountRoot + "/usr/share/plymouth/themes/default"
	if err := primitives.Run("rm", "-f", dstLink).Run(); err != nil {
		return fmt.Errorf("failed to clear existing plymouth default symlink: %w", err)
	}
	if err := primitives.Run("ln", "-s", "aelum", dstLink).Run(); err != nil {
		return fmt.Errorf("failed to create plymouth default symlink: %w", err)
	}

	// 4. Regenerate the initramfs so plymouth bakes the aelum theme in for
	//    early-boot. plymouth-set-default-theme only rewrites the Theme line in
	//    place, so the keys written in step 2 are preserved.
	output, err := primitives.RunChroot("plymouth-set-default-theme", "-R", "aelum")
	if err != nil {
		return fmt.Errorf("failed to set Plymouth aelum theme: %w: %s", err, string(output))
	}

	if ctx.Username != "" {
		if err := createHushlogin(ctx.Username); err != nil {
			if ctx.Logger != nil {
				ctx.Logger.Error("Warning: failed to create .hushlogin: %v", err)
			}
		}
	}
	return nil
}

// installAelumTheme copies the Aelum LUKS unlock Plymouth theme from the live
// ISO into the installed system. It fails loudly if the theme is missing from
// the ISO: the early-boot unlock splash depends on these files.
func installAelumTheme(ctx *shared.InstallContext) error {
	dstDir := shared.MountRoot + "/usr/share/plymouth/themes/aelum"

	if err := primitives.Run("test", "-d", aelumThemeISOPath).Run(); err != nil {
		return fmt.Errorf("cannot configure early-boot unlock splash: Aelum plymouth theme not found on ISO at %s (rebuild the ISO with the theme)", aelumThemeISOPath)
	}

	if err := primitives.Run("mkdir", "-p", dstDir).Run(); err != nil {
		return fmt.Errorf("failed to create plymouth theme dir: %w", err)
	}
	// Copy the theme contents (the .plymouth/.script and PNG assets).
	out, err := primitives.Run("cp", "-a", aelumThemeISOPath+"/.", dstDir+"/").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy Aelum plymouth theme from ISO: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("Installed Aelum LUKS unlock Plymouth theme")
	}
	return nil
}

// createHushlogin creates the user's .hushlogin so the first login shell
// stays quiet (no motd/banner) after the unlock splash.
func createHushlogin(username string) error {
	home := shared.MountRoot + filepath.Join("/home", username)
	cmd := primitives.Run("touch", filepath.Join(home, ".hushlogin"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create .hushlogin: %w: %s", err, string(output))
	}
	_, err = primitives.RunChroot("chown", username+":"+username, filepath.Join("/home", username, ".hushlogin"))
	if err != nil {
		return fmt.Errorf("failed to set .hushlogin ownership: %w", err)
	}
	return nil
}

// ============================================================================
// zram swap configuration
// ============================================================================

// ConfigureZram sets up zram-based compressed swap in the installed system.
func ConfigureZram(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}
	swapMiB := calculateZramSizeMiB()
	zramConf := fmt.Sprintf(`[zram0]
zram-size = %d
compression-algorithm = zstd
`, swapMiB)
	if ctx.Logger != nil {
		ctx.Logger.Info("Configuring zram swap: %d MiB", swapMiB)
	}
	if err := primitives.WriteFile("/etc/systemd/zram-generator.conf", zramConf); err != nil {
		return fmt.Errorf("failed to write zram-generator config: %w", err)
	}
	return nil
}

var meminfoKBRegex = regexp.MustCompile(`^MemTotal:\s+(\d+)\s+kB$`)

const zramSizeMaxMiB = 8192

func calculateZramSizeMiB() int {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 2048
	}
	for _, line := range strings.Split(string(data), "\n") {
		matches := meminfoKBRegex.FindStringSubmatch(line)
		if matches != nil {
			kb, err := strconv.ParseInt(matches[1], 10, 64)
			if err != nil {
				continue
			}
			giB := int(kb / 1048576)
			switch {
			case giB <= 4:
				return 2048
			case giB <= 8:
				return 4096
			default:
				return zramSizeMaxMiB
			}
		}
	}
	return 2048
}

// ============================================================================
// Service management
// ============================================================================

var servicesToEnable = []string{"NetworkManager", "greetd", "bluetooth"}

// EnableServices enables all essential systemd services in the installed system.
func EnableServices(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}
	for _, svc := range servicesToEnable {
		if ctx.Logger != nil {
			ctx.Logger.Info("Enabling service: %s", svc)
		}
		output, err := primitives.RunChroot("systemctl", "enable", svc)
		if err != nil {
			return fmt.Errorf("failed to enable service %q: %w: %s", svc, err, string(output))
		}
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("All services enabled successfully")
	}
	return nil
}

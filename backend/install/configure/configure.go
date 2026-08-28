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
//  5. Plymouth BGRT silent boot
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
	if err := writeBootEntry(esp, ctx.Partitions.RootUUID, kernel, initrd, ctx.FileSystem); err != nil {
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

func writeBootEntry(esp, rootUUID, kernel, initrd, fsType string) error {
	options := "root=UUID=" + rootUUID + " rw quiet splash loglevel=3 rd.udev.log_level=3 vt.global_cursor_default=0"
	if fsType == shared.FileSystemBtrfs {
		options += " rootflags=subvol=@"
	}
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

// ConfigurePlymouth sets up Plymouth BGRT theme for silent boot.
func ConfigurePlymouth(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}

	plymouthdConf := `[Daemon]
Theme=bgrt
ShowDelay=0
UseFirmwareBackground=true
`
	if err := primitives.WriteFile("/etc/plymouth/plymouthd.conf", plymouthdConf); err != nil {
		return err
	}

	output, err := primitives.RunChroot("plymouth-set-default-theme", "-R", "bgrt")
	if err != nil {
		return fmt.Errorf("failed to set Plymouth BGRT theme: %w: %s", err, string(output))
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

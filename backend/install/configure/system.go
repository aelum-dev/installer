package configure

import (
	"fmt"
	"regexp"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

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
	data, err := primitives.ReadFile(localeGenPath)
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
	data, err := primitives.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", confPath, err)
	}
	content := string(data)

	// plymouth must come after systemd and before sd-encrypt (dm-crypt). The
	// aelum-luks-hint hook (encrypted boots only) is baked in here, before the
	// initramfs that uses it is generated: it carries the retry watcher, the
	// poweroff unit, and the drop-ins that gate the boot bar on unlock.
	hooks := []string{"base", "systemd", "plymouth"}
	if ctx.Luks {
		if err := installLuksHint(ctx); err != nil {
			// Best-effort, like the boot-progress unit: a broken hint must not
			// fail the whole install, so the hook is only added when every file
			// was written.
			if ctx.Logger != nil {
				ctx.Logger.Error("Warning: failed to install LUKS unlock feedback: %v", err)
			}
		} else {
			hooks = append(hooks, luksHintHookName)
		}
	}
	hooks = append(hooks, "autodetect", "microcode", "modconf", "kms", "keyboard", "sd-vconsole", "block")
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

package configure

import (
	"fmt"
	"os"
	"regexp"
	"slices"
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
	if ctx.Locale == "" {
		return fmt.Errorf("locale not set")
	}
	region := ctx.Region

	// The clock is the person's choice; the region only pre-fills it. The installer
	// resolved the fallback already, so an empty zone here is a bug, not "use the
	// region" — but answer the region's clock rather than write a broken symlink.
	timezone := ctx.Timezone
	if timezone == "" {
		timezone = region.Timezone
	}

	if ctx.Hostname == "" {
		return fmt.Errorf("hostname not set")
	}
	if err := primitives.WriteFile("/etc/hostname", ctx.Hostname+"\n"); err != nil {
		return err
	}

	cmd := primitives.Run("ln", "-sf", fmt.Sprintf("/usr/share/zoneinfo/%s", timezone), shared.MountRoot+"/etc/localtime")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set timezone: %w: %s", err, string(output))
	}

	// The language list is every locale glibc can generate on the build host, but
	// the target's glibc comes from the ISO's package set, which can be older. A
	// locale the target cannot generate would make locale-gen fail the install at
	// ~90%, so check it exists in the target first and fall back to the default
	// locale with a warning rather than aborting.
	locale := ctx.Locale
	if !localeAvailableInTarget(locale) {
		fallback := shared.DefaultLanguageCode + ".UTF-8"
		if ctx.Logger != nil {
			ctx.Logger.Error("Warning: the target's glibc cannot generate locale %q; installing %q instead", locale, fallback)
		}
		locale = fallback
	}

	localeGenPath := shared.MountRoot + "/etc/locale.gen"
	data, err := primitives.ReadFile(localeGenPath)
	if err != nil {
		return fmt.Errorf("failed to read locale.gen: %w", err)
	}
	// The locale comes from the language the person chose; the timezone, console
	// keymap and mirrors below come from where the machine is. They are separate
	// inputs because they are separate questions.
	localeGenContent := enableLocale(string(data), locale)
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

	if err := primitives.WriteFile("/etc/locale.conf", "LANG="+locale+"\n"); err != nil {
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

// copyMirrorList writes the installed system's mirror list: the chosen country's
// mirrors first, then the worldwide ones as a fallback (shared.BuildMirrorList),
// with the Server lines activated. This list is only ever used after the install
// — the install itself reads the mirror inside the ISO.
func copyMirrorList(regionCode string) error {
	rawContent, err := shared.BuildMirrorList(regionCode)
	if err != nil {
		return fmt.Errorf("no mirror list available for region %q: %w", regionCode, err)
	}
	content := uncommentMirrors(string(rawContent))
	return primitives.WriteFile(shared.PacmanMirrorListPath, content)
}

// configureInitramfs writes the installed system's mkinitcpio HOOKS line so the
// boot splash (plymouth) shows and, when LUKS and/or LVM are enabled, the
// initramfs unlocks and activates the root before mounting it. Aelum supports a
// systemd-based initramfs only, so the order is:
// systemd ... plymouth ... [sd-encrypt] ... [lvm2] ... filesystems.
//
// The two halves of that order that are not cosmetic: sd-encrypt must precede
// lvm2 (the decrypted device is the PV), and systemd must precede lvm2, because
// the lvm2 hook picks its activation path by whether install/systemd has been
// sourced by the time its build() runs — see shared.LvmInitramfsHook.
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
	// lvm2 activates the volume group in the initramfs. LUKS sits below LVM, so
	// the hook must follow sd-encrypt (the decrypted device is the PV) and
	// precede filesystems (the root LV is what gets mounted).
	if ctx.Lvm {
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

	// NVIDIA early KMS: the HOOKS above end in autodetect, and autodetect
	// inspects the *live session*, which has no NVIDIA module loaded — an
	// NVIDIA install's initramfs would carry no nvidia module and the branded
	// splash would not come up before root. When the desktop step chose the
	// NVIDIA set, name the modules in this same pass (the names are the same
	// for the open kernel module); the packages exist by now because the
	// desktop step installed them and mkinitcpio -P below runs after this
	// edit. Whether nvidia_drm.modeset=1 must also go into the boot entry is
	// measured on real hardware, not assumed here.
	if slices.Contains(ctx.DriverPackages, shared.NvidiaDriverPackage) {
		modules := "MODULES=(" + strings.Join(shared.NvidiaInitramfsModules, " ") + ")"
		modulesRe := regexp.MustCompile(`(?ms)^MODULES=\(.*?\)`)
		if modulesRe.MatchString(content) {
			content = modulesRe.ReplaceAllString(content, modules)
		} else {
			// The stock file always has MODULES=(); if a future one does not,
			// adding the line is the only way the modules can be named.
			content = modules + "\n" + content
		}
		if ctx.Logger != nil {
			ctx.Logger.Info("initramfs %s", modules)
		}
	}

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

// localeAvailableInTarget reports whether the installed system's glibc can
// generate a locale — that is, whether its locale source file is present in the
// target. The file is named by the locale's base ("en_US" for en_US.UTF-8,
// "sr_RS" for sr_RS.UTF-8@latin: the @modifier is a variant of the same source),
// under /usr/share/i18n/locales. A locale the target does not have would make
// locale-gen fail the install near the end, so the caller checks first and falls
// back. An unreadable directory is treated as available: the installer must not
// refuse a locale it merely could not confirm.
func localeAvailableInTarget(locale string) bool {
	base := locale
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	if base == "" {
		return false
	}
	_, err := primitives.Stat(shared.MountRoot + "/usr/share/i18n/locales/" + base)
	if err != nil && !os.IsNotExist(err) {
		return true
	}
	return err == nil
}

// enableLocale makes sure locale is enabled (uncommented) in the target's
// /etc/locale.gen. It accepts the standard forms of the file:
//
//	#en_US.UTF-8 UTF-8      (commented, no space)
//	# en_US.UTF-8 UTF-8     (commented, with space)
//	en_US.UTF-8 UTF-8       (already enabled)
//
// An already-enabled locale is left untouched. When the file has no entry for
// the locale at all, a UTF-8 line is appended rather than aborting the install:
// locale-gen will then either generate it or report the real problem. The
// installer must not fail at ~90% over a cosmetic mismatch in locale.gen.
func enableLocale(content, locale string) string {
	lines := strings.Split(content, "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		commented := strings.HasPrefix(trimmed, "#")
		body := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
		fields := strings.Fields(body)
		if len(fields) == 0 || fields[0] != locale {
			continue
		}
		found = true
		if commented {
			lines[i] = body
		}
		break
	}
	if !found {
		newLine := locale + " UTF-8"
		// Keep the file's trailing newline: append before the empty element that
		// a trailing "\n" produces, then restore it.
		if n := len(lines); n > 0 && lines[n-1] == "" {
			lines[n-1] = newLine
			lines = append(lines, "")
		} else {
			lines = append(lines, newLine)
		}
	}
	return strings.Join(lines, "\n")
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

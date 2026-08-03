package install

import (
	"fmt"
	"strings"

	"aestheticos-installer/backend/internal/shared"
)

// ConfigureSystem sets up system configuration in the installed system via chroot.
// Configures: hostname, timezone, locale, keymap — all from region data.
func ConfigureSystem(ctx *InstallContext) error {
	if err := validateMountedSystem(ctx); err != nil {
		return err
	}

	if ctx.Username == "" {
		return fmt.Errorf("username not set")
	}

	if ctx.Region.Code == "" {
		return fmt.Errorf("region not set")
	}

	region := ctx.Region

	// Step 1: Set hostname
	if ctx.Hostname == "" {
		return fmt.Errorf("hostname not set")
	}
	if ctx.Logger != nil {
	}
	if err := writeFile(ctx, "/etc/hostname", ctx.Hostname+"\n"); err != nil {
		return err
	}

	// Step 2: Set timezone from region
	if ctx.Logger != nil {
	}
	cmd := ctx.run(HelperCmdSetSymlink,
		fmt.Sprintf("/usr/share/zoneinfo/%s", region.Timezone),
		shared.MountRoot+"/etc/localtime")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set timezone: %w: %s", err, string(output))
	}

	// Step 3: Configure locale from region
	// Uncomment locale in locale.gen by reading, modifying, and writing back
	if ctx.Logger != nil {
	}
	localeGenPath := shared.MountRoot + "/etc/locale.gen"
	cmd = ctx.run(HelperCmdReadFile, localeGenPath)
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to read locale.gen: %w", err)
	}
	localeGenContent := string(output)
	// Use fixed-string replacement to avoid regex metacharacters in locale names (e.g. ".")
	localeGenContent = strings.Replace(localeGenContent,
		"#"+region.Locale, region.Locale,
		1)
	if err := writeFile(ctx, "/etc/locale.gen", localeGenContent); err != nil {
		return err
	}

	// Generate locale
	if ctx.Logger != nil {
	}
	cmd = ctx.run(HelperCmdRunLocaleGen)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to generate locale: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	// Write locale.conf
	if err := writeFile(ctx, "/etc/locale.conf", "LANG="+region.Locale+"\n"); err != nil {
		return err
	}

	// Step 4: Set console keymap from region
	if ctx.Logger != nil {
	}
	// Write KEYMAP line
	if err := writeFile(ctx, "/etc/vconsole.conf", "KEYMAP="+region.Keymap+"\n"); err != nil {
		return err
	}
	// Append FONT line
	if err := appendFile(ctx, "/etc/vconsole.conf", "FONT=ter-132n\n"); err != nil {
		return err
	}

	// Step 5: Enable NTP via systemd-timesyncd
	if ctx.Logger != nil {
	}
	cmd = ctx.run(HelperCmdEnableService, "systemd-timesyncd")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to enable NTP: %w: %s", err, string(output))
	}

	// Step 6: Copy mirror list for region
	if ctx.Logger != nil {
	}
	if err := copyMirrorList(ctx, region.Code); err != nil {
		return err
	}

	return nil
}

// copyMirrorList copies the region-specific mirror list to /etc/pacman.d/mirrorlist.
func copyMirrorList(ctx *InstallContext, regionCode string) error {
	// Read from embedded mirrorlist (baked into binary)
	// Falls back to default (worldwide) if region-specific file doesn't exist
	rawContent, err := shared.GetEmbeddedMirrorList(regionCode)
	if err != nil {
		return fmt.Errorf("no mirror list available for region %q: %w", regionCode, err)
	}

	// Uncomment mirror lines (strip leading # from #Server = ...)
	content := uncommentMirrors(string(rawContent))

	// Write to target mirrorlist
	if err := writeFile(ctx, "/etc/pacman.d/mirrorlist", content); err != nil {
		return err
	}

	return nil
}

// uncommentMirrors removes the leading # from #Server = lines in the mirrorlist.
func uncommentMirrors(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, shared.MirrorListCommentedServerPrefix) {
			lines[i] = shared.MirrorListServerPrefix + strings.TrimPrefix(line, shared.MirrorListCommentedServerPrefix)
		}
	}
	return strings.Join(lines, "\n")
}

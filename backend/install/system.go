package install

import (
	"fmt"
	"os"
	"strings"

	"aestheticos-installer/backend/shared"
)

// ConfigureSystemBase sets up system configuration in the installed system via chroot.
// Configures: hostname, timezone, locale, keymap — all from region data.
// Called by ConfigureSystem (the merged step) in configuration.go.
func ConfigureSystemBase(ctx *InstallContext) error {
	if err := validateMountedSystem(); err != nil {
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
	if err := writeFile("/etc/hostname", ctx.Hostname+"\n"); err != nil {
		return err
	}

	// Step 2: Set timezone from region
	cmd := Run("ln", "-sf",
		fmt.Sprintf("/usr/share/zoneinfo/%s", region.Timezone),
		shared.MountRoot+"/etc/localtime")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set timezone: %w: %s", err, string(output))
	}

	// Step 3: Configure locale from region
	// Uncomment locale in locale.gen by reading, modifying, and writing back
	localeGenPath := shared.MountRoot + "/etc/locale.gen"
	data, err := os.ReadFile(localeGenPath)
	if err != nil {
		return fmt.Errorf("failed to read locale.gen: %w", err)
	}
	localeGenContent := string(data)
	// Use fixed-string replacement to avoid regex metacharacters in locale names (e.g. ".")
	localeGenContent = strings.Replace(localeGenContent,
		"#"+region.Locale, region.Locale,
		1)
	if err := writeFile("/etc/locale.gen", localeGenContent); err != nil {
		return err
	}

	// Generate locale
	output, err = runChroot("locale-gen")
	if err != nil {
		return fmt.Errorf("failed to generate locale: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	// Write locale.conf
	if err := writeFile("/etc/locale.conf", "LANG="+region.Locale+"\n"); err != nil {
		return err
	}

	// Step 4: Set console keymap from region
	// Write KEYMAP line
	if err := writeFile("/etc/vconsole.conf", "KEYMAP="+region.Keymap+"\n"); err != nil {
		return err
	}
	// Append FONT line
	if err := appendFile("/etc/vconsole.conf", "FONT=ter-132n\n"); err != nil {
		return err
	}

	// Step 5: Enable NTP via systemd-timesyncd
	output, err = runChroot("systemctl", "enable", "systemd-timesyncd")
	if err != nil {
		return fmt.Errorf("failed to enable NTP: %w: %s", err, string(output))
	}

	// Step 6: Copy mirror list for region
	if err := copyMirrorList(region.Code); err != nil {
		return err
	}

	return nil
}

// copyMirrorList copies the region-specific mirror list to /etc/pacman.d/mirrorlist.
func copyMirrorList(regionCode string) error {
	// Read from embedded mirrorlist (baked into binary)
	// Falls back to default (worldwide) if region-specific file doesn't exist
	rawContent, err := shared.GetEmbeddedMirrorList(regionCode)
	if err != nil {
		return fmt.Errorf("no mirror list available for region %q: %w", regionCode, err)
	}

	// Uncomment mirror lines (strip leading # from #Server = ...)
	content := uncommentMirrors(string(rawContent))

	// Write to target mirrorlist
	if err := writeFile("/etc/pacman.d/mirrorlist", content); err != nil {
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

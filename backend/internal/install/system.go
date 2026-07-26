package install

import (
	"fmt"
	"os/exec"
	"regexp"
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

	helper := ctx.Helper
	region := ctx.Region

	// Step 1: Set hostname
	if ctx.Hostname == "" {
		return fmt.Errorf("hostname not set")
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("Setting hostname to '%s'", ctx.Hostname)
	}
	// Use printf with positional args to avoid shell interpolation of hostname
	cmd := exec.Command("sudo", helper, HelperCmdRun, "bash", "-c",
		"printf '%s\\n' \"$1\" > \"$2\"", "--", ctx.Hostname, shared.MountRoot+"/etc/hostname")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set hostname: %w: %s", err, string(output))
	}

	// Step 2: Set timezone from region
	if ctx.Logger != nil {
		ctx.Logger.Info("Setting timezone to %s", region.Timezone)
	}
	cmd = exec.Command("sudo", helper, HelperCmdRun, "bash", "-c", fmt.Sprintf("ln -sf /usr/share/zoneinfo/%s %s/etc/localtime", region.Timezone, shared.MountRoot))
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set timezone: %w: %s", err, string(output))
	}

	// Step 3: Configure locale from region
	// Uncomment locale in locale.gen
	if ctx.Logger != nil {
		ctx.Logger.Info("Configuring locale: %s", region.Locale)
	}
	// Use fixed-string grep replacement to avoid regex metacharacters in locale names (e.g. ".")
	cmd = exec.Command("sudo", helper, HelperCmdRun, "bash", "-c",
		fmt.Sprintf("sed -i 's/^#%s/%s/' %s/etc/locale.gen", regexp.QuoteMeta(region.Locale), region.Locale, shared.MountRoot))
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to configure locale: %w: %s", err, string(output))
	}

	// Generate locale
	if ctx.Logger != nil {
		ctx.Logger.Info("Generating locale")
	}
	cmd = exec.Command("sudo", helper, HelperCmdRun, ChrootCommand, shared.MountRoot, "locale-gen")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to generate locale: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	// Write locale.conf
	cmd = exec.Command("sudo", helper, HelperCmdRun, "bash", "-c", fmt.Sprintf("echo 'LANG=%s' > %s/etc/locale.conf", region.Locale, shared.MountRoot))
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to write locale.conf: %w: %s", err, string(output))
	}

	// Step 4: Set console keymap from region
	if ctx.Logger != nil {
		ctx.Logger.Info("Setting console keymap to %s", region.Keymap)
	}
	// Write KEYMAP line
	cmd = exec.Command("sudo", helper, HelperCmdRun, "bash", "-c", fmt.Sprintf("echo 'KEYMAP=%s' > %s/etc/vconsole.conf", region.Keymap, shared.MountRoot))
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set keymap: %w: %s", err, string(output))
	}
	// Write FONT line
	cmd = exec.Command("sudo", helper, HelperCmdRun, "bash", "-c", fmt.Sprintf("echo 'FONT=ter-132n' >> %s/etc/vconsole.conf", shared.MountRoot))
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set font: %w: %s", err, string(output))
	}

	// Step 5: Copy mirror list for region
	if ctx.Logger != nil {
		ctx.Logger.Info("Copying mirror list for region %s", region.Code)
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

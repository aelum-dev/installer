package steps

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"aesthetic-os-installer/backend/shared"
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
	cmd := exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf("echo '%s' > /mnt/etc/hostname", ctx.Hostname))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set hostname: %w: %s", err, string(output))
	}

	// Step 2: Set timezone from region
	cmd = exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf("ln -sf /usr/share/zoneinfo/%s /mnt/etc/localtime", region.Timezone))
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set timezone: %w: %s", err, string(output))
	}

	// Step 3: Configure locale from region
	// Uncomment locale in locale.gen
	cmd = exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf("sed -i 's/^#%s/%s/' /mnt/etc/locale.gen", region.Locale, region.Locale))
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to configure locale: %w: %s", err, string(output))
	}

	// Generate locale
	cmd = exec.Command("sudo", helper, "run", "chroot", "/mnt", "locale-gen")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to generate locale: %w: %s", err, string(output))
	}

	// Write locale.conf
	cmd = exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf("echo 'LANG=%s' > /mnt/etc/locale.conf", region.Locale))
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to write locale.conf: %w: %s", err, string(output))
	}

	// Step 4: Set console keymap from region
	cmd = exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf("echo 'KEYMAP=%s' > /mnt/etc/vconsole.conf", region.Keymap))
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set keymap: %w: %s", err, string(output))
	}

	// Step 5: Copy mirror list for region
	if err := copyMirrorList(helper, region.Code); err != nil {
		return err
	}

	return nil
}

// copyMirrorList copies the region-specific mirror list to /etc/pacman.d/mirrorlist.
func copyMirrorList(helper, regionCode string) error {
	// Read from embedded mirrorlist (baked into binary)
	rawContent, err := shared.GetEmbeddedMirrorList(regionCode)
	if err != nil {
		// Fallback to default mirrorlist
		rawContent, err = shared.GetEmbeddedMirrorList("default")
		if err != nil {
			return fmt.Errorf("no mirror list found for region %q and no default available: %w", regionCode, err)
		}
		fmt.Fprintf(os.Stderr, "Warning: no mirror list for %q, using default\n", regionCode)
	}

	// Uncomment mirror lines (strip leading # from #Server = ...)
	content := uncommentMirrors(string(rawContent))

	// Write to target mirrorlist
	cmd := exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf("cat > /mnt/etc/pacman.d/mirrorlist << 'EOF'\n%s\nEOF", content))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to write mirror list: %w: %s", err, strings.TrimSpace(string(output)))
	}

	return nil
}

// uncommentMirrors removes the leading # from #Server = lines in the mirrorlist.
func uncommentMirrors(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "#Server = ") {
			lines[i] = "Server = " + strings.TrimPrefix(line, "#Server = ")
		}
	}
	return strings.Join(lines, "\n")
}

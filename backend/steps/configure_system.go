package steps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	mirrorListsDir := "/usr/share/aesthetic-os/mirrorlists"
	sourcePath := filepath.Join(mirrorListsDir, regionCode+".list")

	// Check if region-specific mirror list exists
	if _, err := os.Stat(sourcePath); err != nil {
		// Fallback to default
		sourcePath = filepath.Join(mirrorListsDir, "default.list")
		if _, err := os.Stat(sourcePath); err != nil {
			return fmt.Errorf("no mirror list found for region %q and no default available", regionCode)
		}
		fmt.Fprintf(os.Stderr, "Warning: no mirror list for %q, using default\n", regionCode)
	}

	// Read source mirror list
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read mirror list: %w", err)
	}

	// Write to target mirrorlist
	cmd := exec.Command("sudo", helper, "run", "bash", "-c", fmt.Sprintf("cat > /mnt/etc/pacman.d/mirrorlist << 'EOF'\n%s\nEOF", string(content)))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to write mirror list: %w: %s", err, strings.TrimSpace(string(output)))
	}

	return nil
}

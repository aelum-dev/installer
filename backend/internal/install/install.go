package steps

import (
	"bufio"
	_ "embed"
	"fmt"
	"os/exec"
	"strings"
)

//go:embed packages.x86_64
var packagesFile string

// Install installs the Arch Linux base system via pacstrap.
// Packages are read from the embedded packages.x86_64 file.
func Install(ctx *InstallContext) error {
	if err := validateMountedSystem(ctx); err != nil {
		return err
	}

	packages, err := parsePackages(packagesFile)
	if err != nil {
		return fmt.Errorf("failed to parse packages: %w", err)
	}

	if len(packages) == 0 {
		return fmt.Errorf("no packages to install")
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Installing %d packages via pacstrap", len(packages))
	}

	args := []string{PacstrapMountPoint}
	args = append(args, packages...)
	cmd := exec.Command("sudo", append([]string{ctx.Helper, HelperCmdRun, "pacstrap"}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pacstrap failed: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}
	return nil
}

// parsePackages parses a packages file string (one per line, # comments and blank lines ignored).
func parsePackages(content string) ([]string, error) {
	var packages []string
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		packages = append(packages, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return packages, nil
}

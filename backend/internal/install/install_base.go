package install

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
)

// basePackages are the essential system packages installed during the base
// system step. Kept minimal so this step finishes quickly and the progress
// bar moves smoothly.
var basePackages = []string{
	// Base system
	"base",
	"linux-lts",
	"linux-firmware",
	"mkinitcpio",
	"arch-install-scripts",
	"mkinitcpio-archiso",
	"qemu-guest-agent",
	"edk2-shell",
	"memtest86+-efi",

	// Utilities
	"util-linux",
	"gptfdisk",
	"parted",
	"dosfstools",
	"e2fsprogs",
	"btrfs-progs",
	"pv",

	// System
	"sudo",
	"networkmanager",

	// Editor
	"vim",
	"nano",
}

// Install installs the Arch Linux base system via pacstrap.
func Install(ctx *InstallContext) error {
	return installPackages(ctx, basePackages, "base system")
}

// installPackages runs pacstrap with the given package list.
// Streams stdout to parse real progress from pacstrap's (N/M) output lines.
func installPackages(ctx *InstallContext, packages []string, description string) error {
	if err := validateMountedSystem(ctx); err != nil {
		return err
	}

	if len(packages) == 0 {
		return fmt.Errorf("no packages to install")
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Installing %d %s packages via pacstrap", len(packages), description)
	}

	args := []string{PacstrapMountPoint}
	args = append(args, packages...)
	cmd := exec.Command("sudo", append([]string{ctx.Helper, HelperCmdRun, "pacstrap"}, args...)...)

	// Stream stdout to parse real progress from pacstrap's (N/M) lines.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start pacstrap: %w", err)
	}

	// Track progress across pacstrap's multiple phases (checking, installing, hooks).
	progress := newPacstrapProgressTracker()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if ctx.Logger != nil {
			ctx.Logger.Command(line)
		}

		// Parse progress: pacstrap outputs lines like "(45/85) installing packageX"
		if n, m := parsePacstrapProgressPair(line); n >= 0 && m >= 0 {
			pct := progress.update(n, m)
			if ctx.ProgressCallback != nil {
				ctx.ProgressCallback(pct)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading pacstrap output: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("pacstrap failed: %w", err)
	}
	return nil
}

// pacstrapProgressRegex matches pacstrap's (N/M) progress lines.
// Examples:
//   "(45/85) installing packageX"
//   "(10/10) checking keys in keyring"
//   "( 1/85) installing package1"
var pacstrapProgressRegex = regexp.MustCompile(`^\(\s*(\d+)\s*/\s*(\d+)\s*\)`)

// parsePacstrapProgressPair extracts the (N, M) pair from a pacstrap progress line.
// Returns (-1, -1) if the line doesn't contain progress information.
func parsePacstrapProgressPair(line string) (int, int) {
	matches := pacstrapProgressRegex.FindStringSubmatch(line)
	if len(matches) != 3 {
		return -1, -1
	}

	var n, m int
	fmt.Sscanf(matches[1], "%d", &n)
	fmt.Sscanf(matches[2], "%d", &m)

	if m == 0 {
		return -1, -1
	}

	return n, m
}

// pacstrapProgressTracker tracks progress across pacstrap's multiple phases.
// Pacstrap outputs progress in phases (checking keys, installing packages, hooks),
// each with their own (N/M) counter. This tracker sums denominators across phases
// to calculate overall progress.
type pacstrapProgressTracker struct {
	completedPhasesTotal int
	currentPhaseTotal    int
	currentPhaseCurrent  int
}

func newPacstrapProgressTracker() *pacstrapProgressTracker {
	return &pacstrapProgressTracker{}
}

// update processes a new (N, M) progress line and returns the overall percentage.
func (t *pacstrapProgressTracker) update(n, m int) int {
	// Detect phase boundary: M changed means we're in a new phase.
	if m != t.currentPhaseTotal {
		if t.currentPhaseTotal > 0 {
			t.completedPhasesTotal += t.currentPhaseTotal
		}
		t.currentPhaseTotal = m
		t.currentPhaseCurrent = 0
	}
	t.currentPhaseCurrent = n

	overallTotal := t.completedPhasesTotal + t.currentPhaseTotal
	overallCurrent := t.completedPhasesTotal + t.currentPhaseCurrent

	if overallTotal == 0 {
		return 0
	}
	return int(float64(overallCurrent) / float64(overallTotal) * 100)
}

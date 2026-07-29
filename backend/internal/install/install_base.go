package install

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
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

		// Process all line types: Packages count, downloading, verifying,
		// installing, and (N/M) hook progress.
		pct := progress.processLine(line)
		if ctx.ProgressCallback != nil {
			ctx.ProgressCallback(pct)
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
var pacstrapProgressRegex = regexp.MustCompile(`^\(\s*(\d+)\s*/\s*(\d+)\s*\)`)

// packagesRegex matches "Packages (N)" to get total package count.
// Pacstrap outputs this as ":: Packages (190) ..." so we match anywhere in the line.
var packagesRegex = regexp.MustCompile(`Packages \((\d+)\)`)

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

// parsePackagesCount extracts the package count from "Packages (N)" output.
// Returns 0 if the line doesn't match.
func parsePackagesCount(line string) int {
	matches := packagesRegex.FindStringSubmatch(line)
	if len(matches) != 2 {
		return 0
	}
	var count int
	fmt.Sscanf(matches[1], "%d", &count)
	return count
}

// pacstrapProgressTracker tracks progress across all pacstrap phases using
// actual line counts rather than relying solely on (N/M) counters.
//
// Pacstrap's output phases (in order):
//   1. Pre-download: resolving deps, package list (Packages (N))
//   2. Downloading: "<pkg> downloading..." lines — one per package
//   3. Verification: "checking keyring...", "loading package files...", etc.
//   4. Installing: "installing <pkg>..." lines — one per package
//   5. Post-hooks: "( N/M) ..." lines with explicit counters
//
// Progress = (downloaded + verified + installed + hookCurrent) /
//            (totalPackages + verifySteps + totalPackages + hookDenominator) * 100
//
// Key design: when a hook phase ends (M changes), we credit currentPhaseCurrent
// (the highest N actually observed) rather than currentPhaseTotal (M), because
// pacstrap may jump to the next phase before outputting all N/M lines.
type pacstrapProgressTracker struct {
	totalPackages int // from "Packages (N)"

	// Phase counters — each represents completed work units.
	downloaded int // "downloading..." lines
	verified   int // verification steps (checking keyring, integrity, etc.)
	installed  int // "installing <pkg>..." lines

	// Hook (N/M) tracking for post-transaction phase.
	// The hook denominator grows as new hook phases appear (M changes).
	hookCompletedPhases int
	hookCurrentTotal    int
	hookCurrentCurrent  int
}

func newPacstrapProgressTracker() *pacstrapProgressTracker {
	return &pacstrapProgressTracker{}
}

// processLine handles all pacstrap output line types and returns the current
// overall progress percentage (0-100).
func (t *pacstrapProgressTracker) processLine(line string) int {
	// Phase 1: "Packages (190)" — record total package count.
	if count := parsePackagesCount(line); count > 0 {
		t.totalPackages = count
		return t.calculatePercent()
	}

	// Phase 2: "<pkg> downloading..." — count downloads.
	if strings.HasSuffix(line, "downloading...") {
		t.downloaded++
		return t.calculatePercent()
	}

	// Phase 3: Verification steps.
	if isVerifyLine(line) {
		t.verified++
		return t.calculatePercent()
	}

	// Phase 4: "installing <pkg>..." — count installs.
	if strings.HasPrefix(strings.TrimSpace(line), "installing ") {
		t.installed++
		return t.calculatePercent()
	}

	// Phase 5: "( N/M) ..." — hook progress.
	if n, m := parsePacstrapProgressPair(line); n >= 0 && m >= 0 {
		return t.updateHook(n, m)
	}

	return t.calculatePercent()
}

// isVerifyLine returns true for pacstrap verification phase lines.
func isVerifyLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "checking ") ||
		strings.HasPrefix(trimmed, "loading ") ||
		strings.HasPrefix(trimmed, ":: Processing package changes")
}

// updateHook processes a hook (N/M) line and returns overall progress.
func (t *pacstrapProgressTracker) updateHook(n, m int) int {
	// Detect phase boundary: M changed means we're in a new hook phase.
	if m != t.hookCurrentTotal {
		if t.hookCurrentTotal > 0 {
			// Phase ended — credit the highest N actually observed, not M.
			t.hookCompletedPhases += t.hookCurrentCurrent
		}
		t.hookCurrentTotal = m
		t.hookCurrentCurrent = 0
	}

	// Track the highest N seen in this phase.
	if n > t.hookCurrentCurrent {
		t.hookCurrentCurrent = n
	}

	return t.calculatePercent()
}

// calculatePercent computes overall progress as a percentage (0-100).
//
// Work model:
//   - Download: totalPackages units
//   - Verification: t.verified units (each verify line = 1 unit)
//   - Install: totalPackages units
//   - Hooks: hookCompletedPhases + hookCurrentTotal (grows with new phases)
func (t *pacstrapProgressTracker) calculatePercent() int {
	// Hook denominator: sum of all seen phase totals (grows as new phases appear).
	hookDenominator := t.hookCompletedPhases + t.hookCurrentTotal
	hookCurrent := t.hookCompletedPhases + t.hookCurrentCurrent

	// totalPackages * 2 covers both download and install phases.
	totalUnits := (t.totalPackages * 2) + t.verified + hookDenominator
	currentUnits := t.downloaded + t.verified + t.installed + hookCurrent

	if totalUnits == 0 {
		return 0
	}

	pct := int(float64(currentUnits) / float64(totalUnits) * 100)
	if pct > 100 {
		pct = 100
	}
	if pct < 0 {
		pct = 0
	}
	return pct
}

// update is kept for backward compatibility — delegates to processLine
// for (N/M) lines. Non-hook lines should use processLine directly.
func (t *pacstrapProgressTracker) update(n, m int) int {
	return t.updateHook(n, m)
}

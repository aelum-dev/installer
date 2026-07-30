package install

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// pacstrap line-type constants — matched against pacstrap stdout to track progress.
const (
	pacstrapSuffixDownloading       = "downloading..."
	pacstrapPrefixInstalling        = "installing "
	pacstrapPrefixChecking          = "checking "
	pacstrapPrefixLoading           = "loading "
	pacstrapPrefixProcessingChanges = ":: Processing package changes"
)

// pacstrapProgressRegex matches pacstrap's (N/M) progress lines.
var pacstrapProgressRegex = regexp.MustCompile(`^\(\s*(\d+)\s*/\s*(\d+)\s*\)`)

// packagesRegex matches "Packages (N)" to get total package count.
// Pacstrap outputs this as ":: Packages (190) ..." so we match anywhere in the line.
var packagesRegex = regexp.MustCompile(`Packages \((\d+)\)`)

// installPackages runs pacstrap with the given package list.
// Streams stdout to parse real progress from pacstrap's output lines.
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

	// Stream stdout to parse real progress from pacstrap's lines.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start pacstrap: %w", err)
	}

	// Track progress across pacstrap's multiple phases.
	progress := newPacstrapProgressTracker()

	// Large buffer for the "Packages (N) pkg1 pkg2 ..." line which can exceed
	// the default 64KB scanner buffer when installing many packages.
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
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

// parsePacstrapProgressPair extracts the (N, M) pair from a pacstrap progress line.
// Returns (-1, -1) if the line doesn't contain progress information.
func parsePacstrapProgressPair(line string) (int, int) {
	matches := pacstrapProgressRegex.FindStringSubmatch(line)
	if len(matches) != 3 {
		return -1, -1
	}

	n, err := strconv.Atoi(matches[1])
	if err != nil {
		return -1, -1
	}
	m, err := strconv.Atoi(matches[2])
	if err != nil {
		return -1, -1
	}

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
	count, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
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
// Work model:
//   - Download: totalPackages units
//   - Verification: verified count (each verify line = 1 unit)
//   - Install: totalPackages units
//   - Hooks: hookCompletedPhases + hookCurrentTotal (grows with new phases)
//
// Progress = (downloaded + verified + installed + hookCurrent) /
//            (2*totalPackages + verified + hookDenominator) * 100
//
// The factor of 2 assumes download and install phases consume equal work units.
// This is approximate — actual time depends on package sizes and disk speed —
// but line-count-based tracking provides smooth, monotonic progress regardless.
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
	// Phase 1: "Packages (N)" — record total package count.
	if count := parsePackagesCount(line); count > 0 {
		t.totalPackages = count
		return t.calculatePercent()
	}

	// Phase 2: "<pkg> downloading..." — count downloads.
	if strings.HasSuffix(line, pacstrapSuffixDownloading) {
		t.downloaded++
		return t.calculatePercent()
	}

	// Phase 3: Verification steps.
	if isVerifyLine(line) {
		t.verified++
		return t.calculatePercent()
	}

	// Phase 4: "installing <pkg>..." — count installs.
	if strings.HasPrefix(strings.TrimSpace(line), pacstrapPrefixInstalling) {
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
	return strings.HasPrefix(trimmed, pacstrapPrefixChecking) ||
		strings.HasPrefix(trimmed, pacstrapPrefixLoading) ||
		strings.HasPrefix(trimmed, pacstrapPrefixProcessingChanges)
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

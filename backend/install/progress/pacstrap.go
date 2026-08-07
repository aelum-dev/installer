package progress

import (
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
var packagesRegex = regexp.MustCompile(`Packages \((\d+)\)`)

// PacstrapProgressTracker tracks progress across all pacstrap phases using
// actual line counts rather than relying solely on (N/M) counters.
//
// Pacstrap's output phases (in order):
//  1. Pre-download: resolving deps, package list (Packages (N))
//  2. Downloading: "<pkg> downloading..." lines — one per package
//  3. Verification: "checking keyring...", "loading package files...", etc.
//  4. Installing: "installing <pkg>..." lines — one per package
//  5. Post-hooks: "( N/M) ..." lines with explicit counters
//
// Work model:
//   - Download: totalPackages units
//   - Verification: verified count (each verify line = 1 unit)
//   - Install: totalPackages units
//   - Hooks: hookCompletedPhases + hookCurrentTotal (grows with new phases)
//
// Progress = (downloaded + verified + installed + hookCurrent) /
//
//	(2*totalPackages + verified + hookDenominator) * 100
type PacstrapProgressTracker struct {
	totalPackages int // from "Packages (N)"

	// Phase counters — each represents completed work units.
	downloaded int // "downloading..." lines
	verified   int // verification steps (checking keyring, integrity, etc.)
	installed  int // "installing <pkg>..." lines

	// Hook (N/M) tracking for post-transaction phase.
	hookCompletedPhases int
	hookCurrentTotal    int
	hookCurrentCurrent  int
}

// NewPacstrapProgressTracker creates a new progress tracker.
func NewPacstrapProgressTracker() *PacstrapProgressTracker {
	return &PacstrapProgressTracker{}
}

// TotalPackages returns the total package count from the "Packages (N)" line.
func (t *PacstrapProgressTracker) TotalPackages() int {
	return t.totalPackages
}

// SetTotalPackages sets the total package count (used by tests).
func (t *PacstrapProgressTracker) SetTotalPackages(n int) {
	t.totalPackages = n
}

// ProcessLine handles all pacstrap output line types and returns the current
// overall progress percentage (0-100).
func (t *PacstrapProgressTracker) ProcessLine(line string) int {
	// Phase 1: "Packages (N)" — record total package count.
	if count := parsePackagesCount(line); count > 0 {
		t.totalPackages = count
		return t.CalculatePercent()
	}

	// Phase 2: "<pkg> downloading..." — count downloads.
	if strings.HasSuffix(line, pacstrapSuffixDownloading) {
		t.downloaded++
		return t.CalculatePercent()
	}

	// Phase 3: Verification steps.
	if isVerifyLine(line) {
		t.verified++
		return t.CalculatePercent()
	}

	// Phase 4: "installing <pkg>..." — count installs.
	if strings.HasPrefix(strings.TrimSpace(line), pacstrapPrefixInstalling) {
		t.installed++
		return t.CalculatePercent()
	}

	// Phase 5: "( N/M) ..." — hook progress.
	if n, m := parsePacstrapProgressPair(line); n >= 0 && m >= 0 {
		return t.UpdateHook(n, m)
	}

	return t.CalculatePercent()
}

// parsePacstrapProgressPair extracts the (N, M) pair from a pacstrap progress line.
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

// isVerifyLine returns true for pacstrap verification phase lines.
func isVerifyLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, pacstrapPrefixChecking) ||
		strings.HasPrefix(trimmed, pacstrapPrefixLoading) ||
		strings.HasPrefix(trimmed, pacstrapPrefixProcessingChanges)
}

// UpdateHook processes a hook (N/M) line and returns overall progress.
func (t *PacstrapProgressTracker) UpdateHook(n, m int) int {
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

	return t.CalculatePercent()
}

// CalculatePercent computes overall progress as a percentage (0-100).
func (t *PacstrapProgressTracker) CalculatePercent() int {
	hookDenominator := t.hookCompletedPhases + t.hookCurrentTotal
	hookCurrent := t.hookCompletedPhases + t.hookCurrentCurrent

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

// --- Step progress tracker --------------------------------------------------

// StepProgressTracker advances a percentage from 0 to 100 over a number of
// ticks. Useful for steps that don't produce their own progress output.
type StepProgressTracker struct {
	ticksElapsed int
	totalTicks   int
}

// NewStepProgressTracker creates a tracker that advances 0→100 over totalTicks ticks.
func NewStepProgressTracker(totalTicks int) *StepProgressTracker {
	return &StepProgressTracker{totalTicks: totalTicks}
}

// Advance advances the tracker by one tick and returns the current percentage.
func (t *StepProgressTracker) Advance() int {
	if t.totalTicks <= 0 {
		return 100
	}
	if t.ticksElapsed < t.totalTicks {
		t.ticksElapsed++
	}
	return int(float64(t.ticksElapsed) / float64(t.totalTicks) * 100)
}

package progress

import (
	"regexp"
	"strconv"
	"strings"

	"aelum-installer/backend/shared"
)

// pacstrap line-type constants — matched against pacstrap stdout to track progress.
const (
	pacstrapSuffixDownloading = "downloading..."
	pacstrapPrefixInstalling  = "installing "
)

// pacstrapProgressRegex matches pacstrap's (N/M) progress lines.
var pacstrapProgressRegex = regexp.MustCompile(`^\(\s*(\d+)\s*/\s*(\d+)\s*\)`)

// packagesRegex matches "Packages (N)" to get total package count.
var packagesRegex = regexp.MustCompile(`Packages \((\d+)\)`)

// PacstrapProgressTracker tracks progress across all pacstrap phases.
//
// Pacstrap's output phases (in order):
//  1. Pre-download: resolving deps, package list (Packages (N))
//  2. Downloading: "<pkg> downloading..." lines — one per package
//  3. Verification: "checking keyring...", "loading package files...", etc.
//  4. Installing: "installing <pkg>..." lines — one per package
//  5. Post-hooks: "( N/M) ..." lines with explicit counters
//
// Rather than weighing every line equally — which front-loads the bar, because
// an offline install finishes the entire download phase in under a second —
// each phase is mapped to its own band of the step:
//
//	download: 0            → shared.PacstrapDownloadBandEnd
//	install:  download end → shared.PacstrapInstallBandEnd
//	hooks:    install end  → 100
//
// The install band tops out below 100 so the post-transaction hooks (initramfs
// rebuild, font/icon caches) — which run after the last package is extracted —
// always have room to move the bar. The reported percentage is clamped to be
// monotonically non-decreasing.
type PacstrapProgressTracker struct {
	totalPackages int // from "Packages (N)"

	// Phase counters — each represents completed work units.
	downloaded int // "downloading..." lines
	installed  int // "installing <pkg>..." lines

	// Hook (N/M) tracking for post-transaction phase.
	hookCompletedPhases int
	hookCurrentTotal    int
	hookCurrentCurrent  int

	// maxPercent holds the highest percentage ever reported so the tracker can
	// guarantee forward-only progress (see CalculatePercent).
	maxPercent int
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
	trimmed := strings.TrimSpace(line)

	// Phase 1: "Packages (N)" — record total package count.
	if count := parsePackagesCount(line); count > 0 {
		t.totalPackages = count
		return t.CalculatePercent()
	}

	// Phase 2: "<pkg> downloading..." — count downloads.
	if strings.HasSuffix(trimmed, pacstrapSuffixDownloading) {
		t.downloaded++
		return t.CalculatePercent()
	}

	// Phase 3: "installing <pkg>..." — count installs.
	if strings.HasPrefix(trimmed, pacstrapPrefixInstalling) {
		t.installed++
		return t.CalculatePercent()
	}

	// Phase 4: "( N/M) ..." — post-transaction hook progress.
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
	pct := t.rawPercent()
	if pct > 100 {
		pct = 100
	}
	if pct < 0 {
		pct = 0
	}

	// Guarantee forward-only progress. A new hook phase enlarges the
	// denominator before the numerator catches up, but the displayed bar must
	// never move backwards.
	if pct > t.maxPercent {
		t.maxPercent = pct
	}
	return t.maxPercent
}

// rawPercent returns the unclamped percentage for the furthest phase observed.
// Later phases always map to a higher band than earlier ones, so the result is
// naturally monotonic across phase transitions.
func (t *PacstrapProgressTracker) rawPercent() int {
	switch {
	case t.hooksStarted():
		return t.hookPercent()
	case t.installed > 0:
		return t.installPercent()
	default:
		return t.downloadPercent()
	}
}

// downloadPercent maps download lines onto the [0, PacstrapDownloadBandEnd] band.
func (t *PacstrapProgressTracker) downloadPercent() int {
	if t.totalPackages <= 0 {
		return 0
	}
	if t.downloaded >= t.totalPackages {
		return shared.PacstrapDownloadBandEnd
	}
	return t.downloaded * shared.PacstrapDownloadBandEnd / t.totalPackages
}

// installPercent maps install lines onto the
// [PacstrapDownloadBandEnd, PacstrapInstallBandEnd] band.
func (t *PacstrapProgressTracker) installPercent() int {
	if t.totalPackages <= 0 {
		return shared.PacstrapDownloadBandEnd
	}
	done := t.installed
	if done > t.totalPackages {
		done = t.totalPackages
	}
	span := shared.PacstrapInstallBandEnd - shared.PacstrapDownloadBandEnd
	return shared.PacstrapDownloadBandEnd + done*span/t.totalPackages
}

// hooksStarted reports whether any post-transaction hook line has been seen.
func (t *PacstrapProgressTracker) hooksStarted() bool {
	return t.hookCurrentTotal > 0 || t.hookCompletedPhases > 0
}

// hookPercent maps post-transaction hook progress onto the
// [PacstrapInstallBandEnd, 100] band.
func (t *PacstrapProgressTracker) hookPercent() int {
	denom := t.hookCompletedPhases + t.hookCurrentTotal
	if denom <= 0 {
		return shared.PacstrapInstallBandEnd
	}
	num := t.hookCompletedPhases + t.hookCurrentCurrent
	span := 100 - shared.PacstrapInstallBandEnd
	return shared.PacstrapInstallBandEnd + num*span/denom
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

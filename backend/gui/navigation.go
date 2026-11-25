package gui

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"aelum-installer/backend/shared"
)

// PageID represents a page in the installer wizard.
// Defined as a struct so Wails generates proper TypeScript model bindings.
type PageID struct {
	Value int
}

var (
	PageWelcome       = PageID{Value: 0}
	PageUserAccount   = PageID{Value: 1}
	PageDiskSelection = PageID{Value: 2}
	PageConfirmation  = PageID{Value: 3}
	PageInstallation  = PageID{Value: 4}
	PageFinish        = PageID{Value: 5}
)

// App struct
type App struct {
	ctx              context.Context
	currentPage      PageID
	startPage        PageID
	dryRun           bool
	userAccount      shared.UserAccountData
	diskSelection    shared.DiskSelectionData
	hostname         string
	selectedLanguage string
	locale           string        // system locale implied by selectedLanguage
	region           shared.Region // location: keymap, mirrors, default clock
	// timezone is the installed system's clock (an IANA zone name). It defaults to
	// the region's clock but is a choice of its own, so the Welcome page can set it
	// independently of the location.
	timezone     string
	uiLocale     string // UI language (separate from installed system locale)
	fileSystem   string // root filesystem: "ext4" or "btrfs"
	manualMode   bool   // manual partitioning enabled
	manualLayout shared.ManualLayout

	// luks enables optional LUKS2 encryption of the root partition. luksPassphrase
	// is the passphrase held in memory (zeroed buffer) from entry until the CLI
	// consumes it — mirroring the account-password handling. Never logged.
	luks           bool
	luksPassphrase []byte

	// lvm places the root (and, for ext4, home) on logical volumes. It is
	// orthogonal to luks: LUKS always sits below LVM.
	lvm bool

	// installMu guards the state of an installation run below. The
	// Wails-bound methods arrive on a goroutine per call and the
	// goroutine that reaps the CLI clears the same state while a close
	// request, a GetInstallationRunning poll or a retried
	// StartInstallation can be reading it.
	installMu sync.Mutex
	// installationRunning is true while the CLI subprocess of an install
	// runs — or is being started, since StartInstallation reserves it
	// before spawning. Under installMu, so a second call cannot get past
	// the guard and start a second root CLI against the same disk.
	installationRunning bool
	// installCmd is the currently running CLI subprocess, or nil.
	installCmd *exec.Cmd
	// installCancel cancels the context for the current CLI subprocess.
	installCancel func()
	// installDone is closed when the CLI process has exited and cleanup is
	// complete, then set to nil. WaitForCleanup takes the channel out
	// under this same lock and waits on it outside the lock, because the
	// close itself needs the lock to happen.
	installDone chan struct{}
	// installStartedAt is when the CLI subprocess started; the completion
	// event reports the elapsed time so the finish page can show it.
	installStartedAt time.Time
	// installCancelled records that this GUI asked the running install to stop.
	// A cancelled CLI exits with the signal code, and without this flag the GUI
	// would report the stop as a failure of the installer's own.
	installCancelled atomic.Bool
}

// GetCurrentPage returns the current page index (0-based).
func (a *App) GetCurrentPage() int {
	return a.currentPage.Value
}

// SetCurrentPage sets the current page after validation.
func (a *App) SetCurrentPage(pageID PageID) error {
	if pageID.Value < PageWelcome.Value || pageID.Value > PageFinish.Value {
		return fmt.Errorf("invalid page ID: %d", pageID.Value)
	}
	a.currentPage = pageID
	return nil
}

// SetHostname sets the system hostname.
func (a *App) SetHostname(hostname string) {
	a.hostname = hostname
}

// GetHostname returns the system hostname.
func (a *App) GetHostname() string {
	return a.hostname
}

// GetUILocale returns the current UI locale.
func (a *App) GetUILocale() string {
	return a.uiLocale
}

// SetUILocale sets the UI locale.
func (a *App) SetUILocale(locale string) {
	a.uiLocale = locale
}

// SetLuks enables or disables LUKS2 root encryption. Disabling clears the
// held passphrase so the install proceeds unencrypted.
func (a *App) SetLuks(enabled bool) {
	a.luks = enabled
	if !enabled {
		a.clearLuksPassphrase()
	}
}

// SetLuksPassphrase stores the LUKS passphrase in memory (zeroed buffer) for
// the CLI to consume on stdin. It is never logged or passed as a flag.
func (a *App) SetLuksPassphrase(passphrase string) {
	a.clearLuksPassphrase()
	a.luksPassphrase = make([]byte, len(passphrase))
	copy(a.luksPassphrase, []byte(passphrase))
}

// SetLvm enables or disables LVM for the root filesystem. It is independent of
// SetLuks: enabling both encrypts the single physical volume.
func (a *App) SetLvm(enabled bool) {
	a.lvm = enabled
}

// clearLuksPassphrase zeroes and releases the held passphrase.
func (a *App) clearLuksPassphrase() {
	for k := range a.luksPassphrase {
		a.luksPassphrase[k] = 0
	}
	a.luksPassphrase = nil
}

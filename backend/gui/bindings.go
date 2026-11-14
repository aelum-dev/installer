package gui

import (
	"fmt"
	"os/exec"
	"strings"

	"aelum-installer/backend/install/installdesktop"
	"aelum-installer/backend/install/partition"
	"aelum-installer/backend/shared"
)

// GetDisks returns a list of available block devices. Wails binding.
func (a *App) GetDisks() ([]shared.DiskInfo, error) {
	return shared.GetDisks()
}

// GetDiskPartitions returns detailed partition information for a disk. Wails binding.
func (a *App) GetDiskPartitions(diskPath string) ([]shared.PartitionEntry, error) {
	return shared.GetDiskPartitions(diskPath)
}

// GetLanguages returns the list of available languages. Wails binding.
// A language is only how the system reads and writes text — it is not a place.
func (a *App) GetLanguages() []shared.Language {
	return shared.GetLanguages()
}

// GetRegions returns the list of locations the installer can offer: the countries
// Arch has mirrors for, plus the worldwide list. Wails binding.
func (a *App) GetRegions() []shared.Region {
	return shared.GetRegions()
}

// GetSessionInputMethods returns the input methods the session for this language
// and location will offer, named in words — "English (US)", "Ukrainian", "Mozc" —
// in the order the session gets them. The Confirmation page shows that instead of
// a framework name: the fcitx5 core ships on every install, so naming it answers
// nothing, while naming the keyboards says what the person will actually have and
// what the switch key moves between. Wails binding.
func (a *App) GetSessionInputMethods(language string, region string) []string {
	return installdesktop.SessionInputMethodNames(language, region)
}

// GetSelectedLanguage returns the currently selected language code.
func (a *App) GetSelectedLanguage() string {
	return a.selectedLanguage
}

// SetSelectedLanguage sets the selected language and pre-fills the location from
// its country. That is the whole of the link between the two: most people's
// language and country do agree, so they never have to touch the location, and
// the ones whose do not say so directly.
func (a *App) SetSelectedLanguage(code string) error {
	a.selectedLanguage = code
	a.locale = shared.LocaleForLanguage(code)
	a.region = shared.RegionForLanguage(code)
	return nil
}

// GetSelectedRegion returns the code of the currently selected location.
func (a *App) GetSelectedRegion() string {
	return a.region.Code
}

// SetSelectedRegion sets the location: the installed system's clock, console
// keymap and Arch mirrors. It is independent of the language.
func (a *App) SetSelectedRegion(code string) error {
	region := shared.GetRegion(code)
	if region.Code == "" {
		return fmt.Errorf("unknown region: %s", code)
	}
	a.region = region
	return nil
}

// GetFileSystem returns the currently selected filesystem type.
func (a *App) GetFileSystem() string {
	return a.fileSystem
}

// GetManualMode reports whether manual partitioning is active.
func (a *App) GetManualMode() bool {
	return a.manualMode
}

// GetManualLayout returns the manual partition layout for a disk, computed from
// the disk's current partitions. Wails binding.
func (a *App) GetManualLayout(diskID string) (shared.ManualLayout, error) {
	layout, err := partition.ComputeLayout(diskID)
	if err != nil {
		return layout, err
	}
	// Carry the current storage/encryption choices so validation sees the right
	// System-partition floor before the page is saved.
	layout.Luks = a.luks
	layout.Lvm = a.lvm
	return layout, nil
}

// ValidateManualLayout validates a manual partition layout without mutating
// any state. Wails binding.
//
// It uses ValidateFeasible, which also simulates placement against the disk's
// current geometry so a request that only fits after fragmentation is rejected
// here rather than during installation. If the geometry cannot be read (e.g.
// unprivileged), it falls back to the coarse Validate result.
func (a *App) ValidateManualLayout(layout shared.ManualLayout) error {
	// The layout comes from the page, which may not include the toggles the user
	// changed after loading it. Make the current choices authoritative so the
	// LUKS/LVM System floor is enforced at Next and a disabled toggle is not
	// resurrected by a stale layout field.
	layout.Luks = a.luks
	layout.Lvm = a.lvm
	return partition.ValidateFeasible(layout)
}

// SaveManualLayout stores the manual layout and enables manual mode. Wails
// binding.
func (a *App) SaveManualLayout(layout shared.ManualLayout) {
	// The app toggles are authoritative; a layout loaded before the user changed
	// them may still carry the old values.
	layout.Luks = a.luks
	layout.Lvm = a.lvm
	a.manualLayout = layout
	a.manualMode = true
}

// SetWallpaper switches the running hyprpaper wallpaper to match the given
// theme ('light' or 'dark'). It calls hyprctl IPC directly — no wrapper script —
// so it works with current hyprland 0.56+, where the old preload/reload/unload
// requests have been removed.
//
// An empty monitor target is used so the wallpaper applies to every output
// (the same fallback the live ISO's hyprpaper.conf relies on).
//
// It must run as the Hyprland session user (the GUI already does), never as
// root: hyprctl talks to the user's compositor over its wayland socket.
func (a *App) SetWallpaper(theme string) error {
	path := shared.LiveIsoBgLight
	if theme == "dark" {
		path = shared.LiveIsoBgDark
	}

	// hyprctl hyprpaper wallpaper '<monitor>,<path>,<fit_mode>' — monitor may be
	// empty to target all outputs; fit_mode optional (cover here).
	arg := fmt.Sprintf(",%s,cover", path)
	cmd := exec.Command("hyprctl", "hyprpaper", "wallpaper", arg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to switch wallpaper to %s mode: %w: %s", theme, err, strings.TrimSpace(string(output)))
	}
	return nil
}

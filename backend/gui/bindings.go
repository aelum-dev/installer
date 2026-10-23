package gui

import (
	"fmt"
	"os/exec"
	"strings"

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

// GetLanguages returns the list of available languages/regions. Wails binding.
func (a *App) GetLanguages() []shared.Language {
	return shared.GetLanguages()
}

// GetSelectedLanguage returns the currently selected language/region code.
func (a *App) GetSelectedLanguage() string {
	return a.selectedLanguage
}

// SetSelectedLanguage sets the selected language/region.
func (a *App) SetSelectedLanguage(code string) error {
	a.selectedLanguage = code
	a.region = shared.GetRegion(code)
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
	layout.Lvm = a.lvm
	layout.Luks = a.luks
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
	// LVM/LUKS System floor is enforced at Next and a disabled toggle is not
	// resurrected by a stale layout field.
	layout.Lvm = a.lvm
	layout.Luks = a.luks
	return partition.ValidateFeasible(layout)
}

// SaveManualLayout stores the manual layout and enables manual mode. Wails
// binding.
func (a *App) SaveManualLayout(layout shared.ManualLayout) {
	// The app toggles are authoritative; a layout loaded before the user changed
	// them may still carry the old values.
	layout.Lvm = a.lvm
	layout.Luks = a.luks
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

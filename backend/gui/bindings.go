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

// GetManualMode reports whether manual (dualboot-safe) partitioning is active.
func (a *App) GetManualMode() bool {
	return a.manualMode
}

// GetManualLayout returns the manual partition layout for a disk, computed from
// the disk's current partitions. Wails binding.
func (a *App) GetManualLayout(diskID string) (shared.ManualLayout, error) {
	return partition.ComputeLayout(diskID)
}

// ValidateManualLayout validates a manual partition layout without mutating
// any state. Wails binding.
func (a *App) ValidateManualLayout(layout shared.ManualLayout) error {
	return partition.Validate(layout)
}

// SaveManualLayout stores the manual layout and enables manual mode. Wails
// binding.
func (a *App) SaveManualLayout(layout shared.ManualLayout) {
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

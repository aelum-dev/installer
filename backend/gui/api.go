package gui

import (
	"fmt"

	"aesthetic-os-installer/backend/shared"
)

// --- GUI methods (delegate to shared package) ---

// GetDisks returns a list of available block devices (disks) for installation.
// Wails binding: frontend calls this.
func (a *App) GetDisks() ([]shared.DiskInfo, error) {
	return shared.GetDisks()
}

// GetLanguages returns the list of available languages/regions.
// Wails binding: frontend calls this.
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
	if shared.FindLanguage(code) == nil {
		return fmt.Errorf("unknown language code: %s", code)
	}
	return nil
}

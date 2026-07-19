package app

import (
	"fmt"

	"aesthetic-os-installer/backend/shared"
)

// --- GUI methods (delegate to shared package) ---

// GetLanguages returns the list of available languages.
// Wails binding: frontend calls this.
func (a *App) GetLanguages() []shared.Language {
	return shared.GetLanguages()
}

// GetSelectedLanguage returns the currently selected language code.
func (a *App) GetSelectedLanguage() string {
	return a.selectedLanguage
}

// SetSelectedLanguage sets the selected language.
func (a *App) SetSelectedLanguage(code string) error {
	a.selectedLanguage = code
	if shared.FindLanguage(code) == nil {
		return fmt.Errorf("unknown language code: %s", code)
	}
	return nil
}

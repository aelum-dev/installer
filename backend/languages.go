package app

import "fmt"

// GetLanguages returns the list of available languages.
func (a *App) GetLanguages() []Language {
	return []Language{
		{Code: "en", Name: "English", Locale: "en_US.UTF-8"},
		{Code: "de", Name: "Deutsch", Locale: "de_DE.UTF-8"},
		{Code: "fr", Name: "Français", Locale: "fr_FR.UTF-8"},
		{Code: "es", Name: "Español", Locale: "es_ES.UTF-8"},
		{Code: "ja", Name: "日本語", Locale: "ja_JP.UTF-8"},
		{Code: "zh_CN", Name: "简体中文", Locale: "zh_CN.UTF-8"},
		{Code: "ko", Name: "한국어", Locale: "ko_KR.UTF-8"},
		{Code: "ru", Name: "Русский", Locale: "ru_RU.UTF-8"},
	}
}

// GetSelectedLanguage returns the currently selected language code.
func (a *App) GetSelectedLanguage() string {
	return a.selectedLanguage
}

// SetSelectedLanguage sets the selected language and applies the locale.
func (a *App) SetSelectedLanguage(code string) error {
	a.selectedLanguage = code
	lang := a.findLanguage(code)
	if lang == nil {
		return fmt.Errorf("unknown language code: %s", code)
	}
	a.applyLocale(lang.Locale)
	return nil
}

// findLanguage finds a language by code.
func (a *App) findLanguage(code string) *Language {
	for i, lang := range a.GetLanguages() {
		if lang.Code == code {
			return &a.GetLanguages()[i]
		}
	}
	return nil
}

// applyLocale applies the locale string.
func (a *App) applyLocale(locale string) {
	_ = locale
}

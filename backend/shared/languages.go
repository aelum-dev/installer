package shared

// GetLanguages returns the list of available languages.
// Shared by GUI and console installer.
func GetLanguages() []Language {
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

// FindLanguage finds a language by code. Returns nil if not found.
// Shared by GUI and console installer.
func FindLanguage(code string) *Language {
	for _, lang := range GetLanguages() {
		if lang.Code == code {
			return &lang
		}
	}
	return nil
}

package shared

// GetLanguages returns the list of available languages/regions.
func GetLanguages() []Language {
	return []Language{
		{Code: "de_DE", Name: "Deutsch (Deutschland)", Locale: "de_DE.UTF-8"},
		{Code: "en_US", Name: "English (United States)", Locale: "en_US.UTF-8"},
		{Code: "es_ES", Name: "Español (España)", Locale: "es_ES.UTF-8"},
		{Code: "fr_FR", Name: "Français (France)", Locale: "fr_FR.UTF-8"},
		{Code: "it_IT", Name: "Italiano (Italia)", Locale: "it_IT.UTF-8"},
		{Code: "ja_JP", Name: "日本語 (日本)", Locale: "ja_JP.UTF-8"},
		{Code: "ko_KR", Name: "한국어 (대한민국)", Locale: "ko_KR.UTF-8"},
		{Code: "pl_PL", Name: "Polski (Polska)", Locale: "pl_PL.UTF-8"},
		{Code: "pt_BR", Name: "Português (Brasil)", Locale: "pt_BR.UTF-8"},
		{Code: "ru_RU", Name: "Русский (Россия)", Locale: "ru_RU.UTF-8"},
		{Code: "tr_TR", Name: "Türkçe (Türkiye)", Locale: "tr_TR.UTF-8"},
		{Code: "uk_UA", Name: "Українська (Україна)", Locale: "uk_UA.UTF-8"},
		{Code: "zh_CN", Name: "中文 (中国)", Locale: "zh_CN.UTF-8"},
	}
}

// FindLanguage finds a language by code. Returns nil if not found.
// Shared by GUI and console installer.
// NOTE: The returned pointer points into a slice that is recreated on each
// GetLanguages() call. The pointer is only valid until the next GetLanguages()
// call — do not store it long-term.
func FindLanguage(code string) *Language {
	langs := GetLanguages()
	for i := range langs {
		if langs[i].Code == code {
			return &langs[i]
		}
	}
	return nil
}

package shared

// GetLanguages returns the list of available languages/regions.
// Currently limited to 3 regions for simplicity — expand as needed.
func GetLanguages() []Language {
	return []Language{
		{Code: "en_US", Name: "English (United States)", Locale: "en_US.UTF-8"},
		{Code: "ru_RU", Name: "Русский (Россия)", Locale: "ru_RU.UTF-8"},
		{Code: "uk_UA", Name: "Українська (Україна)", Locale: "uk_UA.UTF-8"},
	}
}

// FindLanguage finds a language by code. Returns nil if not found.
// Shared by GUI and console installer.
func FindLanguage(code string) *Language {
	langs := GetLanguages()
	for i := range langs {
		if langs[i].Code == code {
			return &langs[i]
		}
	}
	return nil
}

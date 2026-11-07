package shared

// GetLanguages returns the list of available languages.
//
// A language is only what the system reads and writes: its name and its locale.
// Where the machine is — its clock, console keymap and Arch mirrors — is a
// separate choice, in regions.go. Names here carry no country for that reason;
// the two parentheticals that remain, 简体 and 繁體, name scripts, not places.
func GetLanguages() []Language {
	return []Language{
		{Code: "en_US", Name: "English", Locale: "en_US.UTF-8"},
		{Code: "de_DE", Name: "Deutsch", Locale: "de_DE.UTF-8"},
		{Code: "el_GR", Name: "Ελληνικά", Locale: "el_GR.UTF-8"},
		{Code: "eo", Name: "Esperanto", Locale: "eo.UTF-8"},
		{Code: "es_ES", Name: "Español", Locale: "es_ES.UTF-8"},
		{Code: "fi_FI", Name: "Suomi", Locale: "fi_FI.UTF-8"},
		{Code: "fr_FR", Name: "Français", Locale: "fr_FR.UTF-8"},
		{Code: "hu_HU", Name: "Magyar", Locale: "hu_HU.UTF-8"},
		{Code: "it_IT", Name: "Italiano", Locale: "it_IT.UTF-8"},
		{Code: "nb_NO", Name: "Norsk bokmål", Locale: "nb_NO.UTF-8"},
		{Code: "nl_NL", Name: "Nederlands", Locale: "nl_NL.UTF-8"},
		{Code: "pl_PL", Name: "Polski", Locale: "pl_PL.UTF-8"},
		{Code: "pt_BR", Name: "Português", Locale: "pt_BR.UTF-8"},
		{Code: "ru_RU", Name: "Русский", Locale: "ru_RU.UTF-8"},
		{Code: "sv_SE", Name: "Svenska", Locale: "sv_SE.UTF-8"},
		{Code: "tr_TR", Name: "Türkçe", Locale: "tr_TR.UTF-8"},
		{Code: "uk_UA", Name: "Українська", Locale: "uk_UA.UTF-8"},
		{Code: "bg_BG", Name: "Български", Locale: "bg_BG.UTF-8"},
		{Code: "ca_ES", Name: "Català", Locale: "ca_ES.UTF-8"},
		{Code: "ar_SA", Name: "العربية", Locale: "ar_SA.UTF-8"},
		{Code: "fa_IR", Name: "فارسی", Locale: "fa_IR.UTF-8"},
		{Code: "he_IL", Name: "עברית", Locale: "he_IL.UTF-8"},
		{Code: "id_ID", Name: "Bahasa Indonesia", Locale: "id_ID.UTF-8"},
		{Code: "ja_JP", Name: "日本語", Locale: "ja_JP.UTF-8"},
		{Code: "ko_KR", Name: "한국어", Locale: "ko_KR.UTF-8"},
		{Code: "vi_VN", Name: "Tiếng Việt", Locale: "vi_VN.UTF-8"},
		{Code: "zh_CN", Name: "中文 (简体)", Locale: "zh_CN.UTF-8"},
		{Code: "zh_TW", Name: "中文 (繁體)", Locale: "zh_TW.UTF-8"},
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

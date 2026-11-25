package shared

// GetLanguages returns the list of available languages: every locale glibc can
// generate, from the generated table in locales_table.go.
//
// A language is only what the system reads and writes: its name and its locale.
// Where the machine is — its clock, console keymap and Arch mirrors — is a
// separate choice, in regions.go. The display name carries a territory only to
// tell two written forms of one language apart ("Deutsch (Germany)" against
// "Deutsch (Austria)"); it is not a location choice.
//
// The table is sorted by the native name the row displays, so the list reads
// alphabetically to the person who can read it; the search box matches the
// English name, the code and the locale too, so a reader hunting for "German"
// still finds "Deutsch".
func GetLanguages() []Language {
	out := make([]Language, len(languageTable))
	copy(out, languageTable)
	return out
}

// FindLanguage finds a language by code. Returns nil if not found.
// Shared by GUI and console installer.
func FindLanguage(code string) *Language {
	for i := range languageTable {
		if languageTable[i].Code == code {
			return &languageTable[i]
		}
	}
	return nil
}

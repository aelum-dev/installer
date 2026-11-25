package shared

import (
	"strings"
	"testing"
)

func TestGetLanguages_Invariants(t *testing.T) {
	langs := GetLanguages()
	if len(langs) < 300 {
		t.Fatalf("expected the full glibc locale list (300+), got %d", len(langs))
	}

	seen := map[string]bool{}
	for _, l := range langs {
		if l.Code == "" || l.Name == "" || l.Locale == "" || l.Native == "" || l.En == "" {
			t.Errorf("language has an empty field: %+v", l)
		}
		// Every locale is UTF-8, optionally with a @modifier written form after it
		// (sr_RS.UTF-8@latin), which is what locale.gen and locale.conf take.
		if !strings.Contains(l.Locale, ".UTF-8") {
			t.Errorf("language %q locale %q is not a UTF-8 locale", l.Code, l.Locale)
		}
		if seen[l.Code] {
			t.Errorf("duplicate language code %q", l.Code)
		}
		seen[l.Code] = true
	}
	if !seen[DefaultLanguageCode] {
		t.Errorf("default language %q missing from GetLanguages", DefaultLanguageCode)
	}
}

func TestFindLanguage(t *testing.T) {
	if l := FindLanguage("en_US"); l == nil || l.Code != "en_US" {
		t.Errorf("FindLanguage(en_US) = %+v, want en_US", l)
	}
	if FindLanguage("xx_XX") != nil {
		t.Error("expected nil for an unknown language code")
	}
}

// TestLanguagesResolveToLocaleAndLocation guards the two tables against drift.
// Every selectable language must carry a locale (otherwise the install aborts with
// "locale not set"), and a language whose code names a country must resolve to
// that location — a language with no country of its own ("eo") is the one case
// where inventing one would be wrong.
func TestLanguagesResolveToLocaleAndLocation(t *testing.T) {
	for _, l := range GetLanguages() {
		if got := LocaleForLanguage(l.Code); got != l.Locale {
			t.Errorf("language %q: LocaleForLanguage = %q, want %q", l.Code, got, l.Locale)
		}

		r := RegionForLanguage(l.Code)
		if r.Code == "" {
			t.Errorf("language %q resolves to no location at all", l.Code)
			continue
		}
		// The generator already resolved the territory half (and dropped any
		// @modifier), so the row says whether this language names a country.
		territory := l.Territory
		if territory == "" {
			if r.Code != DefaultRegionCode {
				t.Errorf("language %q has no territory but resolved to %q", l.Code, r.Code)
			}
			continue
		}
		if r.Code != territory && GetRegion(territory).Code != "" {
			t.Errorf("language %q resolved to %q, not its own territory %q", l.Code, r.Code, territory)
		}
	}
}

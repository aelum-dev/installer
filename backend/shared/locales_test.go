package shared

import (
	"strings"
	"testing"
)

func TestGetLanguages_Invariants(t *testing.T) {
	langs := GetLanguages()
	if len(langs) < 28 {
		t.Fatalf("expected at least 28 languages, got %d", len(langs))
	}

	seen := map[string]bool{}
	for _, l := range langs {
		if l.Code == "" || l.Name == "" || l.Locale == "" {
			t.Errorf("language has an empty field: %+v", l)
		}
		if !strings.HasSuffix(l.Locale, ".UTF-8") {
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

// TestLanguagesMatchRegions guards the language/region tables against drift:
// every selectable language must resolve to a region (otherwise the install
// aborts with "region not set"), and every region must be selectable.
func TestLanguagesMatchRegions(t *testing.T) {
	langCodes := map[string]bool{}
	for _, l := range GetLanguages() {
		langCodes[l.Code] = true
		if r := GetRegion(l.Code); r.Code != l.Code {
			t.Errorf("language %q does not resolve to a region (got %q)", l.Code, r.Code)
		}
	}
	for code := range Regions {
		if !langCodes[code] {
			t.Errorf("region %q has no matching language entry", code)
		}
	}
}

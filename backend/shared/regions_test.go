package shared

import (
	"strings"
	"testing"
)

// TestGetRegionCanonicalizes covers the spellings callers actually use: the table
// is keyed on lowercase ISO codes, but --region is typed by hand and people reach
// for the language code they already know. Any of them must resolve, otherwise the
// installer aborts with "region not set" before the first step.
func TestGetRegionCanonicalizes(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantCode string
	}{
		{"exact table key", "ua", "ua"},
		{"uppercase", "UA", "ua"},
		{"surrounding whitespace", "  ua  ", "ua"},
		{"language code", "uk_UA", "ua"},
		{"posix language tag", "uk-UA", "ua"},
		{"full locale", "uk_UA.UTF-8", "ua"},
		{"the country code, not the language code", "gb", "gb"},
		{"uk is a language code, not a location one", "uk", ""}, // the United Kingdom is "gb"
		{"the worldwide entry", "worldwide", DefaultRegionCode},
		{"unknown code", "xx_YY", ""},
		{"empty code", "", ""},
		{"garbage", "not-a-locale", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetRegion(tt.code)
			if got.Code != tt.wantCode {
				t.Fatalf("GetRegion(%q).Code = %q, want %q", tt.code, got.Code, tt.wantCode)
			}
			// A resolved region must carry the full configuration, not just a code.
			if got.Code != "" {
				if got.Name == "" || got.Timezone == "" || got.Keymap == "" {
					t.Fatalf("GetRegion(%q) = %+v, want name/timezone/keymap populated", tt.code, got)
				}
				if got != regionTable[indexOfRegion(got.Code)] {
					t.Fatalf("GetRegion(%q) = %+v, want the table entry verbatim", tt.code, got)
				}
			}
		})
	}
}

// TestCanonicalRegionCode pins the normalization itself, including the cases
// where it must not invent a match.
func TestCanonicalRegionCode(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"UA", "ua"},
		{"uk_UA", "ua"},
		{"uk-UA", "ua"},
		{"UK_UA.UTF-8", "ua"},
		{"Worldwide", "worldwide"},
		{"  eo  ", "eo"},
		{"", ""},
		{"   ", ""},
		{"_US", "us"},
	}

	for _, tt := range tests {
		if got := canonicalRegionCode(tt.in); got != tt.want {
			t.Errorf("canonicalRegionCode(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestRegionForLanguage pins the one link that remains between language and
// location: a language pre-fills the location of its own country, and a language
// with no country of its own invents none.
func TestRegionForLanguage(t *testing.T) {
	tests := []struct {
		language string
		want     string
	}{
		{"uk_UA", "ua"},
		{"en_US", "us"},
		{"pt_BR", "br"},
		{"ca_ES", "es"}, // Catalan: Arch has no Catalonia section, Spain is the territory
		{"zh_TW", "tw"},
		{"eo", DefaultRegionCode},    // Esperanto has no country
		{"xx_ZZ", DefaultRegionCode}, // a territory we do not have is not a country to guess
		{"", DefaultRegionCode},
	}

	for _, tt := range tests {
		got := RegionForLanguage(tt.language)
		if got.Code != tt.want {
			t.Errorf("RegionForLanguage(%q) = %q, want %q", tt.language, got.Code, tt.want)
		}
	}
}

// TestRegionTableIsUsable holds the generated table to what the installer needs:
// unique codes, every entry complete, the worldwide entry present, and a mirror
// file behind every code — a location the installer cannot supply mirrors for is a
// choice it should not have offered.
func TestRegionTableIsUsable(t *testing.T) {
	if len(regionTable) < 50 {
		t.Fatalf("region table has %d entries; Arch lists far more mirror sections", len(regionTable))
	}

	seen := map[string]bool{}
	for _, r := range regionTable {
		if r.Code == "" || r.Name == "" || r.Timezone == "" || r.Keymap == "" {
			t.Errorf("region has an empty field: %+v", r)
		}
		if seen[r.Code] {
			t.Errorf("duplicate region code %q", r.Code)
		}
		seen[r.Code] = true

		if r.Code != DefaultRegionCode && !isISOLower(r.Code) {
			t.Errorf("region %q code %q is not a lowercase ISO 3166-1 alpha-2 code", r.Name, r.Code)
		}

		body, err := GetEmbeddedMirrorList(r.Code)
		if err != nil {
			t.Errorf("region %q: %v", r.Code, err)
			continue
		}
		if !strings.Contains(string(body), "## Arch Linux mirrorlist for "+r.Name+" ("+r.Code+")") {
			t.Errorf("region %q is not backed by its own mirror file", r.Code)
		}
	}

	if !seen[DefaultRegionCode] {
		t.Errorf("the worldwide region (%q) is missing from the table", DefaultRegionCode)
	}
}

// TestGetRegionsSortedByName is what the picker shows: the list arrives ordered,
// so no caller has to remember to sort it.
func TestGetRegionsSortedByName(t *testing.T) {
	regions := GetRegions()
	if len(regions) != len(regionTable) {
		t.Fatalf("GetRegions returned %d entries, want %d", len(regions), len(regionTable))
	}
	for i := 1; i < len(regions); i++ {
		if strings.ToLower(regions[i-1].Name) > strings.ToLower(regions[i].Name) {
			t.Errorf("GetRegions is not sorted by name: %q before %q", regions[i-1].Name, regions[i].Name)
		}
	}

	// The copy must not alias the table: a caller editing what it was given must
	// not be able to rewrite the generated data.
	regions[0].Name = "changed"
	if GetRegions()[0].Name == "changed" {
		t.Error("GetRegions returned the table itself, not a copy")
	}
}

// TestCuratedTablesCoverTheRegionTable keeps the curated half honest: every
// PreferredZone and KeymapByISO entry must name a code that is actually in the
// table, so a retired country cannot linger in a map nobody reads.
func TestCuratedTablesCoverTheRegionTable(t *testing.T) {
	for code, zone := range PreferredZone {
		if GetRegion(code).Code == "" {
			t.Errorf("PreferredZone has an entry for %q (%s), which is not a region", code, zone)
		}
	}
	for code := range KeymapByISO {
		if GetRegion(code).Code == "" {
			t.Errorf("KeymapByISO has an entry for %q, which is not a region", code)
		}
	}
	for code, layout := range XkbLayoutByISO {
		if GetRegion(code).Code == "" {
			t.Errorf("XkbLayoutByISO has an entry for %q (%s), which is not a region", code, layout)
		}
		if layout == "" || layout == DefaultXkbLayout {
			t.Errorf("XkbLayoutByISO maps %q to %q; a US-layout country belongs in neither table", code, layout)
		}
	}
}

// TestXkbLayoutForRegion covers the desktop half of the keyboard: the answer is
// always a layout name, the same location codes are accepted in any of the forms
// callers hold, and a location with no entry of its own — including the worldwide
// one — is the English layout, which is what tells the caller there is nothing
// local to offer.
func TestXkbLayoutForRegion(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{"ru", "ru"},
		{"RU", "ru"},
		{"ru_RU.UTF-8", "ru"},
		{"ua", "ua"},
		{"gb", "gb"},    // the console table spells this one uk
		{"ar", "latam"}, // Argentina, not Spain
		{"es", "es"},
		{"cn", DefaultXkbLayout}, // Chinese keyboards are US-layout; the engine does the work
		{"us", DefaultXkbLayout},
		{DefaultRegionCode, DefaultXkbLayout},
		{"", DefaultXkbLayout},
		{"zz", DefaultXkbLayout},
	}
	for _, c := range cases {
		if got := XkbLayoutForRegion(c.code); got != c.want {
			t.Errorf("XkbLayoutForRegion(%q) = %q, want %q", c.code, got, c.want)
		}
	}
}

// TestLocaleForLanguage covers the language half of the split: the locale comes
// from the language list and nothing else.
func TestLocaleForLanguage(t *testing.T) {
	if got := LocaleForLanguage("uk_UA"); got != "uk_UA.UTF-8" {
		t.Errorf("LocaleForLanguage(uk_UA) = %q, want uk_UA.UTF-8", got)
	}
	if got := LocaleForLanguage("xx_YY"); got != "" {
		t.Errorf("LocaleForLanguage(xx_YY) = %q, want empty", got)
	}
	if got := LocaleForLanguage(DefaultLanguageCode); got == "" {
		t.Errorf("the default language %q has no locale", DefaultLanguageCode)
	}
}

func indexOfRegion(code string) int {
	for i, r := range regionTable {
		if r.Code == code {
			return i
		}
	}
	return 0
}

// isISOLower reports a two-letter lowercase code, the shape every region code but
// the worldwide one must keep: it is also the name of that region's mirror file.
func isISOLower(code string) bool {
	if len(code) != 2 {
		return false
	}
	for _, r := range code {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

package shared

import "testing"

// TestGetRegionCanonicalizes covers the spellings callers actually use: the
// region table is keyed on "en_US", but --language is typed by hand and POSIX /
// full-locale forms are common. Any of them must resolve, otherwise the
// installer aborts with "region not set" before the first step.
func TestGetRegionCanonicalizes(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantCode string
	}{
		{"exact table key", "en_US", "en_US"},
		{"posix tag", "en-US", "en_US"},
		{"lowercase posix tag", "en-us", "en_US"},
		{"uppercase territory", "EN_US", "en_US"},
		{"lowercase everything", "en_us", "en_US"},
		{"surrounding whitespace", "  en-US  ", "en_US"},
		{"full locale", "en_US.UTF-8", "en_US"},
		{"hyphenated full locale", "en-US.UTF-8", "en_US"},
		{"bare language code", "eo", "eo"},
		{"uppercase bare language code", "EO", "eo"},
		{"hyphenated with region variants", "pt-BR", "pt_BR"},
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
				if got.Locale == "" || got.Timezone == "" || got.Keymap == "" {
					t.Fatalf("GetRegion(%q) = %+v, want locale/timezone/keymap populated", tt.code, got)
				}
				if got != Regions[got.Code] {
					t.Fatalf("GetRegion(%q) = %+v, want the Regions[%q] entry verbatim", tt.code, got, got.Code)
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
		{"en_US", "en_US"},
		{"en-US", "en_US"},
		{"en-us", "en_US"},
		{"EN-us", "en_US"},
		{"en_US.UTF-8", "en_US"},
		{"eo", "eo"},
		{"  eo  ", "eo"},
		{"", ""},
		{"   ", ""},
		{"_US", "_us"},
	}

	for _, tt := range tests {
		if got := canonicalRegionCode(tt.in); got != tt.want {
			t.Errorf("canonicalRegionCode(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

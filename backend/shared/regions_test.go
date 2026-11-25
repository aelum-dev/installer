package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
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
// file behind every country that claims its own mirrors. A country without its own
// mirrors is still a valid location — BuildMirrorList falls back to the worldwide
// list — so Mirrors is a fact shown on the row, not a gate on the list.
func TestRegionTableIsUsable(t *testing.T) {
	if len(regionTable) < 200 {
		t.Fatalf("region table has %d entries; the ISO 3166-1 list is far larger", len(regionTable))
	}

	seen := map[string]bool{}
	for _, r := range regionTable {
		if r.Code == "" || r.Name == "" || r.Timezone == "" || r.Keymap == "" || r.Xkb == "" {
			t.Errorf("region has an empty field: %+v", r)
		}
		if seen[r.Code] {
			t.Errorf("duplicate region code %q", r.Code)
		}
		seen[r.Code] = true

		if r.Code != DefaultRegionCode && !isISOLower(r.Code) {
			t.Errorf("region %q code %q is not a lowercase ISO 3166-1 alpha-2 code", r.Name, r.Code)
		}

		if !r.Mirrors {
			continue
		}
		body, err := GetEmbeddedMirrorList(r.Code)
		if err != nil {
			t.Errorf("region %q claims its own mirrors but: %v", r.Code, err)
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
// PreferredZone, KeymapByISO, XkbLayoutByISO and WeatherByISO entry must name a
// code that is actually in the table, so a retired country cannot linger in a map
// nobody reads.
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
	for code, place := range WeatherByISO {
		if GetRegion(code).Code == "" {
			t.Errorf("WeatherByISO has an entry for %q (%s), which is not a region", code, place.City)
		}
	}
}

// TestRegionTableMatchesTheCuratedTables keeps the generated table from drifting
// away from the curated maps it was built from: nothing else notices, because a
// missing regeneration is invisible in the table itself — Iran kept the default
// keymap for a release after KeymapByISO started naming its own.
func TestRegionTableMatchesTheCuratedTables(t *testing.T) {
	const regenerate = "regenerate with go run ./backend/shared/regions_gen.go"

	for _, r := range regionTable {
		if zone, curated := PreferredZone[r.Code]; curated && r.Timezone != zone {
			t.Errorf("region %q shows %s but PreferredZone picks %s: %s", r.Name, r.Timezone, zone, regenerate)
		}

		keymap, ok := KeymapByISO[r.Code]
		if !ok {
			keymap = DefaultKeymap
		}
		if r.Keymap != keymap {
			t.Errorf("region %q has keymap %q but KeymapByISO says %q: %s", r.Name, r.Keymap, keymap, regenerate)
		}
	}
}

// TestRegionTimezonesAreTheCountryOwnClock holds every location's default clock to
// being a real clock of that country: the same civil time as at least one of the
// tzdata rows that list it. It no longer requires the clock to be *named* after
// the country — the time zone is now a choice the person can change on the Welcome
// page, so a default that reads "Africa/Lagos" for Angola (the same clock, a
// neighbour's city) is a reasonable pre-fill, not a wrong promise. What must never
// happen is a default on a different clock than any the country actually runs on.
//
// A curated PreferredZone is checked the same way: it must be the same clock as one
// of the country's rows, which is what lets Denmark read Europe/Copenhagen even
// though tzdata's only row for it is Europe/Berlin.
//
// Skipped where tzdata's zone table is not installed (the build container), since
// the package is what defines these clocks and there is nothing to check against.
func TestRegionTimezonesAreTheCountryOwnClock(t *testing.T) {
	data, err := os.ReadFile(ZoneTablePath)
	if err != nil {
		t.Skipf("%s is not installed here: the zone names cannot be checked against their source", ZoneTablePath)
	}
	zoneDir := filepath.Dir(ZoneTablePath)

	rows := map[string][]string{} // country -> the zones tzdata lists it in
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		for _, code := range strings.Split(fields[0], ",") {
			code = strings.ToLower(strings.TrimSpace(code))
			if len(code) == 2 {
				rows[code] = append(rows[code], fields[2])
			}
		}
	}

	for _, r := range regionTable {
		if r.Code == DefaultRegionCode {
			continue // Etc/UTC is a clock with no country to be a clock of
		}
		if _, err := os.Stat(filepath.Join(zoneDir, r.Timezone)); err != nil {
			t.Errorf("region %q names zone %q, which this tzdata does not have: %v", r.Name, r.Timezone, err)
			continue
		}

		countryRows := rows[r.Code]
		if len(countryRows) == 0 {
			// A place tzdata gives no row of its own: its default is the curated
			// fallback for the clock of the territory it sits inside.
			if want, ok := ZonelessFallback[r.Code]; !ok || want != r.Timezone {
				t.Errorf("region %q has no tzdata row and default %q, want the curated fallback", r.Name, r.Timezone)
			}
			continue
		}

		matches := false
		for _, home := range countryRows {
			if sameClock(zoneDir, home, r.Timezone) == nil {
				matches = true
				break
			}
		}
		if !matches {
			t.Errorf("region %q defaults to %q, which is not any clock tzdata lists for it (%v)",
				r.Name, r.Timezone, countryRows)
		}
	}
}

// sameClock reports whether two names in one tzdata directory are the same civil
// time from 1970 on — exactly what a zone1970.tab row promises of the countries it
// lists, and the only thing that can be compared across distributions. Byte
// equality cannot be: Arch ships a name shared with a neighbour as a link to the
// row's own file, while Debian compiles each name separately from its own pre-1970
// history, so on a Debian runner Europe/Copenhagen and Europe/Berlin are the same
// clock in different bytes (2137 against 2298) and a byte comparison fails a choice
// that is right. Only the offsets are compared: what a zone calls its clock is not
// where the clock stands.
func sameClock(zoneDir, want, got string) error {
	if want == got {
		return nil
	}
	wanted, err := loadZone(zoneDir, want)
	if err != nil {
		return err
	}
	gotten, err := loadZone(zoneDir, got)
	if err != nil {
		return err
	}

	first := time.Date(1970, 1, 1, 12, 0, 0, 0, time.UTC)
	last := first.AddDate(130, 0, 0)
	for day := first; day.Before(last); day = day.AddDate(0, 0, 1) {
		_, home := day.In(wanted).Zone()
		_, shown := day.In(gotten).Zone()
		if home != shown {
			return fmt.Errorf("%s is %v there on %s, %s is %v",
				want, time.Duration(home)*time.Second, day.Format("2006-01-02"),
				got, time.Duration(shown)*time.Second)
		}
	}
	return nil
}

// loadZone reads one zone out of a tzdata directory the way the installed system
// would, so the comparison is of two clocks and not of two files.
func loadZone(zoneDir, name string) (*time.Location, error) {
	data, err := os.ReadFile(filepath.Join(zoneDir, name))
	if err != nil {
		return nil, err
	}
	return time.LoadLocationFromTZData(name, data)
}

// TestWeatherForRegion holds the weather location to its table: every country the
// curated table names must answer with its capital in the form Dux Shell parses,
// the worldwide location must answer "none" rather than guess, and the well-known
// spellings must be the capital, not the largest city.
//
// It no longer requires every offered country to have a capital. The location list
// is now every ISO country, and the curated capital table covers the countries the
// installer shipped when it was written; the rest correctly answer "none" and the
// weather widget keeps its own default. Widening that table is a follow-up, not a
// correctness requirement.
func TestWeatherForRegion(t *testing.T) {
	for code, want := range WeatherByISO {
		place, ok := WeatherForRegion(code)
		if !ok {
			t.Errorf("region %q has a WeatherByISO entry but WeatherForRegion found nothing", code)
			continue
		}
		if place.City != want.City || place.Coordinates != want.Coordinates {
			t.Errorf("WeatherForRegion(%q) = %+v, want %+v", code, place, want)
		}
		if !isCoordinates(place.Coordinates) {
			t.Errorf("region %q has weather coordinates %q, want \"latitude,longitude\"", code, place.Coordinates)
		}
	}

	// The worldwide location is the one entry with no capital, and it must say so.
	if place, ok := WeatherForRegion(DefaultRegionCode); ok {
		t.Errorf("the worldwide location has a capital: %+v", place)
	}

	// The names are what the desktop prints, so the well-known spellings are the
	// ones that must be in the table — and the coordinates must be the capital, not
	// the largest city of that country.
	for _, want := range []struct {
		code, city, coords string
	}{
		{"ua", "Kyiv", "50.4501,30.5234"},
		{"ru", "Moscow", "55.7558,37.6173"},
		{"kz", "Astana", "51.1694,71.4491"}, // not Almaty, the largest city
		{"us", "Washington, D.C.", "38.9072,-77.0369"},
		{"au", "Canberra", "-35.2809,149.1300"}, // not Sydney
		{"za", "Pretoria", "-25.7479,28.2293"},
	} {
		place, ok := WeatherForRegion(want.code)
		if !ok || place.City != want.city || place.Coordinates != want.coords {
			t.Errorf("WeatherForRegion(%q) = %+v, ok=%v; want %s (%s)",
				want.code, place, ok, want.city, want.coords)
		}
	}

	// The same spellings every other region lookup accepts.
	for _, code := range []string{"UA", "uk_UA", "ru_RU.UTF-8"} {
		if place, ok := WeatherForRegion(code); !ok || place.City == "" {
			t.Errorf("WeatherForRegion(%q) found nothing", code)
		}
	}
	for _, code := range []string{"", "zz", DefaultRegionCode} {
		if place, ok := WeatherForRegion(code); ok {
			t.Errorf("WeatherForRegion(%q) = %+v, want no weather location", code, place)
		}
	}
}

// isCoordinates reports whether s is "latitude,longitude" in decimal degrees, the
// form Dux Shell splits on the comma: two numbers, the first within ±90 and the
// second within ±180.
func isCoordinates(s string) bool {
	lat, lon, found := strings.Cut(s, ",")
	if !found || strings.Contains(lon, ",") {
		return false
	}
	latDeg, err := strconv.ParseFloat(lat, 64)
	if err != nil || latDeg < -90 || latDeg > 90 {
		return false
	}
	lonDeg, err := strconv.ParseFloat(lon, 64)
	return err == nil && lonDeg >= -180 && lonDeg <= 180
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

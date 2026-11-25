//go:build ignore

// This file is meant to be run with: go run ./backend/shared/regions_gen.go
// It regenerates regions_table.go — the locations the installer offers.
//
// The list is every ISO 3166-1 country, not only the ones Arch has a mirror
// section for: where a machine is is a fact about the machine, and a country
// without its own mirrors still installs (the worldwide list is the fallback), so
// hiding it would be the installer deciding a person's location for them. Mirror
// availability becomes a per-country flag on the row instead of a gate on the list.
//
// The sources are not ours to copy: iso-codes names the countries, tzdata's
// zone1970.tab holds the clocks, and the installer's own mirrorlist/ directory
// says which countries have a mirror file. What none of them carries — which zone
// a country that spans several gets, which console keymap and desktop layout
// belong where, the plain name of a country whose official one reads badly — is
// curated in countries.go.
//
// ZONE_TABLE_PATH overrides the tzdata file when it is not where Arch puts it.
// ISO_CODES_DIR overrides where iso-codes' JSON lives.

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"aelum-installer/backend/shared"
)

func main() {
	// Change to the shared package directory so relative paths resolve correctly.
	if err := os.Chdir("backend/shared"); err != nil {
		fmt.Fprintf(os.Stderr, "Error changing directory: %v\n", err)
		os.Exit(1)
	}

	zonePath := os.Getenv("ZONE_TABLE_PATH")
	if zonePath == "" {
		zonePath = shared.ZoneTablePath
	}
	isoDir := os.Getenv("ISO_CODES_DIR")
	if isoDir == "" {
		isoDir = "/usr/share/iso-codes/json"
	}

	if err := generate(zonePath, isoDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// isoCountry is one row of iso-codes' ISO 3166-1 table — only the two fields the
// generator reads.
type isoCountry struct {
	Alpha2 string `json:"alpha_2"`
	Name   string `json:"name"`
}

func generate(zonePath, isoDir string) error {
	countries, err := readISOCountries(filepath.Join(isoDir, "iso_3166-1.json"))
	if err != nil {
		return fmt.Errorf("read the ISO 3166-1 country list: %w", err)
	}

	zones, err := readZoneTable(zonePath)
	if err != nil {
		return fmt.Errorf("read the tzdata zone table: %w", err)
	}

	mirrors, err := mirrorCodes()
	if err != nil {
		return fmt.Errorf("read the mirror list directory: %w", err)
	}
	// A country Arch has a mirror section for is named by that section, which
	// already spells the awkward official names the plain way ("South Korea").
	archName := map[string]string{}
	for section, code := range shared.ArchCountryISO {
		archName[code] = section
	}

	table := make([]shared.Region, 0, len(countries)+1)
	var problems []string

	for _, c := range countries {
		code := strings.ToLower(c.Alpha2)
		if code == "" {
			continue
		}
		zone, err := defaultZone(code, zones)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		keymap, ok := shared.KeymapByISO[code]
		if !ok {
			keymap = shared.DefaultKeymap
		}
		xkb, ok := shared.XkbLayoutByISO[code]
		if !ok {
			xkb = shared.DefaultXkbLayout
		}
		_, hasMirror := mirrors[code]
		table = append(table, shared.Region{
			Code:     code,
			Name:     displayName(code, c.Name, archName),
			Timezone: zone,
			Keymap:   keymap,
			Xkb:      xkb,
			Mirrors:  hasMirror,
		})
	}

	// The worldwide location is not a country, so it is not in the ISO list; it is
	// written here and reads like the rest. It is the honest "no preference"
	// answer: a clock with no city, every mirror, the US keymap.
	table = append(table, shared.Region{
		Code:     shared.DefaultRegionCode,
		Name:     "Worldwide",
		Timezone: "UTC",
		Keymap:   shared.DefaultKeymap,
		Xkb:      shared.DefaultXkbLayout,
		Mirrors:  true,
	})

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("the region table is incomplete:\n  %s", strings.Join(problems, "\n  "))
	}

	sort.Slice(table, func(i, j int) bool {
		return strings.ToLower(table[i].Name) < strings.ToLower(table[j].Name)
	})

	if err := writeTable(table, len(mirrors)); err != nil {
		return err
	}
	fmt.Printf("Wrote %d regions (%d with their own Arch mirrors).\n", len(table), len(mirrors))
	return nil
}

// displayName picks the name a country is shown by: the Arch mirrorlist section
// where there is one (it already spells the official names the plain way), then
// the curated override for an official name that reads badly, then iso-codes' own
// name.
func displayName(code, isoName string, archName map[string]string) string {
	if name, ok := archName[code]; ok {
		return name
	}
	if name, ok := shared.CountryNameByISO[code]; ok {
		return name
	}
	return isoName
}

// readISOCountries reads iso-codes' ISO 3166-1 JSON and returns every country
// that has an alpha-2 code, in the file's own order.
func readISOCountries(path string) ([]isoCountry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Countries []isoCountry `json:"3166-1"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	out := make([]isoCountry, 0, len(doc.Countries))
	for _, c := range doc.Countries {
		if c.Alpha2 != "" {
			out = append(out, c)
		}
	}
	return out, nil
}

// mirrorCodes returns the set of country codes the installer ships a mirror list
// for — every mirrorlist/<code>.list except the worldwide and default files.
func mirrorCodes() (map[string]bool, error) {
	entries, err := os.ReadDir(shared.MirrorListsRelativeDir)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".list") {
			continue
		}
		code := strings.TrimSuffix(name, ".list")
		if code == shared.DefaultRegionCode || code == "default" {
			continue
		}
		out[code] = true
	}
	return out, nil
}

// defaultZone resolves a country to the one clock its install starts on: the
// curated pick where one exists, otherwise its only row, otherwise the row nearest
// its capital, otherwise its first row, otherwise the curated fallback for a place
// tzdata gives no row of its own. Unlike the old mirror-gated generator this never
// errors on a multi-zone country without a curated pick — the clock is a default
// the person can change on the Welcome page, not a promise the install is held to,
// so "nearest the capital" is a reasonable answer where PreferredZone has none.
func defaultZone(code string, zones map[string][]zoneRow) (string, error) {
	if zone, ok := shared.PreferredZone[code]; ok {
		return zone, nil
	}
	rows := zones[code]
	if len(rows) == 1 {
		return rows[0].zone, nil
	}
	if len(rows) > 1 {
		if capital, ok := shared.WeatherByISO[code]; ok {
			if coords, err := parseCoordinates(capital.Coordinates); err == nil {
				if nearest := nearestZone(coords, rows); nearest != "" {
					return nearest, nil
				}
			}
		}
		return rows[0].zone, nil
	}
	if zone, ok := shared.ZonelessFallback[code]; ok {
		return zone, nil
	}
	return "UTC", nil
}

// nearestZone returns the zone whose coordinates are closest to the capital, the
// clock a person sitting in that capital is most likely to want.
func nearestZone(capital [2]float64, rows []zoneRow) string {
	best, bestD := "", 0.0
	for _, r := range rows {
		if !r.hasCoords {
			continue
		}
		dLat := r.coords[0] - capital[0]
		dLon := r.coords[1] - capital[1]
		d := dLat*dLat + dLon*dLon
		if best == "" || d < bestD {
			best, bestD = r.zone, d
		}
	}
	return best
}

// zoneRow is one row of the tzdata table a country appears in: its zone name, its
// coordinates, and whether tzdata lists that country first — the flag that says
// the row's city is inside it.
type zoneRow struct {
	zone      string
	coords    [2]float64
	hasCoords bool
	namesIt   bool
}

// readZoneTable reads tzdata's zone1970.tab: columns are a comma-separated list
// of ISO 3166 codes, coordinates, the zone name, and a comment. tzdata puts the
// country of the row's most populous city first, so the first code is the one the
// zone is named after. The map is keyed on country code; a country in several rows
// keeps them in file order.
func readZoneTable(path string) (map[string][]zoneRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	zones := make(map[string][]zoneRow)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		coords, hasCoords := parseZoneCoords(fields[1])
		codes := strings.Split(fields[0], ",")
		for _, code := range codes {
			code = strings.ToLower(strings.TrimSpace(code))
			if len(code) != 2 {
				continue
			}
			zones[code] = append(zones[code], zoneRow{
				zone:      fields[2],
				coords:    coords,
				hasCoords: hasCoords,
				namesIt:   code == strings.ToLower(strings.TrimSpace(codes[0])),
			})
		}
	}
	return zones, scanner.Err()
}

// parseZoneCoords reads a zone1970.tab coordinate pair ("+4030-03651", or with
// seconds "+403030-0365100") into decimal degrees.
func parseZoneCoords(s string) ([2]float64, bool) {
	parts := splitCoord(s)
	if len(parts) != 2 {
		return [2]float64{}, false
	}
	lat, err1 := coordToDegrees(parts[0], 2)
	lon, err2 := coordToDegrees(parts[1], 3)
	if err1 != nil || err2 != nil {
		return [2]float64{}, false
	}
	return [2]float64{lat, lon}, true
}

// splitCoord cuts a "+DDMM+DDDMM" pair at the second sign, keeping each sign with
// its number.
func splitCoord(s string) []string {
	var out []string
	start := 0
	for i := 1; i < len(s); i++ {
		if s[i] == '+' || s[i] == '-' {
			out = append(out, s[start:i])
			start = i
		}
	}
	out = append(out, s[start:])
	return out
}

// coordToDegrees parses one signed coordinate. digits is how many of the digits
// after the sign are degrees (2 for latitude, 3 for longitude); the rest are
// minutes, and a further pair would be seconds.
func coordToDegrees(v string, digits int) (float64, error) {
	sign := 1.0
	if v[0] == '-' {
		sign = -1
	}
	digitsOnly := v[1:]
	if len(digitsOnly) < digits+2 {
		return 0, fmt.Errorf("coordinate %q is too short", v)
	}
	deg, err := strconv.Atoi(digitsOnly[:digits])
	if err != nil {
		return 0, err
	}
	rest := digitsOnly[digits:]
	min, err := strconv.Atoi(rest[:2])
	if err != nil {
		return 0, err
	}
	total := float64(deg) + float64(min)/60
	if len(rest) > 2 {
		sec, err := strconv.Atoi(rest[2:])
		if err != nil {
			return 0, err
		}
		total += float64(sec) / 3600
	}
	return sign * total, nil
}

// parseCoordinates reads the "latitude,longitude" form WeatherByISO stores.
func parseCoordinates(s string) ([2]float64, error) {
	latS, lonS, found := strings.Cut(s, ",")
	if !found {
		return [2]float64{}, fmt.Errorf("not a coordinate pair: %q", s)
	}
	lat, err := strconv.ParseFloat(strings.TrimSpace(latS), 64)
	if err != nil {
		return [2]float64{}, err
	}
	lon, err := strconv.ParseFloat(strings.TrimSpace(lonS), 64)
	if err != nil {
		return [2]float64{}, err
	}
	return [2]float64{lat, lon}, nil
}

func writeTable(table []shared.Region, mirrorCount int) error {
	out, err := os.Create("regions_table.go")
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	w := bufio.NewWriter(out)
	defer func() { _ = w.Flush() }()

	_, _ = w.WriteString(fmt.Sprintf(`// Code generated by regions_gen.go. DO NOT EDIT.
//
// The locations the installer offers: every ISO 3166-1 country, plus the
// worldwide list. A country Arch has no mirror section for is still offered —
// its Mirrors flag is false and the installed system falls back to the
// worldwide list — so the location list is where a machine is, not which
// mirrors exist. %d of these countries ship their own mirror file. Regenerate
// with
//
//	go run ./backend/shared/regions_gen.go
//
// Names are the Arch mirrorlist section where there is one, else iso-codes'
// name with CountryNameByISO applied over the awkward official forms. The
// default timezone comes from the tzdata zone table; the choices tzdata cannot
// make, and the keymap and desktop layout, are curated in countries.go.

package shared

// regionTable is every location the installer can offer.
var regionTable = []Region{
`, mirrorCount))
	for _, r := range table {
		_, _ = fmt.Fprintf(w, "\t{Code: %q, Name: %q, Timezone: %q, Keymap: %q, Xkb: %q, Mirrors: %v},\n",
			r.Code, r.Name, r.Timezone, r.Keymap, r.Xkb, r.Mirrors)
	}
	_, _ = w.WriteString("}\n")

	return w.Flush()
}

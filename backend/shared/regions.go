package shared

import (
	"sort"
	"strings"
)

// Region is a location: what an installed machine's clock, console keyboard and
// Arch mirrors are set to. It carries no language and no locale on purpose.
//
// Those used to be one record, and one Welcome-page click wrote all of it, so a
// person in Ukraine who chose English got 74 US mirrors and a clock set to
// America/New_York. Language says how someone wants to read a screen; it says
// nothing about where the machine is. The language list lives in locales.go and
// the location list is generated here.
type Region struct {
	// Code is the ISO 3166-1 alpha-2 code, lowercase — the same key the generated
	// mirror file is named by (mirrorlist/<code>.list) — or DefaultRegionCode for
	// the worldwide list.
	Code string `json:"code"`
	// Name is the display name, exactly as Arch's mirrorlist spells its country
	// sections. It is English for every locale: these are the names of the mirrors'
	// countries, not UI text, and they are what a user recognises on a map.
	Name     string `json:"name"`
	Timezone string `json:"timezone"` // zoneinfo name, e.g. "Europe/Kyiv"
	Keymap   string `json:"keymap"`   // console keymap name, e.g. "ua"
}

// regionsByCode indexes the generated table. Package initialisation is enough:
// the table is a literal, so there is no I/O and no ordering hazard.
var regionsByCode = func() map[string]Region {
	m := make(map[string]Region, len(regionTable))
	for _, r := range regionTable {
		m[r.Code] = r
	}
	return m
}()

// GetRegions returns every location the installer can offer, sorted by display
// name. The list is exactly the countries Arch has a mirror section for, plus the
// worldwide entry — a location with no mirrors behind it would be a choice the
// installer cannot honour.
func GetRegions() []Region {
	out := make([]Region, len(regionTable))
	copy(out, regionTable)
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// GetRegion returns a region by code, or an empty region if not found.
//
// Lookups tolerate what callers type: the table is keyed on lowercase ISO codes,
// but --region is hand-typed on the CLI and tools pass territory codes in every
// case ("UA", "ua", "uk_UA").
func GetRegion(code string) Region {
	if r, ok := regionsByCode[code]; ok {
		return r
	}
	return regionsByCode[canonicalRegionCode(code)]
}

// RegionForLanguage returns the location a language code implies: the territory
// half of "uk_UA" is Ukraine, "pt_BR" is Brazil. It is how the installer
// pre-fills the location from the language someone chose, so the person whose
// language and country agree still makes one choice, and only the person whose do
// not has to say where they are.
//
// Only the territory half counts. The language half is deliberately not a
// fallback: ISO 639's "uk" is Ukrainian and ISO 3166's "uk" is the United
// Kingdom, and a language with no territory ("eo") or one whose territory Arch has
// no mirrors for gets the worldwide region. Guessing a country is the mistake this
// split exists to stop.
func RegionForLanguage(languageCode string) Region {
	if r, ok := regionsByCode[territoryOf(languageCode)]; ok {
		return r
	}
	return regionsByCode[DefaultRegionCode]
}

// canonicalRegionCode normalises a caller-supplied location code to the form the
// table is keyed on: lowercased, separators unified, and any language or encoding
// half dropped — so "UA", "ua", "uk_UA" and "uk_UA.UTF-8" all become "ua". A code
// with no separator is taken whole ("uk" is the United Kingdom).
func canonicalRegionCode(code string) string {
	if territory := territoryOf(code); territory != "" {
		return territory
	}
	return normalizeTag(code)
}

// territoryOf returns the lowercased territory half of a language tag —
// "uk_UA", "uk-UA" and "uk_UA.UTF-8" all give "ua" — or "" when it has none.
func territoryOf(code string) string {
	normalized := normalizeTag(code)
	if i := strings.LastIndexByte(normalized, '_'); i >= 0 {
		return normalized[i+1:]
	}
	return ""
}

// normalizeTag lowercases a language or location code, trims it, drops any
// encoding suffix and unifies the separators.
func normalizeTag(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if i := strings.IndexByte(code, '.'); i >= 0 {
		code = code[:i]
	}
	return strings.ReplaceAll(code, "-", "_")
}

// LocaleForLanguage returns the system locale ("en_US.UTF-8") a language code
// selects, or "" when the code is not one the installer offers.
func LocaleForLanguage(code string) string {
	if lang := FindLanguage(code); lang != nil {
		return lang.Locale
	}
	return ""
}

package shared

import (
	"fmt"
	"sort"
	"strings"
)

// Timezone is one civil clock the installer can set the system to. It is now a
// first-class choice on the Welcome page rather than a hidden consequence of the
// location, so the whole tzdb set is offered and the location only pre-fills it.
type Timezone struct {
	// ID is the zoneinfo name written to /etc/localtime, e.g. "Europe/Kyiv".
	ID string `json:"id"`
	// Countries lists the ISO 3166-1 codes (lowercase) tzdata names in this zone's
	// row, so a location's own clocks can be listed without re-reading the table.
	Countries []string `json:"countries"`
	// Comment is tzdata's own note for the row (a territory name where a zone is
	// shared), shown nowhere critical but kept so the source is not lost.
	Comment string `json:"comment"`
}

// timezonesByID indexes the generated table.
var timezonesByID = func() map[string]Timezone {
	m := make(map[string]Timezone, len(timezoneTable))
	for _, z := range timezoneTable {
		m[z.ID] = z
	}
	return m
}()

// GetTimezones returns every time zone the installer can offer, sorted by ID.
func GetTimezones() []Timezone {
	out := make([]Timezone, len(timezoneTable))
	copy(out, timezoneTable)
	return out
}

// GetTimezone returns a time zone by its exact zoneinfo name, and whether it was
// found. Unlike the region lookup this is exact: a zone name is typed or comes
// from the table, never from a hand-written language tag.
func GetTimezone(id string) (Timezone, bool) {
	z, ok := timezonesByID[id]
	return z, ok
}

// DefaultTimezoneForRegion returns the clock a location pre-fills: the curated
// pick where one exists, else the country's only clock, else the clock nearest its
// capital — all resolved at generation time and stored on the region. The
// worldwide location answers UTC. An unknown code also answers UTC, the same
// "no preference" clock the worldwide location carries.
func DefaultTimezoneForRegion(code string) string {
	region := GetRegion(code)
	if region.Timezone == "" {
		return "UTC"
	}
	return region.Timezone
}

// TimezonesForRegion returns the clocks a location actually spans — the zones
// tzdata names it in, plus its own default when that default is a local link name
// (Europe/Copenhagen) that is not itself a zone1970 row. Sorted by the city name a
// person reads, so a country's few clocks come up in the order they would look.
func TimezonesForRegion(code string) []Timezone {
	code = GetRegion(code).Code
	if code == "" || code == DefaultRegionCode {
		return nil
	}
	seen := map[string]bool{}
	var out []Timezone
	for _, z := range timezoneTable {
		for _, c := range z.Countries {
			if c == code && !seen[z.ID] {
				seen[z.ID] = true
				out = append(out, z)
				break
			}
		}
	}
	// The location's default may be a curated link name not in any row; it is a
	// clock of this country and must be listed beside the others.
	if def := DefaultTimezoneForRegion(code); def != "" && !seen[def] {
		if z, ok := timezonesByID[def]; ok {
			out = append(out, z)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(ZoneCity(out[i].ID)) < strings.ToLower(ZoneCity(out[j].ID))
	})
	return out
}

// ZoneCity is the part of a zone name a person recognises — the city, with its
// underscores turned back into spaces, or "UTC" for the clock with no city. It is
// the row's primary text; the full ID is shown beside it for anyone who needs the
// exact name.
func ZoneCity(id string) string {
	if id == "UTC" {
		return "UTC"
	}
	if i := strings.LastIndexByte(id, '/'); i >= 0 {
		id = id[i+1:]
	}
	return strings.ReplaceAll(id, "_", " ")
}

// ValidateTimezone reports whether id is a zone the installer offers, in the style
// of the other validators: a typed --timezone that is not in the table is refused
// before the install, not written as a broken /etc/localtime symlink.
func ValidateTimezone(id string) error {
	if _, ok := timezonesByID[id]; !ok {
		return fmt.Errorf("unknown timezone %q (an IANA zone name, e.g. Europe/Kyiv or America/New_York)", id)
	}
	return nil
}

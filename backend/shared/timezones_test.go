package shared

import (
	"strings"
	"testing"
)

// TestTimezoneTableIsUsable holds the generated zone table to what the Welcome
// page and the install need: unique IDs, every PreferredZone name present (the
// install writes those exact names, so a missing one is a broken default), and
// every country code attached to a zone a real lowercase ISO code.
func TestTimezoneTableIsUsable(t *testing.T) {
	if len(timezoneTable) < 300 {
		t.Fatalf("timezone table has %d entries; tzdb has far more clocks", len(timezoneTable))
	}

	seen := map[string]bool{}
	for _, z := range timezoneTable {
		if z.ID == "" {
			t.Errorf("timezone has an empty ID: %+v", z)
		}
		if seen[z.ID] {
			t.Errorf("duplicate timezone ID %q", z.ID)
		}
		seen[z.ID] = true
		for _, c := range z.Countries {
			if !isISOLower(c) {
				t.Errorf("timezone %q lists country %q, not a lowercase ISO 3166-1 code", z.ID, c)
			}
		}
	}

	if !seen["UTC"] {
		t.Error("UTC is missing from the timezone table")
	}

	// The install writes PreferredZone verbatim into /etc/localtime, so every one
	// of those names — including the tzdata links like Europe/Copenhagen — must be
	// a selectable zone.
	for code, zone := range PreferredZone {
		if !seen[zone] {
			t.Errorf("PreferredZone for %q is %q, which is not in the timezone table", code, zone)
		}
	}
}

// TestDefaultTimezoneForRegion pins the cascade from location to clock: a
// multi-zone country gets its curated pick, the worldwide location gets UTC, and
// every offered location's default is a zone the installer can actually set.
func TestDefaultTimezoneForRegion(t *testing.T) {
	cases := map[string]string{
		"us": "America/New_York",
		"de": "Europe/Berlin",
		"au": "Australia/Sydney",
		"ua": "Europe/Kyiv",
		"va": "Europe/Rome", // no tzdata row of its own: the curated fallback
	}
	for code, want := range cases {
		if got := DefaultTimezoneForRegion(code); got != want {
			t.Errorf("DefaultTimezoneForRegion(%q) = %q, want %q", code, got, want)
		}
	}

	if got := DefaultTimezoneForRegion(DefaultRegionCode); got != "UTC" {
		t.Errorf("the worldwide location defaults to %q, want UTC", got)
	}

	// Every location the Welcome page offers must default to a zone the install can
	// write — a default outside the table is a broken /etc/localtime symlink.
	for _, r := range regionTable {
		if err := ValidateTimezone(DefaultTimezoneForRegion(r.Code)); err != nil {
			t.Errorf("region %q defaults to an unsettable zone: %v", r.Code, err)
		}
	}
}

// TestTimezonesForRegion covers the country-only clock list the Welcome page shows
// before someone switches to the whole set: a multi-zone country lists several
// clocks including its default, a single-zone country lists one, and the worldwide
// location has no clocks of its own.
func TestTimezonesForRegion(t *testing.T) {
	us := TimezonesForRegion("us")
	if len(us) < 5 {
		t.Fatalf("the United States lists %d clocks, want many", len(us))
	}
	hasDefault := false
	for _, z := range us {
		if z.ID == DefaultTimezoneForRegion("us") {
			hasDefault = true
		}
	}
	if !hasDefault {
		t.Error("the United States' clock list does not include its default zone")
	}

	if got := TimezonesForRegion(DefaultRegionCode); got != nil {
		t.Errorf("the worldwide location has clocks of its own: %v", got)
	}

	// A curated link name (Denmark's Europe/Copenhagen) is not a tzdata row, yet it
	// is a clock of the country and must appear in its list.
	dk := TimezonesForRegion("dk")
	found := false
	for _, z := range dk {
		if z.ID == "Europe/Copenhagen" {
			found = true
		}
	}
	if !found {
		t.Error("Denmark's clock list is missing its curated Europe/Copenhagen")
	}
}

func TestValidateTimezone(t *testing.T) {
	for _, id := range []string{"UTC", "Europe/Kyiv", "America/New_York"} {
		if err := ValidateTimezone(id); err != nil {
			t.Errorf("ValidateTimezone(%q) = %v, want ok", id, err)
		}
	}
	for _, id := range []string{"", "Not/AZone", "Europe/Kiev"} {
		if err := ValidateTimezone(id); err == nil {
			t.Errorf("ValidateTimezone(%q) = nil, want an error", id)
		}
	}
}

func TestZoneCity(t *testing.T) {
	cases := map[string]string{
		"UTC":                            "UTC",
		"America/Argentina/Buenos_Aires": "Buenos Aires",
		"Europe/Kyiv":                    "Kyiv",
		"Etc/UTC":                        "UTC",
	}
	for id, want := range cases {
		if got := ZoneCity(id); got != want {
			t.Errorf("ZoneCity(%q) = %q, want %q", id, got, want)
		}
	}
}

// TestGetTimezonesIsACopy guards the generated table from a caller that edits what
// it was handed.
func TestGetTimezonesIsACopy(t *testing.T) {
	zones := GetTimezones()
	if len(zones) != len(timezoneTable) {
		t.Fatalf("GetTimezones returned %d, want %d", len(zones), len(timezoneTable))
	}
	zones[0].ID = "changed"
	if GetTimezones()[0].ID == "changed" {
		t.Error("GetTimezones returned the table itself, not a copy")
	}
	if !strings.HasPrefix(timezoneTable[0].ID, "Africa/") {
		t.Logf("note: first zone is %q", timezoneTable[0].ID)
	}
}

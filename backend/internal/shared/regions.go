package shared

// Region represents a regional configuration that maps language to all
// regional parameters: locale, timezone, keyboard layout, and mirrors.
type Region struct {
	Code     string   // e.g., "en_US", "ru_RU", "uk_UA"
	Name     string   // e.g., "English (United States)"
	Locale   string   // e.g., "en_US.UTF-8"
	Timezone string   // e.g., "America/New_York"
	Keymap   string   // e.g., "us"
}

// Regions provides access to predefined regional configurations.
// Expand by adding new entries here.
var Regions = map[string]Region{
	"en_US": {
		Code:     "en_US",
		Name:     "English (United States)",
		Locale:   "en_US.UTF-8",
		Timezone: "America/New_York",
		Keymap:   "us",
	},
	"ru_RU": {
		Code:     "ru_RU",
		Name:     "Русский (Россия)",
		Locale:   "ru_RU.UTF-8",
		Timezone: "Europe/Moscow",
		Keymap:   "ru",
	},
	"uk_UA": {
		Code:     "uk_UA",
		Name:     "Українська (Україна)",
		Locale:   "uk_UA.UTF-8",
		Timezone: "Europe/Kyiv",
		Keymap:   "ua",
	},
}

// GetRegion returns a region by code, or an empty region if not found.
func GetRegion(code string) Region {
	if r, ok := Regions[code]; ok {
		return r
	}
	return Region{}
}


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
var Regions = map[string]Region{
	"de_DE": {
		Code:     "de_DE",
		Name:     "Deutsch (Deutschland)",
		Locale:   "de_DE.UTF-8",
		Timezone: "Europe/Berlin",
		Keymap:   "de",
	},
	"en_US": {
		Code:     "en_US",
		Name:     "English (United States)",
		Locale:   "en_US.UTF-8",
		Timezone: "America/New_York",
		Keymap:   "us",
	},
	"es_ES": {
		Code:     "es_ES",
		Name:     "Español (España)",
		Locale:   "es_ES.UTF-8",
		Timezone: "Europe/Madrid",
		Keymap:   "es",
	},
	"fr_FR": {
		Code:     "fr_FR",
		Name:     "Français (France)",
		Locale:   "fr_FR.UTF-8",
		Timezone: "Europe/Paris",
		Keymap:   "fr",
	},
	"it_IT": {
		Code:     "it_IT",
		Name:     "Italiano (Italia)",
		Locale:   "it_IT.UTF-8",
		Timezone: "Europe/Rome",
		Keymap:   "it",
	},
	"ja_JP": {
		Code:     "ja_JP",
		Name:     "日本語 (日本)",
		Locale:   "ja_JP.UTF-8",
		Timezone: "Asia/Tokyo",
		Keymap:   "jp",
	},
	"ko_KR": {
		Code:     "ko_KR",
		Name:     "한국어 (대한민국)",
		Locale:   "ko_KR.UTF-8",
		Timezone: "Asia/Seoul",
		Keymap:   "kr",
	},
	"pl_PL": {
		Code:     "pl_PL",
		Name:     "Polski (Polska)",
		Locale:   "pl_PL.UTF-8",
		Timezone: "Europe/Warsaw",
		Keymap:   "pl",
	},
	"pt_BR": {
		Code:     "pt_BR",
		Name:     "Português (Brasil)",
		Locale:   "pt_BR.UTF-8",
		Timezone: "America/Sao_Paulo",
		Keymap:   "br-abnt2",
	},
	"ru_RU": {
		Code:     "ru_RU",
		Name:     "Русский (Россия)",
		Locale:   "ru_RU.UTF-8",
		Timezone: "Europe/Moscow",
		Keymap:   "ru",
	},
	"tr_TR": {
		Code:     "tr_TR",
		Name:     "Türkçe (Türkiye)",
		Locale:   "tr_TR.UTF-8",
		Timezone: "Europe/Istanbul",
		Keymap:   "tr",
	},
	"uk_UA": {
		Code:     "uk_UA",
		Name:     "Українська (Україна)",
		Locale:   "uk_UA.UTF-8",
		Timezone: "Europe/Kyiv",
		Keymap:   "ua",
	},
	"zh_CN": {
		Code:     "zh_CN",
		Name:     "中文 (中国)",
		Locale:   "zh_CN.UTF-8",
		Timezone: "Asia/Shanghai",
		Keymap:   "cn",
	},
}

// GetRegion returns a region by code, or an empty region if not found.
func GetRegion(code string) Region {
	if r, ok := Regions[code]; ok {
		return r
	}
	return Region{}
}


package shared

// Region represents a regional configuration that maps language to all
// regional parameters: locale, timezone, keyboard layout, and mirrors.
type Region struct {
	Code     string // e.g., "en_US", "ru_RU", "uk_UA"
	Name     string // e.g., "English (United States)"
	Locale   string // e.g., "en_US.UTF-8"
	Timezone string // e.g., "America/New_York"
	Keymap   string // e.g., "us"
}

// Regions provides access to predefined regional configurations.
var Regions = map[string]Region{
	"en_US": {
		Code:     "en_US",
		Name:     "English (United States)",
		Locale:   "en_US.UTF-8",
		Timezone: "America/New_York",
		Keymap:   "us",
	},
	"bg_BG": {
		Code:     "bg_BG",
		Name:     "Български (България)",
		Locale:   "bg_BG.UTF-8",
		Timezone: "Europe/Sofia",
		Keymap:   "us",
	},
	"ca_ES": {
		Code:     "ca_ES",
		Name:     "Català (Catalunya)",
		Locale:   "ca_ES.UTF-8",
		Timezone: "Europe/Madrid",
		Keymap:   "es",
	},
	"zh_CN": {
		Code:     "zh_CN",
		Name:     "中文 (简体)",
		Locale:   "zh_CN.UTF-8",
		Timezone: "Asia/Shanghai",
		Keymap:   "us",
	},
	"zh_TW": {
		Code:     "zh_TW",
		Name:     "中文 (繁體)",
		Locale:   "zh_TW.UTF-8",
		Timezone: "Asia/Taipei",
		Keymap:   "us",
	},
	"nl_NL": {
		Code:     "nl_NL",
		Name:     "Nederlands (Nederland)",
		Locale:   "nl_NL.UTF-8",
		Timezone: "Europe/Amsterdam",
		Keymap:   "us",
	},
	"eo": {
		Code:     "eo",
		Name:     "Esperanto",
		Locale:   "eo.UTF-8",
		Timezone: "Europe/Berlin",
		Keymap:   "us",
	},
	"fi_FI": {
		Code:     "fi_FI",
		Name:     "Suomi (Suomi)",
		Locale:   "fi_FI.UTF-8",
		Timezone: "Europe/Helsinki",
		Keymap:   "fi",
	},
	"fr_FR": {
		Code:     "fr_FR",
		Name:     "Français (France)",
		Locale:   "fr_FR.UTF-8",
		Timezone: "Europe/Paris",
		Keymap:   "fr",
	},
	"el_GR": {
		Code:     "el_GR",
		Name:     "Ελληνικά (Ελλάδα)",
		Locale:   "el_GR.UTF-8",
		Timezone: "Europe/Athens",
		Keymap:   "us",
	},
	"hu_HU": {
		Code:     "hu_HU",
		Name:     "Magyar (Magyarország)",
		Locale:   "hu_HU.UTF-8",
		Timezone: "Europe/Budapest",
		Keymap:   "hu",
	},
	"id_ID": {
		Code:     "id_ID",
		Name:     "Bahasa Indonesia (Indonesia)",
		Locale:   "id_ID.UTF-8",
		Timezone: "Asia/Jakarta",
		Keymap:   "us",
	},
	"ja_JP": {
		Code:     "ja_JP",
		Name:     "日本語 (日本)",
		Locale:   "ja_JP.UTF-8",
		Timezone: "Asia/Tokyo",
		Keymap:   "jp106",
	},
	"ko_KR": {
		Code:     "ko_KR",
		Name:     "한국어 (대한민국)",
		Locale:   "ko_KR.UTF-8",
		Timezone: "Asia/Seoul",
		Keymap:   "us",
	},
	"nb_NO": {
		Code:     "nb_NO",
		Name:     "Norsk bokmål (Norge)",
		Locale:   "nb_NO.UTF-8",
		Timezone: "Europe/Oslo",
		Keymap:   "no",
	},
	"fa_IR": {
		Code:     "fa_IR",
		Name:     "فارسی (ایران)",
		Locale:   "fa_IR.UTF-8",
		Timezone: "Asia/Tehran",
		Keymap:   "us",
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
		Keymap:   "us",
	},
	"es_ES": {
		Code:     "es_ES",
		Name:     "Español (España)",
		Locale:   "es_ES.UTF-8",
		Timezone: "Europe/Madrid",
		Keymap:   "es",
	},
	"sv_SE": {
		Code:     "sv_SE",
		Name:     "Svenska (Sverige)",
		Locale:   "sv_SE.UTF-8",
		Timezone: "Europe/Stockholm",
		Keymap:   "sv-latin1",
	},
	"tr_TR": {
		Code:     "tr_TR",
		Name:     "Türkçe (Türkiye)",
		Locale:   "tr_TR.UTF-8",
		Timezone: "Europe/Istanbul",
		Keymap:   "trq",
	},
	"vi_VN": {
		Code:     "vi_VN",
		Name:     "Tiếng Việt (Việt Nam)",
		Locale:   "vi_VN.UTF-8",
		Timezone: "Asia/Ho_Chi_Minh",
		Keymap:   "us",
	},
	"ar_SA": {
		Code:     "ar_SA",
		Name:     "العربية (السعودية)",
		Locale:   "ar_SA.UTF-8",
		Timezone: "Asia/Riyadh",
		Keymap:   "us",
	},
	"de_DE": {
		Code:     "de_DE",
		Name:     "Deutsch (Deutschland)",
		Locale:   "de_DE.UTF-8",
		Timezone: "Europe/Berlin",
		Keymap:   "de-latin1",
	},
	"he_IL": {
		Code:     "he_IL",
		Name:     "עברית (ישראל)",
		Locale:   "he_IL.UTF-8",
		Timezone: "Asia/Jerusalem",
		Keymap:   "us",
	},
	"it_IT": {
		Code:     "it_IT",
		Name:     "Italiano (Italia)",
		Locale:   "it_IT.UTF-8",
		Timezone: "Europe/Rome",
		Keymap:   "it",
	},
	"uk_UA": {
		Code:     "uk_UA",
		Name:     "Українська (Україна)",
		Locale:   "uk_UA.UTF-8",
		Timezone: "Europe/Kyiv",
		Keymap:   "us",
	},
}

// GetRegion returns a region by code, or an empty region if not found.
func GetRegion(code string) Region {
	if r, ok := Regions[code]; ok {
		return r
	}
	return Region{}
}

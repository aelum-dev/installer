package shared

type Region struct {
	Code     string // e.g., "en_US", "ru_RU", "uk_UA"
	Name     string // e.g., "English (United States)"
	Locale   string // e.g., "en_US.UTF-8"
	Timezone string // e.g., "America/New_York"
}

// Regions provides access to predefined regional configurations.
var Regions = map[string]Region{
	"en_US": {
		Code:     "en_US",
		Name:     "English (United States)",
		Locale:   "en_US.UTF-8",
		Timezone: "America/New_York",
	},
	"bg_BG": {
		Code:     "bg_BG",
		Name:     "Български (България)",
		Locale:   "bg_BG.UTF-8",
		Timezone: "Europe/Sofia",
	},
	"ca_ES": {
		Code:     "ca_ES",
		Name:     "Català (Catalunya)",
		Locale:   "ca_ES.UTF-8",
		Timezone: "Europe/Madrid",
	},
	"zh_CN": {
		Code:     "zh_CN",
		Name:     "中文 (简体)",
		Locale:   "zh_CN.UTF-8",
		Timezone: "Asia/Shanghai",
	},
	"zh_TW": {
		Code:     "zh_TW",
		Name:     "中文 (繁體)",
		Locale:   "zh_TW.UTF-8",
		Timezone: "Asia/Taipei",
	},
	"nl_NL": {
		Code:     "nl_NL",
		Name:     "Nederlands (Nederland)",
		Locale:   "nl_NL.UTF-8",
		Timezone: "Europe/Amsterdam",
	},
	"eo": {
		Code:     "eo",
		Name:     "Esperanto",
		Locale:   "eo.UTF-8",
		Timezone: "Europe/Berlin",
	},
	"fi_FI": {
		Code:     "fi_FI",
		Name:     "Suomi (Suomi)",
		Locale:   "fi_FI.UTF-8",
		Timezone: "Europe/Helsinki",
	},
	"fr_FR": {
		Code:     "fr_FR",
		Name:     "Français (France)",
		Locale:   "fr_FR.UTF-8",
		Timezone: "Europe/Paris",
	},
	"el_GR": {
		Code:     "el_GR",
		Name:     "Ελληνικά (Ελλάδα)",
		Locale:   "el_GR.UTF-8",
		Timezone: "Europe/Athens",
	},
	"hu_HU": {
		Code:     "hu_HU",
		Name:     "Magyar (Magyarország)",
		Locale:   "hu_HU.UTF-8",
		Timezone: "Europe/Budapest",
	},
	"id_ID": {
		Code:     "id_ID",
		Name:     "Bahasa Indonesia (Indonesia)",
		Locale:   "id_ID.UTF-8",
		Timezone: "Asia/Jakarta",
	},
	"ja_JP": {
		Code:     "ja_JP",
		Name:     "日本語 (日本)",
		Locale:   "ja_JP.UTF-8",
		Timezone: "Asia/Tokyo",
	},
	"ko_KR": {
		Code:     "ko_KR",
		Name:     "한국어 (대한민국)",
		Locale:   "ko_KR.UTF-8",
		Timezone: "Asia/Seoul",
	},
	"nb_NO": {
		Code:     "nb_NO",
		Name:     "Norsk bokmål (Norge)",
		Locale:   "nb_NO.UTF-8",
		Timezone: "Europe/Oslo",
	},
	"fa_IR": {
		Code:     "fa_IR",
		Name:     "فارسی (ایران)",
		Locale:   "fa_IR.UTF-8",
		Timezone: "Asia/Tehran",
	},
	"pl_PL": {
		Code:     "pl_PL",
		Name:     "Polski (Polska)",
		Locale:   "pl_PL.UTF-8",
		Timezone: "Europe/Warsaw",
	},
	"pt_BR": {
		Code:     "pt_BR",
		Name:     "Português (Brasil)",
		Locale:   "pt_BR.UTF-8",
		Timezone: "America/Sao_Paulo",
	},
	"ru_RU": {
		Code:     "ru_RU",
		Name:     "Русский (Россия)",
		Locale:   "ru_RU.UTF-8",
		Timezone: "Europe/Moscow",
	},
	"es_ES": {
		Code:     "es_ES",
		Name:     "Español (España)",
		Locale:   "es_ES.UTF-8",
		Timezone: "Europe/Madrid",
	},
	"sv_SE": {
		Code:     "sv_SE",
		Name:     "Svenska (Sverige)",
		Locale:   "sv_SE.UTF-8",
		Timezone: "Europe/Stockholm",
	},
	"tr_TR": {
		Code:     "tr_TR",
		Name:     "Türkçe (Türkiye)",
		Locale:   "tr_TR.UTF-8",
		Timezone: "Europe/Istanbul",
	},
	"vi_VN": {
		Code:     "vi_VN",
		Name:     "Tiếng Việt (Việt Nam)",
		Locale:   "vi_VN.UTF-8",
		Timezone: "Asia/Ho_Chi_Minh",
	},
	"ar_IQ": {
		Code:     "ar_IQ",
		Name:     "العربية (العراق)",
		Locale:   "ar_IQ.UTF-8",
		Timezone: "Asia/Baghdad",
	},
	"de_DE": {
		Code:     "de_DE",
		Name:     "Deutsch (Deutschland)",
		Locale:   "de_DE.UTF-8",
		Timezone: "Europe/Berlin",
	},
	"he_IL": {
		Code:     "he_IL",
		Name:     "עברית (ישראל)",
		Locale:   "he_IL.UTF-8",
		Timezone: "Asia/Jerusalem",
	},
	"it_IT": {
		Code:     "it_IT",
		Name:     "Italiano (Italia)",
		Locale:   "it_IT.UTF-8",
		Timezone: "Europe/Rome",
	},
	"uk_UA": {
		Code:     "uk_UA",
		Name:     "Українська (Україна)",
		Locale:   "uk_UA.UTF-8",
		Timezone: "Europe/Kyiv",
	},
}

// GetRegion returns a region by code, or an empty region if not found.
func GetRegion(code string) Region {
	if r, ok := Regions[code]; ok {
		return r
	}
	return Region{}
}

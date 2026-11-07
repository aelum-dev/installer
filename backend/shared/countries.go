package shared

// This file is the human-maintained half of the region table. The table itself
// (regions_table.go) is generated, and its two sources are not ours to copy: the
// Arch mirrorlist's country sections (mirrorlist/all_https.txt) and the tzdata
// zone table. What lives here is only the knowledge neither of them carries:
// which ISO code an Arch section name means, which zone to choose for a country
// that spans several, and which console keymap belongs to a country.
//
// Regenerate the table with: go run ./backend/shared/regions_gen.go

// ArchCountryISO maps an Arch mirrorlist country-section name to its ISO 3166-1
// alpha-2 code. The mirrorlist names its sections in English and carries no
// codes; every other table here is keyed on the code, and the generated mirror
// files are named mirrorlist/<code>.list. A section missing from this map is
// skipped by the generator with an error, so the region list can never quietly
// lose a country Arch added.
//
// Two names are worth noting because they are not obvious: "Czechia" is the ISO
// short name for the Czech Republic, and "South Korea" is the ISO name for the
// Republic of Korea (KR).
var ArchCountryISO = map[string]string{
	"Albania":              "al",
	"Argentina":            "ar",
	"Armenia":              "am",
	"Australia":            "au",
	"Austria":              "at",
	"Azerbaijan":           "az",
	"Bangladesh":           "bd",
	"Belgium":              "be",
	"Brazil":               "br",
	"Bulgaria":             "bg",
	"Cambodia":             "kh",
	"Canada":               "ca",
	"Chile":                "cl",
	"China":                "cn",
	"Colombia":             "co",
	"Czechia":              "cz",
	"Denmark":              "dk",
	"Ecuador":              "ec",
	"Estonia":              "ee",
	"Finland":              "fi",
	"France":               "fr",
	"Georgia":              "ge",
	"Germany":              "de",
	"Greece":               "gr",
	"Hong Kong":            "hk",
	"Hungary":              "hu",
	"Iceland":              "is",
	"India":                "in",
	"Indonesia":            "id",
	"Iran":                 "ir",
	"Israel":               "il",
	"Italy":                "it",
	"Japan":                "jp",
	"Kazakhstan":           "kz",
	"Kenya":                "ke",
	"Latvia":               "lv",
	"Lithuania":            "lt",
	"Luxembourg":           "lu",
	"Malaysia":             "my",
	"Mauritius":            "mu",
	"Mexico":               "mx",
	"Moldova":              "md",
	"Morocco":              "ma",
	"Nepal":                "np",
	"Netherlands":          "nl",
	"New Caledonia":        "nc",
	"New Zealand":          "nz",
	"North Macedonia":      "mk",
	"Norway":               "no",
	"Poland":               "pl",
	"Portugal":             "pt",
	"Romania":              "ro",
	"Russia":               "ru",
	"Saudi Arabia":         "sa",
	"Serbia":               "rs",
	"Singapore":            "sg",
	"Slovakia":             "sk",
	"Slovenia":             "si",
	"South Africa":         "za",
	"South Korea":          "kr",
	"Spain":                "es",
	"Sweden":               "se",
	"Switzerland":          "ch",
	"Taiwan":               "tw",
	"Thailand":             "th",
	"Tunisia":              "tn",
	"Türkiye":              "tr",
	"Ukraine":              "ua",
	"United Arab Emirates": "ae",
	"United Kingdom":       "gb",
	"United States":        "us",
	"Uzbekistan":           "uz",
	"Vietnam":              "vn",
}

// PreferredZone picks the zone for a country whose tzdata rows cover several.
// The rule is the city most people in that country live near — the capital where
// there is one — because that is the clock a fresh install should show. tzdata
// orders its rows by coordinates, not by population, so taking the first row for
// these countries is wrong: Australia's first row is Lord Howe Island and
// Ukraine's is Simferopol.
//
// Countries with exactly one row need no entry here; the generator takes that row.
var PreferredZone = map[string]string{
	"ar": "America/Argentina/Buenos_Aires",
	"au": "Australia/Sydney",
	"br": "America/Sao_Paulo",
	"ca": "America/Toronto",
	"cl": "America/Santiago",
	"cn": "Asia/Shanghai",
	"de": "Europe/Berlin", // Europe/Zurich covers Büsingen only
	"ec": "America/Guayaquil",
	"es": "Europe/Madrid",
	"id": "Asia/Jakarta",
	"kz": "Asia/Almaty",
	"my": "Asia/Singapore", // peninsular Malaysia, where Kuala Lumpur is
	"mx": "America/Mexico_City",
	"nz": "Pacific/Auckland",
	"pt": "Europe/Lisbon",
	"ru": "Europe/Moscow",
	"ua": "Europe/Kyiv", // Europe/Simferopol covers Crimea only
	"us": "America/New_York",
	"uz": "Asia/Tashkent",
	"vn": "Asia/Ho_Chi_Minh",
}

// KeymapByISO maps a country to the console keymap its keyboards most commonly
// use — the name `loadkeys` and /etc/vconsole.conf take, not an X11 layout name.
// The names are the ones the `kbd` package in the live image actually ships
// (`kbd` has no `kr`, `th` or `ar` map, and its `uk` is the United Kingdom, not
// Ukraine). A country with no entry gets DefaultKeymap: a wrong keymap is a
// typing nuisance on a text console, a missing one is a boot you cannot log into.
var KeymapByISO = map[string]string{
	"ar": "es",
	"at": "de-latin1",
	"be": "be-latin1",
	"bg": "bg_pho-utf8",
	"br": "br-abnt2",
	"ca": "ca",
	"ch": "de_CH-latin1",
	"cl": "es",
	"co": "es",
	"cz": "cz-lat2",
	"de": "de-latin1",
	"dk": "dk-latin1",
	"ec": "es",
	"ee": "et",
	"es": "es",
	"fi": "fi",
	"fr": "fr",
	"gb": "uk",
	"ge": "ge",
	"gr": "gr",
	"hu": "hu101",
	"is": "is-latin1",
	"il": "il",
	"ir": "fa",
	"it": "it",
	"jp": "jp106",
	"kz": "kazakh",
	"lt": "lt",
	"lv": "lv",
	"md": "ro",
	"mk": "mk",
	"mx": "es",
	"nc": "fr",
	"nl": "nl2",
	"no": "no-latin1",
	"pl": "pl",
	"pt": "pt-latin1",
	"ro": "ro",
	"rs": "sr-cy",
	"ru": "ru",
	"se": "sv-latin1",
	"si": "slovene",
	"sk": "sk-qwerty",
	"tr": "trq",
	"ua": "ua",
}

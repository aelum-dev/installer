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

// XkbLayoutByISO maps a country to the keyboard layout its keyboards carry, in
// xkeyboard-config's names — the names the framework's `keyboard-<layout>` input
// methods take, and Hyprland's kb_layout. A country is absent when the keyboards
// sold there are the US layout, which is most of the list: India, Indonesia, the
// Gulf states, China and Vietnam all type on US keycaps.
//
// It is a separate table from KeymapByISO, not a translation of it, for three
// reasons that are each a difference in fact rather than in spelling:
//
//   - `kbd` has no map for some real layouts at all (its own AGENTS note names
//     `kr` and `th`), so the console table cannot say those keyboards differ;
//   - the two name spaces disagree (`de-latin1` against `de`, `sr-cy` against
//     `rs`, `bg_pho-utf8` against `bg`);
//   - Spanish keyboards are not one layout: Spain sells `es`, and Argentina,
//     Chile, Colombia, Ecuador and Mexico sell `latam`, which is where the
//     right-hand keys and the @ sign live in different places.
//
// Moldova is the one entry that does not follow its console row: `kbd` has no
// Moldovan map worth using, and the keyboards sold there carry the Romanian
// layout.
//
// A language typed through an engine is not this table's business: the engine is
// chosen in installdesktop/inputmethods.go, and Pinyin, Mozc, Hangul, Zhuyin and
// Telex Vietnamese all take US keys, so `cn`, `tw` and `vn` are deliberately
// absent while `jp` and `kr` — real keyboards with real keycaps — are present.
//
// Every name here is a layout in xkeyboard-config's own evdev.xml (`python3`
// over /usr/share/X11/xkb/rules/evdev.xml lists them); a name that is not there
// is an input method fcitx5 drops as invalid.
var XkbLayoutByISO = map[string]string{
	"ar": "latam",
	"at": "at",
	"be": "be",
	"bg": "bg",
	"br": "br",
	"ca": "ca",
	"ch": "ch",
	"cl": "latam",
	"co": "latam",
	"cz": "cz",
	"de": "de",
	"dk": "dk",
	"ec": "latam",
	"ee": "ee",
	"es": "es",
	"fi": "fi",
	"fr": "fr",
	"gb": "gb",
	"ge": "ge",
	"gr": "gr",
	"hu": "hu",
	"il": "il",
	"is": "is",
	"it": "it",
	"jp": "jp",
	"kr": "kr",
	"kz": "kz",
	"lt": "lt",
	"lv": "lv",
	"md": "ro",
	"mk": "mk",
	"mx": "latam",
	"nc": "fr",
	"nl": "nl",
	"no": "no",
	"pl": "pl",
	"pt": "pt",
	"ro": "ro",
	"rs": "rs",
	"ru": "ru",
	"se": "se",
	"si": "si",
	"sk": "sk",
	"th": "th",
	"tr": "tr",
	"ua": "ua",
}

package shared

// This file is the human-maintained half of the region table. The table itself
// (regions_table.go) is generated, and its two sources are not ours to copy: the
// Arch mirrorlist's country sections (mirrorlist/all_https.txt) and the tzdata
// zone table. What lives here is only the knowledge neither of them carries:
// which ISO code an Arch section name means, which zone to choose for a country
// that spans several, which console keymap and desktop keyboard layout belong to
// a country, and which city its weather is reported for.
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

// WeatherByISO maps a country to the place the installed desktop reports its
// weather for: its capital, with the coordinates the weather service is asked
// with. The Welcome page's location answer is a country, and the weather widget
// needs one point on it — the capital is the point a country agrees on, where its
// largest city would be a guess about where the machine happens to be (Kazakhstan's
// largest city is Almaty, its capital is Astana, and the installer cannot know
// which of the two the person is sitting in).
//
// Names are English for the same reason the region list's names are English: they
// name a place on a map rather than a piece of UI, and the desktop shows this one
// verbatim beside the temperature. Coordinates are "latitude,longitude" in decimal
// degrees — Dux Shell's own format, which it splits on the comma — to four decimals,
// about 11 metres and far more precision than a city needs.
//
// Two rows are worth a note because the country itself is the argument:
//
//   - South Africa names Pretoria, the executive capital and by far the largest of
//     its three seats of government;
//   - Indonesia names Jakarta although the designated new capital, Nusantara, has
//     been under construction since 2022: the seat of government a person on that
//     machine is near is still Jakarta, and a weather widget pointed at a building
//     site in Kalimantan would be wrong in the only way that matters.
//
// A country with no row gets no weather location, and the desktop keeps its own
// default. The worldwide location is the one entry of the region list that has no
// capital, so it is deliberately absent here; TestWeatherForRegion is what holds
// every country that is offered to a capital.
var WeatherByISO = map[string]WeatherPlace{
	"al": {City: "Tirana", Coordinates: "41.3275,19.8187"},
	"ar": {City: "Buenos Aires", Coordinates: "-34.6037,-58.3816"},
	"am": {City: "Yerevan", Coordinates: "40.1792,44.4991"},
	"au": {City: "Canberra", Coordinates: "-35.2809,149.1300"},
	"at": {City: "Vienna", Coordinates: "48.2082,16.3738"},
	"az": {City: "Baku", Coordinates: "40.4093,49.8671"},
	"bd": {City: "Dhaka", Coordinates: "23.8103,90.4125"},
	"be": {City: "Brussels", Coordinates: "50.8503,4.3517"},
	"br": {City: "Brasília", Coordinates: "-15.7975,-47.8919"},
	"bg": {City: "Sofia", Coordinates: "42.6977,23.3219"},
	"kh": {City: "Phnom Penh", Coordinates: "11.5564,104.9282"},
	"ca": {City: "Ottawa", Coordinates: "45.4215,-75.6972"},
	"cl": {City: "Santiago", Coordinates: "-33.4489,-70.6693"},
	"cn": {City: "Beijing", Coordinates: "39.9042,116.4074"},
	"co": {City: "Bogotá", Coordinates: "4.7110,-74.0721"},
	"cz": {City: "Prague", Coordinates: "50.0755,14.4378"},
	"dk": {City: "Copenhagen", Coordinates: "55.6761,12.5683"},
	"ec": {City: "Quito", Coordinates: "-0.1807,-78.4678"},
	"ee": {City: "Tallinn", Coordinates: "59.4370,24.7536"},
	"fi": {City: "Helsinki", Coordinates: "60.1699,24.9384"},
	"fr": {City: "Paris", Coordinates: "48.8566,2.3522"},
	"ge": {City: "Tbilisi", Coordinates: "41.7151,44.8271"},
	"de": {City: "Berlin", Coordinates: "52.5200,13.4050"},
	"gr": {City: "Athens", Coordinates: "37.9838,23.7275"},
	"hk": {City: "Hong Kong", Coordinates: "22.3193,114.1694"},
	"hu": {City: "Budapest", Coordinates: "47.4979,19.0402"},
	"is": {City: "Reykjavík", Coordinates: "64.1466,-21.9426"},
	"in": {City: "New Delhi", Coordinates: "28.6139,77.2090"},
	"id": {City: "Jakarta", Coordinates: "-6.2088,106.8456"},
	"ir": {City: "Tehran", Coordinates: "35.6892,51.3890"},
	"il": {City: "Jerusalem", Coordinates: "31.7683,35.2137"},
	"it": {City: "Rome", Coordinates: "41.9028,12.4964"},
	"jp": {City: "Tokyo", Coordinates: "35.6762,139.6503"},
	"kz": {City: "Astana", Coordinates: "51.1694,71.4491"},
	"ke": {City: "Nairobi", Coordinates: "-1.2921,36.8219"},
	"lv": {City: "Riga", Coordinates: "56.9496,24.1052"},
	"lt": {City: "Vilnius", Coordinates: "54.6872,25.2797"},
	"lu": {City: "Luxembourg", Coordinates: "49.6116,6.1319"},
	"my": {City: "Kuala Lumpur", Coordinates: "3.1390,101.6869"},
	"mu": {City: "Port Louis", Coordinates: "-20.1641,57.5012"},
	"mx": {City: "Mexico City", Coordinates: "19.4326,-99.1332"},
	"md": {City: "Chișinău", Coordinates: "47.0106,28.9480"},
	"ma": {City: "Rabat", Coordinates: "34.0209,-6.8416"},
	"np": {City: "Kathmandu", Coordinates: "27.7172,85.3240"},
	"nl": {City: "Amsterdam", Coordinates: "52.3676,4.9041"},
	"nc": {City: "Nouméa", Coordinates: "-22.2758,166.4580"},
	"nz": {City: "Wellington", Coordinates: "-41.2866,174.7756"},
	"mk": {City: "Skopje", Coordinates: "41.9973,21.4280"},
	"no": {City: "Oslo", Coordinates: "59.9139,10.7522"},
	"pl": {City: "Warsaw", Coordinates: "52.2297,21.0122"},
	"pt": {City: "Lisbon", Coordinates: "38.7223,-9.1393"},
	"ro": {City: "Bucharest", Coordinates: "44.4268,26.1025"},
	"ru": {City: "Moscow", Coordinates: "55.7558,37.6173"},
	"sa": {City: "Riyadh", Coordinates: "24.7136,46.6753"},
	"rs": {City: "Belgrade", Coordinates: "44.7866,20.4489"},
	"sg": {City: "Singapore", Coordinates: "1.3521,103.8198"},
	"sk": {City: "Bratislava", Coordinates: "48.1459,17.1097"},
	"si": {City: "Ljubljana", Coordinates: "46.0569,14.5058"},
	"za": {City: "Pretoria", Coordinates: "-25.7479,28.2293"},
	"kr": {City: "Seoul", Coordinates: "37.5665,126.9780"},
	"es": {City: "Madrid", Coordinates: "40.4168,-3.7038"},
	"se": {City: "Stockholm", Coordinates: "59.3293,18.0686"},
	"ch": {City: "Bern", Coordinates: "46.9480,7.4474"},
	"tw": {City: "Taipei", Coordinates: "25.0330,121.5654"},
	"th": {City: "Bangkok", Coordinates: "13.7563,100.5018"},
	"tn": {City: "Tunis", Coordinates: "36.8065,10.1815"},
	"tr": {City: "Ankara", Coordinates: "39.9334,32.8597"},
	"ua": {City: "Kyiv", Coordinates: "50.4501,30.5234"},
	"ae": {City: "Abu Dhabi", Coordinates: "24.4539,54.3773"},
	"gb": {City: "London", Coordinates: "51.5074,-0.1278"},
	"us": {City: "Washington, D.C.", Coordinates: "38.9072,-77.0369"},
	"uz": {City: "Tashkent", Coordinates: "41.2995,69.2401"},
	"vn": {City: "Hanoi", Coordinates: "21.0278,105.8342"},
}

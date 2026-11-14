package installdesktop

import (
	"encoding/xml"
	"os"
	"slices"
	"strings"
	"testing"

	"aelum-installer/backend/shared"
)

// --- the language table ------------------------------------------------------

// TestIMETable_KeysAreOfferedLanguages pins the table to the language list:
// every row must name a language the installer actually offers, so the table
// can never drift ahead of locales.go (a row for an unofferable language is
// dead code that reads like a promise).
func TestIMETable_KeysAreOfferedLanguages(t *testing.T) {
	offered := map[string]bool{}
	for _, lang := range shared.GetLanguages() {
		offered[lang.Code] = true
	}
	for code := range imeTable {
		if !offered[code] {
			t.Errorf("imeTable has a row for %q, which GetLanguages does not offer", code)
		}
	}
}

// TestIMETable_CoversEveryIMETypeLanguage guards the other direction: every
// offered language whose locale needs an input method engine has a row. These
// are the five whose ordinary typing is not one-key-per-character; a missing
// row would leave a zh/ja/ko/vi install with the core and no engine.
func TestIMETable_CoversEveryIMETypeLanguage(t *testing.T) {
	want := []string{"zh_CN", "zh_TW", "ja_JP", "ko_KR", "vi_VN"}
	for _, code := range want {
		if _, ok := imeTable[code]; !ok {
			t.Errorf("language %q needs an input method engine but has no table row", code)
		}
	}
}

// TestIMCandidatePackages_CoversEverySet is the drivers.go invariant, repeated
// for the language table: every name a row can produce is in the candidate
// union the ISO build reads, or the offline mirror could not resolve it.
func TestIMCandidatePackages_CoversEverySet(t *testing.T) {
	for code, set := range imeTable {
		for _, name := range set.packages {
			if !slices.Contains(IMCandidatePackages, name) {
				t.Errorf("%q is selectable for %s but not in IMCandidatePackages; the offline mirror could not resolve it", name, code)
			}
		}
	}
	seen := map[string]int{}
	for _, name := range IMCandidatePackages {
		if name == "" {
			t.Fatal("IMCandidatePackages contains an empty entry")
		}
		seen[name]++
		if seen[name] > 1 {
			t.Errorf("IMCandidatePackages contains duplicate %q", name)
		}
	}
}

// TestIMECoreIsUnconditional pins the scope model: the core stack is part of
// every install's desktop set, and no language addon is.
func TestIMECoreIsUnconditional(t *testing.T) {
	for _, core := range []string{"fcitx5", "fcitx5-gtk", "fcitx5-qt", "fcitx5-configtool"} {
		if !slices.Contains(DesktopPackages, core) {
			t.Errorf("the fcitx5 core %q is missing from DesktopPackages", core)
		}
	}
	for _, addon := range IMCandidatePackages {
		if slices.Contains(DesktopPackages, addon) {
			t.Errorf("%q is hardcoded into DesktopPackages; inputmethods.go must be the only chooser", addon)
		}
	}
}

// TestIMEAddonsAreNotDriverCandidates keeps the two dynamic lists disjoint —
// a name in both would say two different tables chose it.
func TestIMEAddonsAreNotDriverCandidates(t *testing.T) {
	for _, name := range IMCandidatePackages {
		if slices.Contains(DriverCandidatePackages, name) {
			t.Errorf("%q is in both the driver and the input method candidate lists", name)
		}
	}
}

// --- languageFromLocale ------------------------------------------------------

func TestLanguageFromLocale(t *testing.T) {
	cases := []struct{ in, want string }{
		{"en_US.UTF-8", "en_US"},
		{"zh_CN.UTF-8", "zh_CN"},
		{"eo.UTF-8", "eo"},
		{"zh_CN", "zh_CN"}, // already a bare code
		{"", ""},           // no locale: no language, no addons
		{".UTF-8", ""},     // degenerate, must not panic or invent one
	}
	for _, c := range cases {
		if got := languageFromLocale(c.in); got != c.want {
			t.Errorf("languageFromLocale(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// --- selectIMEAddons / IMESummary --------------------------------------------

func TestSelectIMEAddons(t *testing.T) {
	if got := selectIMEAddons("en_US"); got != nil {
		t.Errorf("a Latin language got addons: %v", got)
	}
	if got := selectIMEAddons(""); got != nil {
		t.Errorf("an empty language got addons: %v", got)
	}
	for code := range imeTable {
		got := selectIMEAddons(code)
		want := imeTable[code].packages
		if !slices.Equal(got, want) {
			t.Errorf("selectIMEAddons(%q) = %v, want %v", code, got, want)
		}
		// The caller must not be able to append into the table row: write
		// through the returned slice and ask again.
		got[0] = "tampered"
		if slices.Contains(selectIMEAddons(code), "tampered") {
			t.Errorf("selectIMEAddons(%q) handed out the table's own slice", code)
		}
	}
}

// --- what the Confirmation page can say ---------------------------------------

// labelOf spells one entry of a session's input method list the way the page
// would: a keyboard by its layout name, an engine by its display name.
func labelOf(im string) string {
	if strings.HasPrefix(im, shared.Fcitx5KeyboardIMPrefix) {
		return xkbLayoutName(strings.TrimPrefix(im, shared.Fcitx5KeyboardIMPrefix))
	}
	return imeDisplayName(im)
}

// TestSessionInputMethodNames_MatchesTheSessionList is the invariant that keeps a
// preview from becoming a story: every name the page shows must be what
// SessionInputMethods actually writes for the same language and location, in the
// same order. A page that names an input method the session never gets is worse
// than one that says nothing.
func TestSessionInputMethodNames_MatchesTheSessionList(t *testing.T) {
	cases := []struct{ language, region string }{
		{"en_US", "us"},
		{"uk_UA", "ua"},
		{"ru_RU", "ru"},
		{"de_DE", "de"},
		{"es_ES", "mx"},
		{"ja_JP", "jp"},
		{"ko_KR", "kr"},
		{"zh_CN", "cn"},
		{"zh_TW", "tw"},
		{"vi_VN", "vn"},
		{"en_US", "world"},
		{"uk_UA", "de"}, // the language is not the keyboard: the location is
	}
	for _, c := range cases {
		ctx := &shared.InstallContext{Locale: c.language + ".UTF-8", Region: shared.Region{Code: c.region}}
		ChooseIMEAddons(ctx)
		list := SessionInputMethods(ctx)
		want := make([]string, 0, len(list))
		for _, im := range list {
			want = append(want, labelOf(im))
		}
		if got := SessionInputMethodNames(c.language, c.region); !slices.Equal(got, want) {
			t.Errorf("SessionInputMethodNames(%s, %s) = %v, want %v (the session list is %v)",
				c.language, c.region, got, want, list)
		}
	}
}

// TestSessionInputMethodNames reads as the answer to "which keyboards will I
// have?" — the plain case is the machine's own language next to English, and a
// machine anywhere else never gets a second keyboard at all.
func TestSessionInputMethodNames(t *testing.T) {
	cases := []struct {
		region   string
		language string
		want     []string
	}{
		{region: "ua", language: "uk_UA", want: []string{"English (US)", "Ukrainian"}},
		{region: "ru", language: "ru_RU", want: []string{"English (US)", "Russian"}},
		{region: "jp", language: "ja_JP", want: []string{"English (US)", "Japanese", "Mozc"}},
		{region: "us", language: "en_US", want: []string{"English (US)"}},
		{region: "world", language: "en_US", want: []string{"English (US)"}},
		// Latin American keyboards are one layout, not one per country, and the
		// engine for Vietnamese takes US keys: no keyboard of its own.
		{region: "mx", language: "es_ES", want: []string{"English (US)", "Spanish (Latin American)"}},
		{region: "vn", language: "vi_VN", want: []string{"English (US)", "Bamboo"}},
	}
	for _, c := range cases {
		if got := SessionInputMethodNames(c.language, c.region); !slices.Equal(got, c.want) {
			t.Errorf("SessionInputMethodNames(%s, %s) = %v, want %v", c.language, c.region, got, c.want)
		}
	}
}

// TestXkbLayoutNames_CoversEveryOfferedLocation: a location the installer can be
// pointed at must have a name to show for it, or the summary silently answers with
// a layout code.
func TestXkbLayoutNames_CoversEveryOfferedLocation(t *testing.T) {
	offered := map[string]bool{shared.DefaultXkbLayout: true}
	for _, layout := range shared.XkbLayoutByISO {
		offered[layout] = true
	}
	for layout := range offered {
		if strings.TrimSpace(xkbLayoutNames[layout]) == "" {
			t.Errorf("no display name for the keyboard layout %q, which a location offers", layout)
		}
	}
	for layout := range xkbLayoutNames {
		if strings.HasPrefix(layout, shared.Fcitx5KeyboardIMPrefix) {
			t.Errorf("xkbLayoutNames keys are layout codes, not framework names: %q", layout)
		}
	}
}

// TestIMEEngineNames_AreAllNamed: every engine a row puts in the session's list
// must have a name to show, and the display table must not keep names for engines
// no row installs (a dead entry reads as an engine that could come back).
func TestIMEEngineNames_AreAllNamed(t *testing.T) {
	installed := map[string]bool{}
	for code, set := range imeTable {
		for _, method := range set.inputMethods {
			if strings.TrimSpace(imeDisplayNames[method]) == "" {
				t.Errorf("no display name for the engine %q that %s installs", method, code)
			}
			installed[method] = true
		}
	}
	for method := range imeDisplayNames {
		if !installed[method] {
			t.Errorf("imeDisplayNames names the engine %q, which no table row installs", method)
		}
	}
}

// evdevLayouts / evdevDescription read xkeyboard-config's own list of layouts.
type evdevLayouts struct {
	LayoutList struct {
		Layouts []struct {
			ConfigItem struct {
				Name        string `xml:"name"`
				Description string `xml:"description"`
			} `xml:"configItem"`
		} `xml:"layout"`
	} `xml:"layoutList"`
}

// TestXkbLayoutNames_AreEvdevDescriptions holds the table against xkeyboard-config
// itself: every name must be the description upstream gives that layout, which is
// the word the person will read in the input method switcher after the install.
// Skipped where xkeyboard-config is not installed (the build container), since the
// package is what defines these names and there is nothing to check against.
func TestXkbLayoutNames_AreEvdevDescriptions(t *testing.T) {
	const evdevPath = "/usr/share/X11/xkb/rules/evdev.xml"

	data, err := os.ReadFile(evdevPath)
	if err != nil {
		t.Skipf("%s is not installed here: the layout names cannot be checked against their source", evdevPath)
	}
	var doc evdevLayouts
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("cannot parse %s: %v", evdevPath, err)
	}
	descriptions := map[string]string{}
	for _, l := range doc.LayoutList.Layouts {
		descriptions[l.ConfigItem.Name] = l.ConfigItem.Description
	}
	for layout, name := range xkbLayoutNames {
		upstream, ok := descriptions[layout]
		if !ok {
			t.Errorf("xkbLayoutNames names %q, which xkeyboard-config has no layout for", layout)
			continue
		}
		if upstream != name {
			t.Errorf("xkbLayoutNames[%q] = %q, but xkeyboard-config describes it as %q", layout, name, upstream)
		}
	}
}

// --- ChooseIMEAddons ----------------------------------------------------------

func TestChooseIMEAddons_FollowsTheLanguage(t *testing.T) {
	ctx := &shared.InstallContext{Locale: "ja_JP.UTF-8"}
	got := ChooseIMEAddons(ctx)
	want := []string{"fcitx5-mozc"}
	if !slices.Equal(got, want) {
		t.Fatalf("ChooseIMEAddons(ja_JP) = %v, want %v", got, want)
	}
	if !slices.Equal(ctx.IMPackages, want) {
		t.Errorf("the choice was not recorded on the context: %v", ctx.IMPackages)
	}
}

// TestChooseIMEAddons_LatinLanguageRecordsAnEmptyDecision: getting no addons
// is a decision, not a gap — it is recorded non-nil so a step re-run and the
// verify step both see it.
func TestChooseIMEAddons_LatinLanguageRecordsAnEmptyDecision(t *testing.T) {
	ctx := &shared.InstallContext{Locale: "de_DE.UTF-8"}
	if got := ChooseIMEAddons(ctx); len(got) != 0 {
		t.Fatalf("a Latin language got addons: %v", got)
	}
	if ctx.IMPackages == nil {
		t.Error("the empty decision was not recorded (IMPackages is nil)")
	}
}

func TestChooseIMEAddons_NoneOverride(t *testing.T) {
	ctx := &shared.InstallContext{Locale: "zh_CN.UTF-8", InputMethod: InputMethodOverrideNone}
	if got := ChooseIMEAddons(ctx); len(got) != 0 {
		t.Errorf("--input-method none still got addons: %v", got)
	}
	if ctx.IMPackages == nil {
		t.Error("the override decision was not recorded (IMPackages is nil)")
	}
}

func TestChooseIMEAddons_ReusesARecordedSet(t *testing.T) {
	ctx := &shared.InstallContext{
		Locale:     "en_US.UTF-8",
		IMPackages: []string{"fcitx5-hangul"}, // a re-run, or a test
	}
	got := ChooseIMEAddons(ctx)
	if !slices.Equal(got, ctx.IMPackages) {
		t.Errorf("a recorded set was not reused: %v", got)
	}
}

// TestValidInputMethodOverride mirrors the driver-override test: the accepted
// words, the empty default, and nothing else.
func TestValidInputMethodOverride(t *testing.T) {
	for _, ok := range []string{"", InputMethodOverrideAuto, InputMethodOverrideNone} {
		if !ValidInputMethodOverride(ok) {
			t.Errorf("ValidInputMethodOverride(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"fcitx5", "auto ", "NONE", "ibus"} {
		if ValidInputMethodOverride(bad) {
			t.Errorf("ValidInputMethodOverride(%q) = true, want false", bad)
		}
	}
}

// --- the engine names a session may be told about -----------------------------

// TestIMEInputMethods_AreWhatThePackagesRegister pins the names a session profile
// can name to the input methods the addon packages actually register. Each is the
// basename of a .conf under usr/share/fcitx5/inputmethod in the current Arch
// package — read out of the packages rather than guessed: pinyin.conf in
// fcitx5-chinese-addons, then chewing.conf, mozc.conf, hangul.conf and bamboo.conf.
// An entry the framework cannot resolve is dropped from the group with a warning,
// so a wrong name is a language that silently never appears in the list.
func TestIMEInputMethods_AreWhatThePackagesRegister(t *testing.T) {
	want := map[string][]string{
		"zh_CN": {"pinyin"},
		"zh_TW": {"chewing"},
		"ja_JP": {"mozc"},
		"ko_KR": {"hangul"},
		"vi_VN": {"bamboo"},
	}
	for code, names := range want {
		set, ok := imeTable[code]
		if !ok {
			t.Fatalf("language %q has no table row", code)
		}
		if !slices.Equal(set.inputMethods, names) {
			t.Errorf("imeTable[%q].inputMethods = %v, want %v", code, set.inputMethods, names)
		}
	}
	for code, set := range imeTable {
		if len(set.inputMethods) == 0 {
			t.Errorf("row %q installs %v but names no engine a session can switch to", code, set.packages)
		}
		for _, name := range set.inputMethods {
			if strings.TrimSpace(name) == "" {
				t.Errorf("row %q names a blank engine", code)
			}
			if strings.HasPrefix(name, shared.Fcitx5KeyboardIMPrefix) {
				t.Errorf("row %q names %q: a keyboard layout comes from the location table, never from a language row", code, name)
			}
		}
	}
}

// TestIMEngineNames_OnlyWhatWasInstalled: the session list reads the package set
// the desktop step recorded rather than the language, so an engine this install
// never got is never named — by --input-method none, and by a context that never
// ran the desktop step at all (a standalone --verify-system).
func TestIMEngineNames_OnlyWhatWasInstalled(t *testing.T) {
	installed := &shared.InstallContext{Locale: "ja_JP.UTF-8"}
	ChooseIMEAddons(installed)
	if got := IMEngineNames(installed); !slices.Equal(got, []string{"mozc"}) {
		t.Errorf("IMEngineNames(ja_JP) = %v, want [mozc]", got)
	}

	off := &shared.InstallContext{Locale: "ja_JP.UTF-8", InputMethod: InputMethodOverrideNone}
	ChooseIMEAddons(off)
	if got := IMEngineNames(off); got != nil {
		t.Errorf("--input-method none still names engines: %v", got)
	}

	fresh := &shared.InstallContext{Locale: "ja_JP.UTF-8"}
	if got := IMEngineNames(fresh); got != nil {
		t.Errorf("a context that never chose anything names engines: %v", got)
	}
}

// --- SessionInputMethods ------------------------------------------------------

// TestSessionInputMethods is the feature as a person experiences it: English first
// and their own language second, whatever the pair of wizard answers was. Note how
// the two inputs split the work — the location decides which keyboard is offered,
// the language decides which engine is, and a CJK install gets the engine beside
// the keyboard its hardware actually has.
func TestSessionInputMethods(t *testing.T) {
	cases := []struct {
		name   string
		locale string
		region string
		want   []string
	}{
		{"an English install in the US", "en_US.UTF-8", "us", []string{"keyboard-us"}},
		{"the worldwide location", "en_US.UTF-8", "worldwide", []string{"keyboard-us"}},
		{"Russian language, Russian location", "ru_RU.UTF-8", "ru", []string{"keyboard-us", "keyboard-ru"}},
		{"Ukrainian language, Ukrainian location", "uk_UA.UTF-8", "ua", []string{"keyboard-us", "keyboard-ua"}},
		{"a layout the console table spells differently", "de_DE.UTF-8", "de", []string{"keyboard-us", "keyboard-de"}},
		{"Latin American keyboards are not Spain's", "es_ES.UTF-8", "ar", []string{"keyboard-us", "keyboard-latam"}},
		{"a keyboard the console table has no map for", "en_US.UTF-8", "th", []string{"keyboard-us", "keyboard-th"}},
		{"an engine on a US keyboard", "zh_CN.UTF-8", "cn", []string{"keyboard-us", "pinyin"}},
		{"an engine and the keyboard the hardware has", "ja_JP.UTF-8", "jp", []string{"keyboard-us", "keyboard-jp", "mozc"}},
		{"the engine is the local input method", "vi_VN.UTF-8", "vn", []string{"keyboard-us", "bamboo"}},
		{"the location decides the layout, not the language", "en_US.UTF-8", "ua", []string{"keyboard-us", "keyboard-ua"}},
		{"the language decides the engine, not the location", "zh_CN.UTF-8", "us", []string{"keyboard-us", "pinyin"}},
	}
	for _, c := range cases {
		ctx := &shared.InstallContext{Locale: c.locale, Region: shared.GetRegion(c.region)}
		ChooseIMEAddons(ctx) // the desktop step's decision, which the list reads back
		got := SessionInputMethods(ctx)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: SessionInputMethods(%s, %s) = %v, want %v", c.name, c.locale, c.region, got, c.want)
		}
	}
}

// TestSessionInputMethods_Invariants holds the two properties every case must have,
// over every language and location the installer offers: English is first, because
// the framework serves that entry to a session with no input method active, and no
// entry is listed twice — a duplicate would read as a language that takes two keys
// to leave.
func TestSessionInputMethods_Invariants(t *testing.T) {
	english := shared.Fcitx5KeyboardIMPrefix + shared.DefaultXkbLayout
	for _, lang := range shared.GetLanguages() {
		for _, region := range shared.GetRegions() {
			ctx := &shared.InstallContext{Locale: lang.Locale, Region: region}
			ChooseIMEAddons(ctx)
			got := SessionInputMethods(ctx)
			if len(got) == 0 || got[0] != english {
				t.Errorf("%s in %s: the list does not start with %s: %v", lang.Code, region.Code, english, got)
			}
			if len(got) > 3 {
				t.Errorf("%s in %s: %d input methods is more than English, a keyboard and an engine: %v", lang.Code, region.Code, len(got), got)
			}
			for i, name := range got {
				if slices.Contains(got[i+1:], name) {
					t.Errorf("%s in %s: %s is listed twice: %v", lang.Code, region.Code, name, got)
				}
			}
		}
	}
}

// TestSessionInputMethods_NoneOverride: forcing no engines must not leave the
// engine in the list. The location's keyboard is a different decision and stays.
func TestSessionInputMethods_NoneOverride(t *testing.T) {
	ctx := &shared.InstallContext{Locale: "ja_JP.UTF-8", Region: shared.GetRegion("jp"), InputMethod: InputMethodOverrideNone}
	ChooseIMEAddons(ctx)
	want := []string{"keyboard-us", "keyboard-jp"}
	if got := SessionInputMethods(ctx); !slices.Equal(got, want) {
		t.Errorf("SessionInputMethods with --input-method none = %v, want %v", got, want)
	}
}

// TestSessionInputMethods_LastEntryIsTheNativeOne: the profile makes the last
// entry the one the framework switches an active context to, so the order is what
// makes Alt+Shift mean "my language" rather than "English again".
func TestSessionInputMethods_LastEntryIsTheNativeOne(t *testing.T) {
	for _, c := range []struct {
		locale, region string
	}{
		{"ru_RU.UTF-8", "ru"},
		{"ja_JP.UTF-8", "jp"},
		{"zh_CN.UTF-8", "cn"},
	} {
		ctx := &shared.InstallContext{Locale: c.locale, Region: shared.GetRegion(c.region)}
		ChooseIMEAddons(ctx)
		got := SessionInputMethods(ctx)
		last := got[len(got)-1]
		if last == shared.Fcitx5KeyboardIMPrefix+shared.DefaultXkbLayout {
			t.Errorf("%s in %s: the entry the switch key reaches is English: %v", c.locale, c.region, got)
		}
	}
}

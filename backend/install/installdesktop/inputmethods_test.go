package installdesktop

import (
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

// TestIMESummary_MatchesTheRows checks what the Confirmation page shows: one
// name per engine the row installs, proper nouns, nothing for a language the
// keyboard covers.
func TestIMESummary_MatchesTheRows(t *testing.T) {
	if got := IMESummary("en_US"); got != nil {
		t.Errorf("a Latin language got engine names: %v", got)
	}
	for code, set := range imeTable {
		got := IMESummary(code)
		if len(got) == 0 {
			t.Errorf("IME summary for %q is empty although its row installs %v", code, set.packages)
		}
		for _, name := range got {
			if strings.TrimSpace(name) == "" {
				t.Errorf("IME summary for %q contains a blank name", code)
			}
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

package installdesktop

// Input method selection.
//
// The Fcitx5 core stack ships on every install — it is part of DesktopPackages,
// and the seeded Hyprland config and the aelum-fcitx5 session unit (live-ISO
// source, copied by install/isocopy) point at it unconditionally. What the
// language decides is the engine addons: the same shape as drivers.go picks the
// GPU set from the PCI bus, this picks the input methods from the language the
// person chose. Latin, Cyrillic, Greek, Arabic, Hebrew and Persian are typed
// through keyboard layouts, which Hyprland already handles (input.kb_layout
// plus grp:alt_shift_toggle); an engine is only needed where typing a word is
// not typing keys — Pinyin and Zhuyin for Chinese, Mozc for Japanese, Hangul
// for Korean, Telex-style Vietnamese.
//
// A core without addons is not dead weight: it is what makes "add an input
// method later" a plain package install — the environment and the autostart are
// already there, and a first-run fcitx5 registers whatever engine appears.
//
// selectIMEAddons is the pure table, ChooseIMEAddons is what the desktop step
// calls at its start, and the chosen set is recorded on ctx.IMPackages so the
// verify step checks exactly what was installed. The language is read from
// ctx.Locale — the locale is the language (see shared/types.go); the location
// is a separate input and changes nothing here.

import (
	"strings"

	"aelum-installer/backend/shared"
)

// InputMethodOverride* are the accepted --input-method values. "auto" follows
// the language table; "none" installs no addons for any language. It is the
// escape hatch for testing a Latin-language machine with CJK input (or the
// reverse), the same role --gpu-driver plays for the driver table. The GUI
// never sets it.
const (
	InputMethodOverrideAuto = "auto"
	InputMethodOverrideNone = "none"
)

// ValidInputMethodOverride reports whether s is an accepted --input-method
// value. The empty string is accepted: it means "auto".
func ValidInputMethodOverride(s string) bool {
	switch s {
	case InputMethodOverrideAuto, InputMethodOverrideNone, "":
		return true
	}
	return false
}

// imeSet is one row of the language table: the addon packages the language
// needs, and the engine names the Confirmation page shows. The summary names
// are proper nouns — the same words in every language — so they need no
// translation, and they can never drift from the packages because both come
// from this one row.
type imeSet struct {
	packages []string
	summary  []string
}

// The language rows, keyed by the same codes shared.GetLanguages offers (which
// is what --language and the GUI carry). Package names are as Arch spells them.
//
//   - zh_CN: chinese-addons carries the native Pinyin/Shuangpin/Wubi engines.
//   - zh_TW: chewing is the Zhuyin engine most of that market types with;
//     table-extra adds Cangjie (and the other Chinese tables). It depends on
//     chinese-addons, which the mirror carries anyway for zh_CN.
//   - ja_JP: mozc — the engine Japanese users expect. anthy still packages but
//     is unmaintained, and kkc is a niche kana-conversion tool.
//   - ko_KR: hangul (libhangul).
//   - vi_VN: bamboo — the modern Vietnamese engine (unikey is the older one).
var imeTable = map[string]imeSet{
	"zh_CN": {
		packages: []string{"fcitx5-chinese-addons"},
		summary:  []string{"Pinyin"},
	},
	"zh_TW": {
		packages: []string{"fcitx5-chewing", "fcitx5-table-extra"},
		summary:  []string{"Zhuyin", "Cangjie"},
	},
	"ja_JP": {
		packages: []string{"fcitx5-mozc"},
		summary:  []string{"Mozc"},
	},
	"ko_KR": {
		packages: []string{"fcitx5-hangul"},
		summary:  []string{"Hangul"},
	},
	"vi_VN": {
		packages: []string{"fcitx5-bamboo"},
		summary:  []string{"Bamboo"},
	},
}

// IMCandidatePackages is the union of everything the table can produce.
// backend/scripts/pkg-list.go prints it into the ISO build's package list:
// an addon chosen at install time that no list carries cannot be installed
// offline, so every name the table might pick must be in the mirror — the same
// invariant as DriverCandidatePackages.
var IMCandidatePackages = unionStrings(
	imeTable["zh_CN"].packages,
	imeTable["zh_TW"].packages,
	imeTable["ja_JP"].packages,
	imeTable["ko_KR"].packages,
	imeTable["vi_VN"].packages,
)

// languageFromLocale takes the language code ("zh_CN") out of an installed
// system locale ("zh_CN.UTF-8"). The locale is the language — the region is a
// separate input (see shared/types.go) — so this is the whole mapping.
func languageFromLocale(locale string) string {
	if i := strings.IndexByte(locale, '.'); i >= 0 {
		return locale[:i]
	}
	return locale
}

// selectIMEAddons is the pure half of the decision: the addon set for a
// language, or nil when that language is typed through keyboard layouts. That
// nil is a real outcome, not a fallback — 23 of the 28 languages need no
// engine, and an install that gets none still has the core framework.
func selectIMEAddons(language string) []string {
	set, ok := imeTable[language]
	if !ok {
		return nil
	}
	return append([]string{}, set.packages...)
}

// IMESummary returns the engine names this language's install gets, for the
// Confirmation page: proper nouns, shown after the framework name. Empty for a
// language that needs no engine — the page still names the core, which every
// install gets.
func IMESummary(language string) []string {
	set, ok := imeTable[language]
	if !ok {
		return nil
	}
	return append([]string{}, set.summary...)
}

// ChooseIMEAddons decides the addon set for this install, records it on
// ctx.IMPackages, and returns it. A set already recorded (a re-run of the
// step, or a test) is reused unchanged. An empty recorded set is a decision,
// not a missing one, so the check is for nil rather than for length — and the
// table is pure, so recomputing would be harmless either way.
func ChooseIMEAddons(ctx *shared.InstallContext) []string {
	if ctx.IMPackages != nil {
		return ctx.IMPackages
	}

	language := languageFromLocale(ctx.Locale)
	var packages []string
	switch ctx.InputMethod {
	case InputMethodOverrideNone:
		packages = []string{}
	default: // "" or InputMethodOverrideAuto
		packages = selectIMEAddons(language)
		if packages == nil {
			// Getting no addons is a decision, and it is recorded as one:
			// a non-nil empty set is what a step re-run and the verify step
			// both read back as "the table said none".
			packages = []string{}
		}
	}

	ctx.IMPackages = packages
	logIMEAddons(ctx, language, packages)
	return packages
}

// logIMEAddons records the decision in the install log. "none" is recorded too:
// a Latin-language install must be able to show it got exactly what the table
// said, and an IME-language install that got none because of --input-method
// none must not look like a silent failure.
func logIMEAddons(ctx *shared.InstallContext, language string, packages []string) {
	if ctx.Logger == nil {
		return
	}
	if len(packages) == 0 {
		ctx.Logger.Info("input method %s → none (keyboard layouts)", language)
		return
	}
	ctx.Logger.Info("input method %s → %s", language, strings.Join(packages, " "))
}

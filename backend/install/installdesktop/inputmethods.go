package installdesktop

// Input method selection.
//
// The Fcitx5 core stack ships on every install — it is part of DesktopPackages,
// and the seeded Hyprland config and the aelum-fcitx5 session unit (live-ISO
// source, copied by install/isocopy) point at it unconditionally. What the two
// location inputs decide is what sits on top of that core: the same shape as
// drivers.go picks the GPU set from the PCI bus, this picks input methods from
// the language and the location the person chose. The language chooses an engine,
// and only where typing a word is not typing keys — Pinyin and Zhuyin for
// Chinese, Mozc for Japanese, Hangul for Korean, Telex-style Vietnamese. The
// location chooses a keyboard layout, which is what Cyrillic, Greek, Arabic,
// Hebrew, Thai and the rest need and what a US-layout machine does not.
// SessionInputMethods composes the two into the list the session starts with.
//
// A core without addons is not dead weight: it is what makes "add an input
// method later" a plain package install — the environment, the autostart and the
// configuration are already there, and the framework registers whatever engine
// appears.
//
// selectIMEAddons is the pure table, ChooseIMEAddons is what the desktop step
// calls at its start, and the chosen set is recorded on ctx.IMPackages so the
// verify step checks exactly what was installed. The language is read from
// ctx.Locale — the locale is the language (see shared/types.go); the location is
// ctx.Region, and its desktop layout comes from shared.XkbLayoutByISO.

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

// imeSet is one row of the language table: the addon packages the language needs
// and the engine names those packages register, which is the same fact stated
// twice — an engine the session list names and an engine the row installs must be
// the same engine — and spelled for a person in imeDisplayNames.
type imeSet struct {
	packages []string

	// inputMethods are those engines as the framework spells them: the unique
	// name of the input method the package registers, which is the only thing a
	// session's input method list can name. They are the basenames of the .conf
	// files each addon ships under /usr/share/fcitx5/inputmethod — read out of
	// the current Arch packages, not guessed. A row names one engine even where
	// the package registers more tables than that: the list is the thing a person
	// switches to, and the rest are reachable through the input method config
	// tool.
	inputMethods []string
}

// The language rows, keyed by the same codes shared.GetLanguages offers (which
// is what --language and the GUI carry). Package names are as Arch spells them.
//
//   - zh_CN: chinese-addons carries the native Pinyin/Shuangpin/Wubi engines.
//   - zh_TW: chewing is the Zhuyin engine most of that market types with;
//     table-extra adds the other Chinese tables (its own AGENTS note called it
//     Cangjie — measured against the packages, plain `cangjie` ships inside
//     chinese-addons, which table-extra depends on, and table-extra itself has
//     cangjie-large/-3/-5 and the rest).
//   - ja_JP: mozc — the engine Japanese users expect. anthy still packages but
//     is unmaintained, and kkc is a niche kana-conversion tool.
//   - ko_KR: hangul (libhangul).
//   - vi_VN: bamboo — the modern Vietnamese engine (unikey is the older one).
var imeTable = map[string]imeSet{
	"zh_CN": {
		packages:     []string{"fcitx5-chinese-addons"},
		inputMethods: []string{"pinyin"},
	},
	"zh_TW": {
		packages:     []string{"fcitx5-chewing", "fcitx5-table-extra"},
		inputMethods: []string{"chewing"},
	},
	"ja_JP": {
		packages:     []string{"fcitx5-mozc"},
		inputMethods: []string{"mozc"},
	},
	"ko_KR": {
		packages:     []string{"fcitx5-hangul"},
		inputMethods: []string{"hangul"},
	},
	"vi_VN": {
		packages:     []string{"fcitx5-bamboo"},
		inputMethods: []string{"bamboo"},
	},
}

// imeDisplayNames spells an engine's framework name in the words a person reads
// for it. They are proper nouns — the same words in every language — which is why
// the Confirmation page can show them untranslated.
var imeDisplayNames = map[string]string{
	"pinyin":  "Pinyin",
	"chewing": "Zhuyin",
	"mozc":    "Mozc",
	"hangul":  "Hangul",
	"bamboo":  "Bamboo",
}

// xkbLayoutNames spells a layout code from shared.XkbLayoutByISO in the words a
// person reads for it. Each value is xkeyboard-config's own <description> for that
// layout — the same words fcitx5's switcher shows for the same entry — read out of
// /usr/share/X11/xkb/rules/evdev.xml rather than written from memory;
// TestXkbLayoutNames_AreEvdevDescriptions holds the table against that file
// wherever xkeyboard-config is installed.
//
// English, like the location names the same page already shows: a keyboard layout
// is a thing in the world (the keycaps on the box), not a piece of UI.
var xkbLayoutNames = map[string]string{
	"us":    "English (US)",
	"latam": "Spanish (Latin American)",
	"at":    "German (Austria)",
	"be":    "Belgian",
	"bg":    "Bulgarian",
	"br":    "Portuguese (Brazil)",
	"ca":    "French (Canada)",
	"ch":    "German (Switzerland)",
	"cz":    "Czech",
	"de":    "German",
	"dk":    "Danish",
	"ee":    "Estonian",
	"es":    "Spanish",
	"fi":    "Finnish",
	"fr":    "French",
	"gb":    "English (UK)",
	"ge":    "Georgian",
	"gr":    "Greek",
	"hu":    "Hungarian",
	"il":    "Hebrew",
	"is":    "Icelandic",
	"it":    "Italian",
	"jp":    "Japanese",
	"kr":    "Korean",
	"kz":    "Kazakh",
	"lt":    "Lithuanian",
	"lv":    "Latvian",
	"mk":    "Macedonian",
	"nl":    "Dutch",
	"no":    "Norwegian",
	"pl":    "Polish",
	"pt":    "Portuguese",
	"ro":    "Romanian",
	"rs":    "Serbian",
	"ru":    "Russian",
	"se":    "Swedish",
	"si":    "Slovenian",
	"sk":    "Slovak",
	"th":    "Thai",
	"tr":    "Turkish",
	"ua":    "Ukrainian",
}

// xkbLayoutName and imeDisplayName name one input method for a person to read. An
// entry the tables do not carry falls back to the code it was asked about: a gap
// in a table should read as an odd label, never as an input method that is
// missing from the answer.
func xkbLayoutName(layout string) string {
	if name, ok := xkbLayoutNames[layout]; ok {
		return name
	}
	return layout
}

func imeDisplayName(method string) string {
	if name, ok := imeDisplayNames[method]; ok {
		return name
	}
	return method
}

// SessionInputMethodNames is the Confirmation page's answer: the input methods the
// session for this language and location will offer, named in words a person
// recognises, in the order the session gets them — "English (US)" and
// "Ukrainian" for a machine in Ukraine, "Japanese" and "Mozc" for one in Japan.
//
// It reads the two tables directly rather than a recorded package set, because at
// confirmation time nothing is recorded yet: the desktop step has not run, and
// `--input-method none` has not been decided against a context. It is a preview of
// the same decision, and TestSessionInputMethodNames_MatchesTheSessionList holds
// it entry for entry against what SessionInputMethods actually writes.
func SessionInputMethodNames(language, regionCode string) []string {
	im := []string{xkbLayoutName(shared.DefaultXkbLayout)}
	if layout := shared.XkbLayoutForRegion(regionCode); layout != shared.DefaultXkbLayout {
		im = append(im, xkbLayoutName(layout))
	}
	for _, method := range imeTable[language].inputMethods {
		im = append(im, imeDisplayName(method))
	}
	return im
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

// IMEngineNames returns the engines this install has, in the framework's own
// names, for the session's input method list. It reads the package set the
// desktop step recorded rather than re-asking the language, because the list must
// never name an engine the install never installed: --input-method none records an
// empty set, and an entry the framework cannot resolve is quietly dropped from the
// group, which reads as a missing language rather than as a decision.
func IMEngineNames(ctx *shared.InstallContext) []string {
	if len(ctx.IMPackages) == 0 {
		return nil
	}
	set, ok := imeTable[languageFromLocale(ctx.Locale)]
	if !ok {
		return nil
	}
	return append([]string{}, set.inputMethods...)
}

// SessionInputMethods is the input method list every account on this install
// starts with, in the order the framework reads it:
//
//  1. English, first. The framework uses the first entry of the group as the
//     state a session is in when the input method is switched off — for an
//     inactive context it serves inputMethodList()[0] — so the language a person
//     did not choose is always the plain keyboard, and password fields and a
//     session without the framework behave the same way.
//  2. The location's keyboard layout, when the keyboards sold there are not the US
//     layout: Russian Cyrillic, Ukrainian, Thai, a Japanese 106-key board. A
//     layout is an input method to this framework ("keyboard-<layout>", one entry
//     per xkeyboard-config layout), which is what lets a single key switch a
//     layout and an engine with the same press.
//  3. The engine the language table chose, when it chose one.
//
// So the ordinary case is exactly two — English and the language of the place the
// person said they are in — and a CJK install adds its engine beside the keyboard
// its hardware actually carries. The last entry is the one the toggle switches to,
// which is what the configure step writes as the group's default.
func SessionInputMethods(ctx *shared.InstallContext) []string {
	im := []string{shared.Fcitx5KeyboardIMPrefix + shared.DefaultXkbLayout}
	if layout := shared.XkbLayoutForRegion(ctx.Region.Code); layout != shared.DefaultXkbLayout {
		im = append(im, shared.Fcitx5KeyboardIMPrefix+layout)
	}
	return append(im, IMEngineNames(ctx)...)
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

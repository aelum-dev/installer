package configure

// The session's input method list.
//
// The desktop step decides which input methods an install *has* — the fcitx5 core
// always, the language's engine when the language needs one
// (installdesktop/inputmethods.go). What it does not decide is which of them a
// session offers and in what order, and a first-run framework would answer that
// by itself: a group containing one entry, `keyboard-us`, whatever the machine's
// language. This step writes the answer instead.
//
// The file is system-side (/etc/xdg/fcitx5/profile, and the framework's own
// config beside it) rather than seeded through /etc/skel, because nothing in it is
// per-user: an install has one language and one location. fcitx5 reads its
// PkgConfig path list in order — ~/.config/fcitx5, then /etc/xdg/fcitx5 — so this
// is the default every account starts with, including accounts created later, and a
// person's own settings take over from the moment they change any (the framework
// autosaves a copy into the home).
//
// Like every other file the desktop needs, it is written before the "after
// installation" snapshot: a snapshot runs the configuration it contains.

import (
	"fmt"
	"strings"

	"aelum-installer/backend/install/installdesktop"
	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// fcitx5GroupName is the name of the single input method group. The framework
// requires a non-empty name and a non-empty default layout for a group to be
// accepted at all — a group that has neither is dropped with a warning and the
// built-in single-entry default comes back — so both are always written.
const fcitx5GroupName = "Default"

// ConfigureInputMethod writes the input method list this install's sessions start
// with: English first, then the layout of the location, then the engine of the
// language. See installdesktop.SessionInputMethods for why that order is the one
// the framework gives the intended behaviour with.
func ConfigureInputMethod(ctx *shared.InstallContext) error {
	im := installdesktop.SessionInputMethods(ctx)

	if err := primitives.WriteFile(shared.Fcitx5ProfilePath, fcitx5Profile(im)); err != nil {
		return fmt.Errorf("failed to configure the input methods: %w", err)
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("input method list: %s (switch with Alt+Shift)", strings.Join(im, " "))
	}
	return nil
}

// fcitx5Profile renders the list as the framework's own INI format. The shape is
// what its InputMethodManager reads and writes: a `Groups/0` section for the
// group, one `Groups/0/Items/<n>` section per input method, and a `GroupOrder`
// listing the group by name — the same file fcitx5 saves when the user edits it,
// which is what makes a hand-written one safe.
//
// `DefaultIM` is the last entry, not the first: the framework switches an
// *inactive* context (which is how every session starts, ActiveByDefault being
// off) to the first entry — English — and an active one to DefaultIM, so the last
// entry is the thing the toggle key reaches and English is the state a session is
// in when nobody is using an input method. Password fields get the same treatment
// through `Default Layout`, which is the layout name for the keyboard a disabled
// context falls back to; keeping it the English one is what makes a passphrase
// field type the same way at the greeter and in the session.
//
// An item's own `Layout` stays empty on purpose. The framework takes a
// `keyboard-<layout>` entry's layout from its name when the field is empty, and
// only overrides the system layout when the two differ — so English keeps using
// the layout the compositor was started with and the local layout is the one that
// needs its own.
func fcitx5Profile(im []string) string {
	if len(im) == 0 {
		im = []string{shared.Fcitx5KeyboardIMPrefix + shared.DefaultXkbLayout}
	}

	var b strings.Builder
	b.WriteString(fcitx5ProfileHeader(im))

	b.WriteString("\n[Groups/0]\n")
	b.WriteString("Name=" + fcitx5GroupName + "\n")
	b.WriteString("Default Layout=" + shared.DefaultXkbLayout + "\n")
	b.WriteString("DefaultIM=" + im[len(im)-1] + "\n")

	for i, name := range im {
		fmt.Fprintf(&b, "\n[Groups/0/Items/%d]\nName=%s\nLayout=\n", i, name)
	}

	b.WriteString("\n[GroupOrder]\n0=" + fcitx5GroupName + "\n")
	return b.String()
}

// fcitx5ProfileHeader explains the file to whoever opens it on an installed
// system, and names what this install chose: the file is read-only evidence of a
// decision made from two wizard answers, and a reader who did not run the wizard
// should not have to work out which answer produced which line.
func fcitx5ProfileHeader(im []string) string {
	return strings.Join([]string{
		"# Aelum: the input methods every account on this system starts with.",
		"#",
		"# English first, then the local one: " + strings.Join(im, ", ") + ".",
		"# Alt+Shift switches between them — the key is set in the config file",
		"# beside this one, and it is the framework's key, not the compositor's:",
		"# a keyboard layout is an input method to this framework, so one key",
		"# switches layouts and engines alike.",
		"#",
		"# fcitx5 reads ~/.config/fcitx5/profile first and this file otherwise, so",
		"# editing the input methods in the menu makes the framework write its own",
		"# copy in the home and that one wins from then on.",
	}, "\n")
}

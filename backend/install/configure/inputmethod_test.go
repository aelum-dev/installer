package configure

// Writing the session's input method list.
//
// These tests read the rendered file back rather than matching a golden string:
// the shape of it is what fcitx5's InputMethodManager parses (a Groups/0 section,
// one Groups/0/Items/<n> section per input method, a GroupOrder naming the group),
// and the two properties that carry the feature are the *order* of the items and
// which one DefaultIM names. The header text is prose and is free to change.

import (
	"errors"
	"strings"
	"testing"

	"aelum-installer/backend/install/installdesktop"
	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// profileValue reads one key out of the rendered file, by its path in the
// framework's own section syntax ("Groups/0/DefaultIM").
func profileValue(t *testing.T, body, section, key string) string {
	t.Helper()
	inSection := false
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "["+section+"]":
			inSection = true
		case strings.HasPrefix(line, "["):
			inSection = false
		case inSection && strings.HasPrefix(line, key+"="):
			return strings.TrimPrefix(line, key+"=")
		}
	}
	return ""
}

// profileItems reads the input method names back in the order their Items
// sections appear — the order the framework serves them, and the whole point of
// this file.
func profileItems(t *testing.T, body string) []string {
	t.Helper()
	var names []string
	inItems := false
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "[Groups/0/Items/"):
			inItems = true
		case strings.HasPrefix(line, "["):
			inItems = false
		case inItems && strings.HasPrefix(line, "Name="):
			names = append(names, strings.TrimPrefix(line, "Name="))
		}
	}
	return names
}

// writeProfile runs the step against an in-memory target and returns what it wrote.
func writeProfile(t *testing.T, ctx *shared.InstallContext) string {
	t.Helper()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := ConfigureInputMethod(ctx); err != nil {
		t.Fatalf("ConfigureInputMethod: %v", err)
	}
	body, ok := fs.Read(shared.MountRoot + shared.Fcitx5ProfilePath)
	if !ok {
		t.Fatalf("no profile at %s", shared.MountRoot+shared.Fcitx5ProfilePath)
	}
	return body
}

// TestConfigureInputMethod_RussianPair is the feature in one case: English first,
// Russian second, and the switch key lands on Russian.
func TestConfigureInputMethod_RussianPair(t *testing.T) {
	ctx := &shared.InstallContext{Locale: "ru_RU.UTF-8", Region: shared.GetRegion("ru")}
	body := writeProfile(t, ctx)

	want := []string{"keyboard-us", "keyboard-ru"}
	if got := profileItems(t, body); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("items = %v, want %v", got, want)
	}
	if got := profileValue(t, body, "Groups/0", "DefaultIM"); got != "keyboard-ru" {
		t.Errorf("DefaultIM = %q, want keyboard-ru: that is the entry the switch key reaches", got)
	}
	if got := profileValue(t, body, "Groups/0", "Default Layout"); got != shared.DefaultXkbLayout {
		t.Errorf("Default Layout = %q, want %s: it is the layout a disabled context (a password field) falls back to", got, shared.DefaultXkbLayout)
	}
	if got := profileValue(t, body, "GroupOrder", "0"); got != fcitx5GroupName {
		t.Errorf("GroupOrder names %q, want the group %q", got, fcitx5GroupName)
	}
}

// TestConfigureInputMethod_JapaneseInstall: a machine whose keyboards are not the
// US layout and whose language needs an engine lists all three, with the engine
// last — the keyboard is what its English is, and the engine is what the key is for.
func TestConfigureInputMethod_JapaneseInstall(t *testing.T) {
	ctx := &shared.InstallContext{Locale: "ja_JP.UTF-8", Region: shared.GetRegion("jp")}
	installdesktop.ChooseIMEAddons(ctx) // the desktop step runs first in a real install
	body := writeProfile(t, ctx)

	want := []string{"keyboard-us", "keyboard-jp", "mozc"}
	if got := profileItems(t, body); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("items = %v, want %v", got, want)
	}
	if got := profileValue(t, body, "Groups/0", "DefaultIM"); got != "mozc" {
		t.Errorf("DefaultIM = %q, want mozc", got)
	}
}

// TestConfigureInputMethod_EnglishOnly: an English install in an English-speaking
// place still gets a valid file. The framework builds the same group itself when
// there is none, so writing it is about one code path and about the file being
// there to read and edit — not about a second language that does not exist.
func TestConfigureInputMethod_EnglishOnly(t *testing.T) {
	ctx := &shared.InstallContext{Locale: "en_US.UTF-8", Region: shared.GetRegion("us")}
	body := writeProfile(t, ctx)

	if got := profileItems(t, body); strings.Join(got, ",") != "keyboard-us" {
		t.Errorf("items = %v, want only keyboard-us", got)
	}
	if got := profileValue(t, body, "Groups/0", "DefaultIM"); got != "keyboard-us" {
		t.Errorf("DefaultIM = %q, want keyboard-us", got)
	}
}

// TestConfigureInputMethod_NoneOverride: --input-method none installed no engine,
// so the profile must not name one. The location's keyboard is a separate
// decision and stays.
func TestConfigureInputMethod_NoneOverride(t *testing.T) {
	ctx := &shared.InstallContext{
		Locale:      "ja_JP.UTF-8",
		Region:      shared.GetRegion("jp"),
		InputMethod: installdesktop.InputMethodOverrideNone,
	}
	installdesktop.ChooseIMEAddons(ctx)
	body := writeProfile(t, ctx)

	want := []string{"keyboard-us", "keyboard-jp"}
	if got := profileItems(t, body); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("items = %v, want %v", got, want)
	}
}

// TestFcitx5Profile_NeverWritesAnEmptyGroup: the framework drops a group whose
// name or default layout is empty and falls back to its own built-in default, so a
// malformed file is not a smaller list but no list at all.
func TestFcitx5Profile_NeverWritesAnEmptyGroup(t *testing.T) {
	for _, in := range [][]string{nil, {}} {
		body := fcitx5Profile(in)
		if got := profileItems(t, body); strings.Join(got, ",") != "keyboard-us" {
			t.Errorf("fcitx5Profile(%v) items = %v, want keyboard-us", in, got)
		}
		if profileValue(t, body, "Groups/0", "Name") != fcitx5GroupName {
			t.Error("the group has no name; the framework would discard it")
		}
		if profileValue(t, body, "Groups/0", "Default Layout") == "" {
			t.Error("the group has no default layout; the framework would discard it")
		}
	}
}

// TestConfigureInputMethod_WriteFailure: a target that will not take the file is
// an error naming the feature, not a silently missing language.
func TestConfigureInputMethod_WriteFailure(t *testing.T) {
	fs := primitivestest.NewMemFS().
		WithWriteError(shared.MountRoot+shared.Fcitx5ProfilePath, errors.New("read-only file system"))
	defer primitives.SetFileSystem(fs)()

	err := ConfigureInputMethod(&shared.InstallContext{Locale: "uk_UA.UTF-8", Region: shared.GetRegion("ua")})
	if err == nil {
		t.Fatal("a failed write was not reported")
	}
	if !strings.Contains(err.Error(), "input method") {
		t.Errorf("error = %q, want it to name the input methods", err)
	}
}

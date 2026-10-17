package gui

import (
	"testing"
)

func TestNewApp_Defaults(t *testing.T) {
	app := NewApp(false)

	if got := app.GetCurrentPage(); got != PageWelcome.Value {
		t.Errorf("GetCurrentPage() = %d, want %d (welcome)", got, PageWelcome.Value)
	}
	if app.GetDryRun() {
		t.Error("GetDryRun() = true, want false")
	}
	if got := app.GetStartPage(); got != -1 {
		t.Errorf("GetStartPage() = %d, want -1 when unset", got)
	}
}

func TestSetCurrentPage(t *testing.T) {
	app := NewApp(false)

	// Every valid page is accepted and read back.
	for _, page := range []PageID{
		PageWelcome, PageUserAccount, PageDiskSelection,
		PageConfirmation, PageInstallation, PageFinish,
	} {
		if err := app.SetCurrentPage(page); err != nil {
			t.Errorf("SetCurrentPage(%d) returned error: %v", page.Value, err)
		}
		if got := app.GetCurrentPage(); got != page.Value {
			t.Errorf("after SetCurrentPage(%d), GetCurrentPage() = %d", page.Value, got)
		}
	}

	// Out-of-range pages are rejected and must not mutate state.
	app.SetCurrentPage(PageDiskSelection)
	for _, invalid := range []PageID{{Value: -1}, {Value: 6}, {Value: 99}} {
		if err := app.SetCurrentPage(invalid); err == nil {
			t.Errorf("SetCurrentPage(%d) = nil, want error", invalid.Value)
		}
		if got := app.GetCurrentPage(); got != PageDiskSelection.Value {
			t.Errorf("invalid SetCurrentPage(%d) mutated state to %d", invalid.Value, got)
		}
	}
}

func TestStartPage_RoundTrip(t *testing.T) {
	app := NewApp(false)

	if err := app.SetStartPage(PageConfirmation); err != nil {
		t.Fatalf("SetStartPage(confirmation) error: %v", err)
	}
	if got := app.GetStartPage(); got != PageConfirmation.Value {
		t.Errorf("GetStartPage() = %d, want %d", got, PageConfirmation.Value)
	}

	// ClearStartPage resets to the sentinel -1.
	app.ClearStartPage()
	if got := app.GetStartPage(); got != -1 {
		t.Errorf("after ClearStartPage, GetStartPage() = %d, want -1", got)
	}

	// Out-of-range start pages are rejected and leave state untouched.
	if err := app.SetStartPage(PageInstallation); err != nil {
		t.Fatalf("SetStartPage(installation) error: %v", err)
	}
	for _, invalid := range []PageID{{Value: -1}, {Value: 6}} {
		if err := app.SetStartPage(invalid); err == nil {
			t.Errorf("SetStartPage(%d) = nil, want error", invalid.Value)
		}
	}
	if got := app.GetStartPage(); got != PageInstallation.Value {
		t.Errorf("invalid SetStartPage mutated state to %d", got)
	}
}

func TestHostname_RoundTrip(t *testing.T) {
	app := NewApp(false)

	if got := app.GetHostname(); got != "" {
		t.Errorf("default GetHostname() = %q, want empty", got)
	}
	app.SetHostname("aelum-box")
	if got := app.GetHostname(); got != "aelum-box" {
		t.Errorf("GetHostname() = %q, want %q", got, "aelum-box")
	}
}

func TestUILocale_RoundTrip(t *testing.T) {
	app := NewApp(false)

	if got := app.GetUILocale(); got != "" {
		t.Errorf("default GetUILocale() = %q, want empty", got)
	}
	app.SetUILocale("en-US")
	if got := app.GetUILocale(); got != "en-US" {
		t.Errorf("GetUILocale() = %q, want %q", got, "en-US")
	}
	app.SetUILocale("ar-SA")
	if got := app.GetUILocale(); got != "ar-SA" {
		t.Errorf("GetUILocale() = %q, want %q", got, "ar-SA")
	}
}

func TestSetLvm(t *testing.T) {
	app := NewApp(false)

	if app.lvm {
		t.Error("lvm should default to false")
	}
	app.SetLvm(true)
	if !app.lvm {
		t.Error("SetLvm(true) did not enable lvm")
	}
	app.SetLvm(false)
	if app.lvm {
		t.Error("SetLvm(false) did not disable lvm")
	}
}

func TestSetLuksPassphrase_StoresCopy(t *testing.T) {
	app := NewApp(false)

	original := []byte("correct horse battery staple")
	app.SetLuksPassphrase(string(original))
	if string(app.luksPassphrase) != string(original) {
		t.Errorf("stored passphrase = %q, want %q", app.luksPassphrase, original)
	}

	// The stored buffer must be a copy: mutating the caller's slice later
	// must not change what the app holds.
	original[0] = 'X'
	if app.luksPassphrase[0] == 'X' {
		t.Error("SetLuksPassphrase did not copy the input; stored buffer aliases caller memory")
	}

	// Setting a second passphrase replaces the first entirely.
	app.SetLuksPassphrase("short")
	if string(app.luksPassphrase) != "short" {
		t.Errorf("replacement passphrase = %q, want %q", app.luksPassphrase, "short")
	}
}

func TestSetLuks_DisablingClearsPassphrase(t *testing.T) {
	app := NewApp(false)

	app.SetLuks(true)
	app.SetLuksPassphrase("secret")
	if len(app.luksPassphrase) == 0 {
		t.Fatal("passphrase not stored while LUKS enabled")
	}

	// Re-enabling must not clear a held passphrase.
	app.SetLuks(true)
	if len(app.luksPassphrase) == 0 {
		t.Error("SetLuks(true) unexpectedly cleared the passphrase")
	}

	// Disabling clears it so the install proceeds unencrypted.
	app.SetLuks(false)
	if app.luks {
		t.Error("SetLuks(false) did not disable luks")
	}
	if app.luksPassphrase != nil {
		t.Errorf("SetLuks(false) left passphrase behind: %q", app.luksPassphrase)
	}
}

func TestClearLuksPassphrase_Idempotent(t *testing.T) {
	app := NewApp(false)

	// Clearing with nothing held is a no-op.
	app.clearLuksPassphrase()
	if app.luksPassphrase != nil {
		t.Error("clearLuksPassphrase on empty state left a non-nil slice")
	}

	app.SetLuksPassphrase("hunter2")
	app.clearLuksPassphrase()
	if app.luksPassphrase != nil {
		t.Errorf("clearLuksPassphrase left %q behind", app.luksPassphrase)
	}
}

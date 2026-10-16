package gui

import (
	"strings"
	"testing"
)

// TestStartInstallationRefusesLuksWithoutPassphrase guards the half-applied
// encryption state. The frontend sends the LUKS passphrase and the on/off flag
// as two separate Wails calls, and the runtime dispatches each on its own
// goroutine, so the backend can observe luks=true with no passphrase. Spawning
// the CLI in that state makes it exit before its first step (it reads the
// passphrase from stdin), which the user sees as a bare failure at 0%.
// StartInstallation must refuse and report the real cause instead.
func TestStartInstallationRefusesLuksWithoutPassphrase(t *testing.T) {
	a := NewApp(false) // not dry-run: the passphrase is required
	a.luks = true
	a.luksPassphrase = nil

	err := a.StartInstallation()
	if err == nil {
		t.Fatal("expected StartInstallation to refuse encryption without a passphrase")
	}
	if !strings.Contains(err.Error(), "passphrase") {
		t.Errorf("error should name the passphrase, got: %v", err)
	}
}

// TestSetLuksPassphraseKeepsHeldPassphrase documents the ordering the frontend
// relies on: sending the passphrase before flipping the flag must never discard
// it, so encryption is only ever enabled with a passphrase already held.
func TestSetLuksPassphraseKeepsHeldPassphrase(t *testing.T) {
	a := NewApp(false)

	a.SetLuksPassphrase("s3cret")
	a.SetLuks(true)

	if !a.luks {
		t.Fatal("expected luks to be enabled")
	}
	if string(a.luksPassphrase) != "s3cret" {
		t.Fatalf("expected the held passphrase to survive SetLuks(true), got %q", string(a.luksPassphrase))
	}

	// Disabling still clears it, so an aborted attempt leaves nothing behind.
	a.SetLuks(false)
	if len(a.luksPassphrase) != 0 {
		t.Fatalf("expected SetLuks(false) to clear the passphrase, got %q", string(a.luksPassphrase))
	}
}

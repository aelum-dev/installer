package installdesktop

import (
	"testing"

	"aelum-installer/backend/shared"
)

func TestDesktopPackages_NotEmpty(t *testing.T) {
	if len(DesktopPackages) == 0 {
		t.Error("expected non-empty DesktopPackages")
	}
}

func TestDesktopPackages_ContainsEssential(t *testing.T) {
	expected := []string{"hyprland", "greetd", "kitty", "firefox", "plymouth", "pipewire"}
	for _, pkg := range expected {
		found := false
		for _, p := range DesktopPackages {
			if p == pkg {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected DesktopPackages to contain %q", pkg)
		}
	}
}

func TestDesktopPackages_NoDuplicatesOrEmpty(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range DesktopPackages {
		if p == "" {
			t.Error("DesktopPackages contains an empty entry")
			continue
		}
		if seen[p] {
			t.Errorf("DesktopPackages contains duplicate %q", p)
		}
		seen[p] = true
	}
}

// TestRun_NotMounted checks the step fails cleanly when the installed system is
// not mounted (the normal state during unit tests).
func TestRun_NotMounted(t *testing.T) {
	if err := Run(&shared.InstallContext{}); err == nil {
		t.Error("expected an error when /mnt is not mounted")
	}
}

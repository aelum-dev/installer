package installdesktop

import (
	"testing"
)

func TestDesktopPackages_NotEmpty(t *testing.T) {
	if len(DesktopPackages) == 0 {
		t.Error("expected non-empty DesktopPackages")
	}
}

func TestDesktopPackages_ContainsEssential(t *testing.T) {
	expected := []string{"hyprland", "greetd", "alacritty", "firefox"}
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

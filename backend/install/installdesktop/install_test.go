package installdesktop

import (
	"testing"
)

func TestDesktopPackages_NotEmpty(t *testing.T) {
	if len(desktopPackages) == 0 {
		t.Error("expected non-empty desktopPackages")
	}
}

func TestDesktopPackages_ContainsEssential(t *testing.T) {
	expected := []string{"hyprland", "greetd", "alacritty", "firefox"}
	for _, pkg := range expected {
		found := false
		for _, p := range desktopPackages {
			if p == pkg {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected desktopPackages to contain %q", pkg)
		}
	}
}

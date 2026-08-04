package install

import (
	"testing"
)

// Note: Swap file support has been replaced with zram.
// These tests verify the zram-related helper functions.

func TestContainsModule_Found(t *testing.T) {
	config := "MODULES=(zram block filesystems)\n"
	if !containsModule(config, "zram") {
		t.Error("expected zram to be found in MODULES")
	}
}

func TestContainsModule_NotFound(t *testing.T) {
	config := "MODULES=(block filesystems)\n"
	if containsModule(config, "zram") {
		t.Error("expected zram to not be found in MODULES")
	}
}

func TestContainsModule_CommentedOut(t *testing.T) {
	config := "# MODULES=(zram block filesystems)\n"
	if containsModule(config, "zram") {
		t.Error("expected zram to not be found when commented out")
	}
}

func TestReplaceModules_AddsModule(t *testing.T) {
	config := "MODULES=(block filesystems)\n"
	result := replaceModules(config, "zram")
	if !containsModule(result, "zram") {
		t.Errorf("expected zram to be added, got:\n%s", result)
	}
	if !containsModule(result, "block") {
		t.Errorf("expected block to still be present, got:\n%s", result)
	}
}

func TestReplaceModules_Duplicate(t *testing.T) {
	config := "MODULES=(zram block filesystems)\n"
	result := replaceModules(config, "zram")
	// Should not add another zram
	count := 0
	for _, line := range splitLines(result) {
		if containsModuleInLine(line, "zram") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 zram entry, got %d:\n%s", count, result)
	}
}

func TestReplaceModules_NoModulesLine(t *testing.T) {
	config := "HOOKS=(base udev)\n"
	result := replaceModules(config, "zram")
	if !containsModule(result, "zram") {
		t.Errorf("expected zram to be added, got:\n%s", result)
	}
}

func TestAddZramToMkinitcpio_Logical(t *testing.T) {
	// Test the mkinitcpio manipulation logic without a real helper
	config := "MODULES=(block filesystems)\nHOOKS=(base udev archiso block filesystems)\n"

	// After adding zram to MODULES
	result := replaceModules(config, "zram")
	if !containsModule(result, "zram") {
		t.Errorf("expected zram in MODULES, got:\n%s", result)
	}
}

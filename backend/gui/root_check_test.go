package gui

import (
	"testing"

	"aestheticos-installer/backend/internal/core"
)

func TestRootCheckResult_Structure(t *testing.T) {
	// Verify RootCheckResult structure is valid
	result := core.RootCheckResult{
		Authorized: true,
		Message:    "Ready to install",
	}

	if result.Authorized && result.Error != "" {
		t.Error("Result cannot be both authorized and have an error")
	}

	// Test unauthorized result
	result2 := core.RootCheckResult{
		Authorized: false,
		Error:      "sudo_not_configured",
		Message:    "Please configure sudo to run without a password for the helper program.",
	}

	if result2.Authorized {
		t.Error("Result should not be authorized")
	}
	if result2.Error == "" {
		t.Error("Unauthorized result should have an error code")
	}
}

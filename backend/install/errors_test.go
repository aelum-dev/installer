package install

import (
	"errors"
	"testing"
)

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *ValidationError
		want string
	}{
		{"with field", &ValidationError{Field: "disk", Message: "is required"}, "disk: is required"},
		{"without field", &ValidationError{Message: "is required"}, "is required"},
		{"constructor sets both fields", NewValidationError("user", "invalid"), "user: invalid"},
	}
	for _, tt := range tests {
		if got := tt.err.Error(); got != tt.want {
			t.Errorf("%s: Error() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestInstallError_ErrorAndUnwrap(t *testing.T) {
	cause := errors.New("boom")

	withStep := NewInstallError("format_disk", cause)
	if got, want := withStep.Error(), `step "format_disk" failed: boom`; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	withoutStep := &InstallError{Cause: cause}
	if got := withoutStep.Error(); got != "boom" {
		t.Errorf("Error() = %q, want boom", got)
	}

	if !errors.Is(withStep, cause) {
		t.Error("errors.Is(withStep, cause) = false, want true (Unwrap contract)")
	}
	if got := errors.Unwrap(withStep); got != cause {
		t.Errorf("Unwrap() = %v, want the cause", got)
	}
}

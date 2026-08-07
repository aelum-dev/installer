package install

import "fmt"

// ============================================================================
// Error types
// ============================================================================

// ValidationError indicates a user-input or precondition failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

// InstallError indicates a runtime failure during installation.
type InstallError struct {
	Step  string
	Cause error
}

func (e *InstallError) Error() string {
	if e.Step != "" {
		return fmt.Sprintf("step %q failed: %v", e.Step, e.Cause)
	}
	return e.Cause.Error()
}

func (e *InstallError) Unwrap() error {
	return e.Cause
}

func NewInstallError(step string, cause error) *InstallError {
	return &InstallError{Step: step, Cause: cause}
}

// ============================================================================
// Root check result
// ============================================================================

// RootCheckResult contains the result of a root privilege check.
type RootCheckResult struct {
	Authorized bool   `json:"authorized"`
	Error      string `json:"error,omitempty"`
	Message    string `json:"message"`
	Output     string `json:"output,omitempty"`
}

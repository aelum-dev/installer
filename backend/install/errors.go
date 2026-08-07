package install


import "fmt"

// ValidationError indicates a user-input or precondition failure.
// These errors are not transient — the user must fix the input before retrying.
type ValidationError struct {
	Field   string // e.g. "username", "disk", "filesystem"
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

// NewValidationError creates a new ValidationError.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

// InstallError indicates a runtime failure during installation.
// These errors are fatal — the installation cannot continue.
type InstallError struct {
	Step string // the step that failed
	Cause error
}

func (e *InstallError) Error() string {
	if e.Step != "" {
		return fmt.Sprintf("step %q failed: %v", e.Step, e.Cause)
	}
	return e.Cause.Error()
}

// Unwrap returns the underlying cause for errors.As/Is compatibility.
func (e *InstallError) Unwrap() error {
	return e.Cause
}

// NewInstallError creates a new InstallError for a step failure.
func NewInstallError(step string, cause error) *InstallError {
	return &InstallError{Step: step, Cause: cause}
}

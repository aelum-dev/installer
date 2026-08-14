package install

import (
	"aestheticos-installer/backend/shared"
)

// Re-export from shared for backward compatibility.
type Logger = shared.Logger

const (
	CommandOutputPrefix = "  > "
	ErrorPrefix         = "ERROR: "
)

// NewConsoleLogger creates a new ConsoleLogger.
var NewConsoleLogger = shared.NewConsoleLogger

// GetLogger returns the shared logger instance.
func GetLogger() shared.Logger {
	return shared.GetLogger()
}

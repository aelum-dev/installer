package gui

import "aestheticos-installer/backend/internal/core"

// logger is the central log orchestrator shared by CLI and GUI.
var logger = core.NewConsoleLogger()

// GetLogger returns the central logger instance.
func GetLogger() core.Logger {
	return logger
}

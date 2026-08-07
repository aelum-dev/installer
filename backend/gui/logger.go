package gui

import "aestheticos-installer/backend/install"

// logger is the central log orchestrator shared by CLI and GUI.
var logger = install.NewConsoleLogger()

// GetLogger returns the central logger instance.
func GetLogger() install.Logger {
	return logger
}

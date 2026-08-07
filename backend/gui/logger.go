package gui

import "aestheticos-installer/backend/install"

// GetLogger returns the central log orchestrator shared by CLI and GUI.
func GetLogger() install.Logger {
	return install.GetLogger()
}

package shared

import (
	"os"
	"path/filepath"
)

// GetLogPath returns the path to the installation log file.
// Shared by core and steps packages to avoid duplication and import cycles.
func GetLogPath() string {
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		return filepath.Join(dir, LogFileName)
	}
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, LogFileName)
	}
	return LogFileName
}

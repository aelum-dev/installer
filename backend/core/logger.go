package core

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// GetLogPath returns the path to the installation log file.
func GetLogPath() string {
	// Same directory as the executable
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		return filepath.Join(dir, "installation.log")
	}
	// Fallback to user's home directory
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, "installation.log")
	}
	return "installation.log"
}

// WriteLog writes a message to the installation log file.
func WriteLog(format string, args ...interface{}) error {
	logPath := GetLogPath()

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf("[%s] %s\n", timestamp, fmt.Sprintf(format, args...))
	_, err = f.WriteString(msg)
	return err
}

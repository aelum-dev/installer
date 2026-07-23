package gui

import (
	"fmt"
	"os"
	"time"

	"aestheticos-installer/backend/internal/shared"
)

// WriteErrorLog writes installation error details to the log file and returns the log path.
func WriteErrorLog(stepLabel, errMsg string) string {
	logPath := shared.GetLogPath()

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, shared.LogFileMode)
	if err != nil {
		return logPath
	}
	defer f.Close()

	timestamp := time.Now().Format(shared.TimestampFormatFull)
	header := fmt.Sprintf("\n=== INSTALLATION ERROR at %s ===\n", timestamp)
	stepInfo := fmt.Sprintf("Step: %s\n", stepLabel)
	errorMsg := fmt.Sprintf("Error: %s\n", errMsg)
	logLocation := fmt.Sprintf("Log file: %s\n", logPath)

	f.WriteString(header)
	f.WriteString(stepInfo)
	f.WriteString(errorMsg)
	f.WriteString(logLocation)
	f.WriteString("=== END ERROR ===\n\n")

	return logPath
}



package core

import (
	"fmt"
	"os"
	"time"

	"aestheticos-installer/backend/internal/shared"
)

// WriteLog writes a message to the installation log file.
// Silently ignores write errors so callers don't need to handle them.
func WriteLog(format string, args ...interface{}) {
	logPath := shared.GetLogPath()

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, shared.LogFileMode)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format(shared.TimestampFormatFull)
	msg := fmt.Sprintf("[%s] %s\n", timestamp, fmt.Sprintf(format, args...))
	f.WriteString(msg)
}

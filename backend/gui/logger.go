package gui

import (
	"fmt"
	"os"
	"time"

	"aestheticos-installer/backend/install"
	"aestheticos-installer/backend/shared"
)

// logger is the central log orchestrator for the GUI.
// GUI logs go to stderr (terminal) + log file.
// CLI output goes to log file ONLY — never to stdout/stderr (would leak to WebView).
var logger = NewGUILogger()

// GUILogger writes GUI-originated logs to stderr + log file.
// CLI output is filtered out — only log file writes for CLI data.
type GUILogger struct {
	next install.Logger
}

func NewGUILogger() *GUILogger {
	return &GUILogger{next: install.GetLogger()}
}

func (l *GUILogger) Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	// Write to stderr for terminal visibility
	fmt.Fprintf(os.Stderr, "[%s] %s\n", time.Now().Format(shared.TimestampFormatShort), msg)
	// Write to log file
	l.next.LogToFile(msg)
}

func (l *GUILogger) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "[%s] ERROR: %s\n", time.Now().Format(shared.TimestampFormatShort), msg)
	l.next.LogToFile("ERROR: " + msg)
}

func (l *GUILogger) Command(line string) {
	// CLI command output — log file only, never stdout/stderr
	l.next.LogToFile(line)
}

func (l *GUILogger) CommandNoNewline(line string) {
	l.next.LogToFile(line)
}

func (l *GUILogger) LogToFile(line string) {
	l.next.LogToFile(line)
}

func (l *GUILogger) WriteErrorLog(stepLabel, errMsg string) {
	l.next.WriteErrorLog(stepLabel, errMsg)
}

func (l *GUILogger) SetProgressContext(label string, percent int) {
	l.next.SetProgressContext(label, percent)
}

func (l *GUILogger) Progress(label string, percent int) {
	l.next.LogToFile(fmt.Sprintf("[%d%%] %s", percent, label))
}

func (l *GUILogger) ProgressLine(label string, percent int) {
	l.next.LogToFile(fmt.Sprintf("[%d%%] %s", percent, label))
}

func (l *GUILogger) ProgressDone() {
	l.next.LogToFile("✓")
}

func (l *GUILogger) ProgressJSON(label string, percent int, phase string) {
	l.next.ProgressJSON(label, percent, phase)
}

func (l *GUILogger) Write(line string) {
	l.next.LogToFile(line)
}

// GetLogger returns the GUI logger.
// GUI-originated logs go to stderr (terminal) + log file.
// CLI output goes to log file only.
func GetLogger() install.Logger {
	return logger
}

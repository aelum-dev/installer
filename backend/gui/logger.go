package gui

import "aestheticos-installer/backend/install"

// logger is the central log orchestrator for the GUI.
// It wraps the shared logger but suppresses stdout output to prevent
// logs from leaking into the Wails WebView.
var logger = NewSilentLogger()

// SilentLogger wraps an install.Logger but suppresses stdout output.
// Only log file writes are preserved. Used by the GUI to prevent
// console output from leaking into the WebView.
type SilentLogger struct {
	next install.Logger
}

func NewSilentLogger() *SilentLogger {
	return &SilentLogger{next: install.GetLogger()}
}

func (l *SilentLogger) Info(format string, args ...interface{}) {
	// Suppress stdout — only write to log file.
	l.next.LogToFile(format)
}

func (l *SilentLogger) Error(format string, args ...interface{}) {
	// Suppress stdout — only write to log file.
	l.next.LogToFile("ERROR: " + format)
}

func (l *SilentLogger) Command(line string) {
	l.next.LogToFile(line)
}

func (l *SilentLogger) CommandNoNewline(line string) {
	l.next.LogToFile(line)
}

func (l *SilentLogger) LogToFile(line string) {
	l.next.LogToFile(line)
}

func (l *SilentLogger) WriteErrorLog(stepLabel, errMsg string) {
	l.next.WriteErrorLog(stepLabel, errMsg)
}

func (l *SilentLogger) SetProgressContext(label string, percent int) {
	l.next.SetProgressContext(label, percent)
}

func (l *SilentLogger) Progress(label string, percent int) {
	l.next.LogToFile(formatProgress(label, percent))
}

func (l *SilentLogger) ProgressLine(label string, percent int) {
	l.next.LogToFile(formatProgress(label, percent))
}

func (l *SilentLogger) ProgressDone() {
	l.next.LogToFile("✓")
}

func (l *SilentLogger) Write(line string) {
	l.next.LogToFile(line)
}

// formatProgress formats a progress line for log file output.
func formatProgress(label string, percent int) string {
	return fmt.Sprintf("[%d%%] %s", percent, label)
}

// GetLogger returns the central log orchestrator for the GUI.
// Unlike the CLI logger, this suppresses stdout to prevent
// log leakage into the Wails WebView.
func GetLogger() install.Logger {
	return logger
}

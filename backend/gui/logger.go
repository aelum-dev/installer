package gui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"aestheticos-installer/backend/install"
	"aestheticos-installer/backend/shared"
)

var logger = NewGUILogger()

// GUILogger wraps the shared console logger.
// GUI-originated messages go to stderr + log file (byte-identical).
// CLI output goes to log file only (terminal gets it from CLI stdout).
type GUILogger struct {
	mu   sync.Mutex
	next install.Logger
}

func NewGUILogger() *GUILogger {
	return &GUILogger{next: install.GetLogger()}
}

func (l *GUILogger) Info(format string, args ...interface{}) {
	l.mu.Lock()
	var msg strings.Builder
	fmt.Fprintf(&msg, format, args...)
	line := fmt.Sprintf("[%s] %s", time.Now().Format(shared.TimestampFormatFull), msg.String())
	fmt.Fprintf(os.Stderr, "%s\n", line)
	l.next.LogToFile(line)
	l.mu.Unlock()
}

func (l *GUILogger) Error(format string, args ...interface{}) {
	l.mu.Lock()
	var msg strings.Builder
	fmt.Fprintf(&msg, format, args...)
	line := fmt.Sprintf("[%s] ERROR: %s", time.Now().Format(shared.TimestampFormatFull), msg.String())
	fmt.Fprintf(os.Stderr, "%s\n", line)
	l.next.LogToFile(line)
	l.mu.Unlock()
}

func (l *GUILogger) Command(line string) {
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

func (l *GUILogger) UpdateContext(label string, percent int) {
	l.next.SetProgressContext(label, percent)
}

func (l *GUILogger) Progress(label string, percent int) {
	l.mu.Lock()
	line := fmt.Sprintf("[%s] [%d%%] %s", time.Now().Format(shared.TimestampFormatFull), percent, label)
	fmt.Fprintf(os.Stderr, "%s\n", line)
	l.next.LogToFile(line)
	l.mu.Unlock()
}

func (l *GUILogger) ProgressLine(label string, percent int) {
	l.mu.Lock()
	line := fmt.Sprintf("[%s] [%d%%] %s", time.Now().Format(shared.TimestampFormatFull), percent, label)
	fmt.Fprintf(os.Stderr, "%s\n", line)
	l.next.LogToFile(line)
	l.mu.Unlock()
}

func (l *GUILogger) ProgressDone() {
	l.mu.Lock()
	line := fmt.Sprintf("[%s] ✓", time.Now().Format(shared.TimestampFormatFull))
	fmt.Fprintf(os.Stderr, "%s\n", line)
	l.next.LogToFile(line)
	l.mu.Unlock()
}

func (l *GUILogger) ProgressJSON(label string, percent int, phase string) {
	l.next.ProgressJSON(label, percent, phase)
}

func (l *GUILogger) Write(line string) {
	l.mu.Lock()
	line = fmt.Sprintf("[%s] %s", time.Now().Format(shared.TimestampFormatFull), line)
	fmt.Fprintf(os.Stderr, "%s\n", line)
	l.next.LogToFile(line)
	l.mu.Unlock()
}

func GetLogger() install.Logger {
	return logger
}

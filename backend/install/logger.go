package install

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"aestheticos-installer/backend/shared"

	"golang.org/x/term"
)

// defaultLogger is the shared logger instance used by both CLI and GUI.
var defaultLogger Logger = NewConsoleLogger()

// GetLogger returns the shared logger instance.
func GetLogger() Logger {
	return defaultLogger
}

// Logger is the shared logging interface.
type Logger interface {
	Info(format string, args ...interface{})
	Error(format string, args ...interface{})
	Command(line string)
	CommandNoNewline(line string)
	LogToFile(line string)
	WriteErrorLog(stepLabel, errMsg string)
	SetProgressContext(label string, percent int)
	// UpdateContext updates the progress label and percentage without writing
	// to stdout. Used by the tick goroutine so Command() uses the latest %.
	UpdateContext(label string, percent int)
	Progress(label string, percent int)
	ProgressLine(label string, percent int)
	ProgressDone()
	ProgressJSON(label string, percent int, phase string)
	Write(line string)
	// Close flushes any buffered log entries and waits for the writer goroutine.
	Close()
}

const (
	CommandOutputPrefix  = "  > "
	ErrorPrefix          = "ERROR: "
)

// ConsoleLogger writes every log entry to both stdout and the log file
// using the same timestamp. Terminal output is byte-identical to the log.
// Progress state is accessed atomically so no mutex is needed.
type ConsoleLogger struct {
	logFile         *os.File
	progressLabel   atomic.Pointer[string]
	progressPercent atomic.Int32
}

func NewConsoleLogger() *ConsoleLogger {
	return &ConsoleLogger{}
}

func (l *ConsoleLogger) getLogFile() *os.File {
	if l.logFile != nil {
		return l.logFile
	}
	logPath := shared.GetLogPath()
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, shared.LogFileMode)
	if err != nil {
		return nil
	}
	l.logFile = f
	return f
}

// Close closes the log file.
func (l *ConsoleLogger) Close() {
	if l.logFile != nil {
		_ = l.logFile.Close()
		l.logFile = nil
	}
}

// writeLine formats msg with a timestamp and writes the SAME bytes to both
// stdout and the log file.
func (l *ConsoleLogger) writeLine(msg string) {
	ts := time.Now().Format(shared.TimestampFormatFull)
	line := fmt.Sprintf("[%s] %s", ts, msg)
	fmt.Printf("%s\n", line)
	if f := l.getLogFile(); f != nil {
		_, _ = fmt.Fprintf(f, "%s\n", line)
	}
}

// --- Public methods ---------------------------------------------------------

func (l *ConsoleLogger) Info(format string, args ...interface{}) {
	var msg strings.Builder
	fmt.Fprintf(&msg, format, args...)
	l.writeLine(msg.String())
}

func (l *ConsoleLogger) Error(format string, args ...interface{}) {
	var msg strings.Builder
	fmt.Fprintf(&msg, ErrorPrefix+format, args...)
	line := fmt.Sprintf("[%s] %s", time.Now().Format(shared.TimestampFormatFull), msg.String())
	fmt.Fprintf(os.Stderr, "%s\n", line)
	if f := l.getLogFile(); f != nil {
		_, _ = fmt.Fprintf(f, "%s\n", line)
	}
}

func (l *ConsoleLogger) Command(line string) {
	if line == "" {
		return
	}
	for _, line := range strings.Split(strings.TrimRight(line, "\n"), "\n") {
		label := ""
		if p := l.progressLabel.Load(); p != nil {
			label = *p
		}
		pct := int(l.progressPercent.Load())
		var msg strings.Builder
		if label != "" {
			fmt.Fprintf(&msg, "[%d%%] %s => %s", pct, label, line)
		} else {
			fmt.Fprintf(&msg, "%s%s", CommandOutputPrefix, line)
		}
		l.writeLine(msg.String())
	}
}

func (l *ConsoleLogger) CommandNoNewline(line string) {
	if line == "" {
		return
	}
	label := ""
	if p := l.progressLabel.Load(); p != nil {
		label = *p
	}
	pct := int(l.progressPercent.Load())
	var msg strings.Builder
	if label != "" {
		fmt.Fprintf(&msg, "[%d%%] %s => %s", pct, label, line)
	} else {
		fmt.Fprintf(&msg, "%s%s", CommandOutputPrefix, line)
	}
	l.writeLine(msg.String())
}

func (l *ConsoleLogger) SetProgressContext(label string, percent int) {
	l.progressLabel.Store(&label)
	l.progressPercent.Store(int32(percent))
}

func (l *ConsoleLogger) UpdateContext(label string, percent int) {
	l.progressLabel.Store(&label)
	l.progressPercent.Store(int32(percent))
}

func (l *ConsoleLogger) Progress(label string, percent int) {
	l.progressLabel.Store(&label)
	l.progressPercent.Store(int32(percent))
	l.writeLine(fmt.Sprintf("[%d%%] %s", percent, label))
	l.ProgressJSON(label, percent, "running")
}

func (l *ConsoleLogger) ProgressLine(label string, percent int) {
	l.progressLabel.Store(&label)
	l.progressPercent.Store(int32(percent))
	l.writeLine(fmt.Sprintf("[%d%%] %s", percent, label))
	l.ProgressJSON(label, percent, "running")
}

func (l *ConsoleLogger) ProgressDone() {
	l.writeLine("✓")
	l.progressLabel.Store(nil)
	l.progressPercent.Store(0)
}

func (l *ConsoleLogger) ProgressJSON(label string, percent int, phase string) {
	if term.IsTerminal(int(os.Stderr.Fd())) {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, `{"label":"%s","percent":%d,"phase":"%s"}
`, escapeJSON(label), percent, phase)
}

func (l *ConsoleLogger) LogToFile(line string) {
	if line == "" {
		return
	}
	if f := l.getLogFile(); f != nil {
		_, _ = fmt.Fprintf(f, "%s\n", line)
	}
}

func (l *ConsoleLogger) Write(line string) {
	if line == "" {
		return
	}
	l.writeLine(line)
}

func (l *ConsoleLogger) WriteErrorLog(stepLabel, errMsg string) {
	if f := l.getLogFile(); f != nil {
		_, _ = fmt.Fprintf(f, "\n")
		_, _ = fmt.Fprintf(f, "=== INSTALLATION ERROR at %s ===\n", time.Now().Format(shared.TimestampFormatFull))
		_, _ = fmt.Fprintf(f, "Step: %s\n", stepLabel)
		_, _ = fmt.Fprintf(f, "Error: %s\n", errMsg)
		_, _ = fmt.Fprintf(f, "Log file: %s\n", shared.GetLogPath())
		_, _ = fmt.Fprintf(f, "=== END ERROR ===\n")
		_, _ = fmt.Fprintf(f, "\n")
	}
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

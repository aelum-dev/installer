package shared

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
	Progress(stepID, label string, percent int)
	ProgressLine(stepID, label string, percent int)
	ProgressDone()
	ProgressJSON(stepID, label string, percent int, phase string)
	Write(line string)
	// Close flushes any buffered log entries and waits for the writer goroutine.
	Close()
}

const (
	CommandOutputPrefix = "  > "
	ErrorPrefix         = "ERROR: "
)

// ConsoleLogger writes every log entry to both stdout and the log file
// using the same timestamp. Terminal output is byte-identical to the log.
// Progress state is accessed atomically so no mutex is needed.
type ConsoleLogger struct {
	logFileOnce     sync.Once
	logFile         *os.File
	mirrorFile      *os.File
	progressLabel   atomic.Pointer[string]
	progressPercent atomic.Int32
}

func NewConsoleLogger() *ConsoleLogger {
	return &ConsoleLogger{}
}

func (l *ConsoleLogger) getLogFile() *os.File {
	// Lazy-open the log file exactly once, safely under concurrent access
	// (the command scanner, progress tick goroutine, and main updater all
	// call writeLine -> getLogFile simultaneously). sync.Once removes the
	// data race on the logFile field that a bare nil check had.
	l.logFileOnce.Do(func() {
		l.logFile = openLogFile(GetLogPath())
		if mirror := GetLogMirrorPath(); mirror != "" {
			l.mirrorFile = openLogFile(mirror)
		}
	})
	return l.logFile
}

// openLogFile opens path for append/create, returning nil on failure. A failed
// open is never fatal: the installer keeps streaming to the terminal.
func openLogFile(path string) *os.File {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, LogFileMode)
	if err != nil {
		return nil
	}
	return f
}

// writeToLogFiles writes one line to every open log destination (primary +
// optional /shared mirror).
func (l *ConsoleLogger) writeToLogFiles(line string) {
	l.getLogFile()
	for _, f := range []*os.File{l.logFile, l.mirrorFile} {
		if f != nil {
			_, _ = fmt.Fprintf(f, "%s\n", line)
		}
	}
}

// Close closes the log files.
func (l *ConsoleLogger) Close() {
	for _, f := range []*os.File{l.logFile, l.mirrorFile} {
		if f != nil {
			_ = f.Close()
		}
	}
	l.logFile = nil
	l.mirrorFile = nil
}

// writeLine formats msg with a timestamp and writes the SAME bytes to both
// stdout and the log file.
func (l *ConsoleLogger) writeLine(msg string) {
	ts := time.Now().Format(TimestampFormatFull)
	line := fmt.Sprintf("[%s] %s", ts, msg)
	fmt.Printf("%s\n", line)
	l.writeToLogFiles(line)
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
	line := fmt.Sprintf("[%s] %s", time.Now().Format(TimestampFormatFull), msg.String())
	fmt.Fprintf(os.Stderr, "%s\n", line)
	l.writeToLogFiles(line)
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

func (l *ConsoleLogger) Progress(stepID, label string, percent int) {
	l.progressLabel.Store(&label)
	l.progressPercent.Store(int32(percent))
	l.writeLine(fmt.Sprintf("[%d%%] %s", percent, label))
	l.ProgressJSON(stepID, label, percent, "running")
}

func (l *ConsoleLogger) ProgressLine(stepID, label string, percent int) {
	l.progressLabel.Store(&label)
	l.progressPercent.Store(int32(percent))
	l.writeLine(fmt.Sprintf("[%d%%] %s", percent, label))
	l.ProgressJSON(stepID, label, percent, "running")
}

func (l *ConsoleLogger) ProgressDone() {
	l.writeLine("✓")
	l.progressLabel.Store(nil)
	l.progressPercent.Store(0)
}

// ProgressJSON emits one machine-readable progress event on stderr. stepID is
// the stable step identifier (e.g. "format_disk") the frontend localizes;
// label is the human-readable English text, kept for logs/debugging.
func (l *ConsoleLogger) ProgressJSON(stepID, label string, percent int, phase string) {
	if term.IsTerminal(int(os.Stderr.Fd())) {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, `{"stepId":"%s","label":"%s","percent":%d,"phase":"%s"}
`, escapeJSON(stepID), escapeJSON(label), percent, phase)
}

func (l *ConsoleLogger) LogToFile(line string) {
	if line == "" {
		return
	}
	l.writeToLogFiles(line)
}

func (l *ConsoleLogger) Write(line string) {
	if line == "" {
		return
	}
	l.writeLine(line)
}

func (l *ConsoleLogger) WriteErrorLog(stepLabel, errMsg string) {
	block := fmt.Sprintf("\n=== INSTALLATION ERROR at %s ===\nStep: %s\nError: %s\nLog file: %s\n=== END ERROR ===\n",
		time.Now().Format(TimestampFormatFull), stepLabel, errMsg, GetLogPath())
	l.writeToLogFiles(block)
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

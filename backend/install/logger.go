package install

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"aestheticos-installer/backend/shared"
	"golang.org/x/term"
)

// defaultLogger is the shared logger instance used by both CLI and GUI.
// It is initialized lazily on first use to avoid early file handle creation.
var defaultLogger Logger = NewConsoleLogger()

// mu protects defaultLogger from concurrent access during initialization.
var mu sync.RWMutex

// GetLogger returns the shared logger instance used by both CLI and GUI.
func GetLogger() Logger {
	mu.RLock()
	defer mu.RUnlock()
	return defaultLogger
}

// SetLogger sets the shared logger instance. Must be called before any logging.
func SetLogger(l Logger) {
	mu.Lock()
	defer mu.Unlock()
	defaultLogger = l
}

// Logger is the shared logging interface used by installation steps and the GUI.
type Logger interface {
	// Info logs an informational message to stdout and the log file.
	Info(format string, args ...interface{})
	// Error logs an error message to stderr and the log file.
	Error(format string, args ...interface{})
	// Command logs a line of command output to stdout and the log file.
	// If a progress context is active, outputs: [timestamp] [percent%] label => line
	Command(line string)
	// CommandNoNewline logs a line of command output without a trailing newline.
	CommandNoNewline(line string)
	// LogToFile writes a line to the log file only (no console output).
	LogToFile(line string)
	// WriteErrorLog writes a formatted error block to the log file.
	WriteErrorLog(stepLabel, errMsg string)
	// SetProgressContext sets the current step label and percentage for command output formatting.
	SetProgressContext(label string, percent int)
	// Progress sets context and prints to stdout without newline.
	// Command() appends => line on the same line. Does NOT write to log file.
	Progress(label string, percent int)
	// ProgressLine logs a complete progress line to stdout and log file.
	// Used by tick goroutine for steps without command output.
	ProgressLine(label string, percent int)
	// ProgressDone logs a completion marker (✓) on its own line.
	ProgressDone()
	// ProgressJSON emits a JSON progress event to stderr.
	// Used by the GUI to receive clean step-name events without parsing logs.
	ProgressJSON(label string, percent int, phase string)
	// Write writes a line to both stdout and the log file (no prefix).
	Write(line string)
}

// --- Logging constants -------------------------------------------------------

// LogChannelBufferSize is the buffer size for the async log channel.
const LogChannelBufferSize = 1000

// LogWriteRetries is the number of retries before falling back to synchronous write.
const LogWriteRetries = 3

// LogWriteBackoffMs is the backoff duration between retry attempts (ms).
const LogWriteBackoffMs = 1

// CommandOutputPrefix is the prefix for command output lines.
const CommandOutputPrefix = "  > "

// ErrorPrefix is the prefix for error log lines.
const ErrorPrefix = "ERROR: "

// ConsoleLogger writes logs to the terminal AND to installation.log.
// File writes are asynchronous to avoid blocking the installation process.
// This is the default logger used by both CLI and GUI.
type ConsoleLogger struct {
	mu              sync.Mutex
	logFile         *os.File
	logChan         chan string
	done            chan struct{}
	progressLabel   string
	progressPercent int
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

func (l *ConsoleLogger) startWriter() {
	if l.logChan != nil {
		return // already started
	}
	l.logChan = make(chan string, LogChannelBufferSize)
	l.done = make(chan struct{})

	go func() {
		f := l.getLogFile()
		if f == nil {
			close(l.done)
			return
		}
		for msg := range l.logChan {
			timestamp := time.Now().Format(shared.TimestampFormatFull)
			_, _ = fmt.Fprintf(f, "[%s] %s\n", timestamp, msg)
		}
		_ = f.Close()
		close(l.done)
	}()
}

// Close flushes any remaining log messages and closes the log file.
// Safe to call multiple times. Idempotent.
func (l *ConsoleLogger) Close() {
	l.mu.Lock()
	ch := l.logChan
	l.mu.Unlock()

	if ch != nil {
		close(ch)
		<-l.done // wait for writer goroutine to finish
	}
}

func (l *ConsoleLogger) writeToFile(msg string) {
	l.mu.Lock()
	if l.logChan == nil {
		l.startWriter()
	}
	ch := l.logChan
	l.mu.Unlock()

	// Use non-blocking send with a small retry loop to avoid dropping messages.
	// If the channel is consistently full, fall back to synchronous write.
	for retries := 0; retries < LogWriteRetries; retries++ {
		select {
		case ch <- msg:
			return
		default:
			// Channel full — small backoff before retry
			time.Sleep(time.Duration(LogWriteBackoffMs) * time.Millisecond)
		}
	}

	// Channel consistently full — write synchronously as last resort
	f := l.getLogFile()
	if f == nil {
		return
	}
	timestamp := time.Now().Format(shared.TimestampFormatFull)
	_, _ = fmt.Fprintf(f, "[%s] %s\n", timestamp, msg)
}

func (l *ConsoleLogger) Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(os.Stdout, "[%s] %s\n", time.Now().Format(shared.TimestampFormatShort), msg)
	l.writeToFile(msg)
}

// Progress sets the current step context and prints to stdout WITHOUT
// a trailing newline. It does NOT write to the log file — Command()
// will write the combined line when a command follows.
func (l *ConsoleLogger) Progress(label string, percent int) {
	_, _ = fmt.Fprintf(os.Stdout, "[%s] [%d%%] %s ", time.Now().Format(shared.TimestampFormatShort), percent, label)
	l.SetProgressContext(label, percent)
	l.ProgressJSON(label, percent, "running")
}

// ProgressLine logs a complete progress line to both stdout and the log file.
// Used by the tick goroutine for steps that don't produce command output.
func (l *ConsoleLogger) ProgressLine(label string, percent int) {
	msg := fmt.Sprintf("[%d%%] %s", percent, label)
	_, _ = fmt.Fprintf(os.Stdout, "[%s] %s\n", time.Now().Format(shared.TimestampFormatShort), msg)
	l.writeToFile(msg)
	l.SetProgressContext(label, percent)
	l.ProgressJSON(label, percent, "running")
}

// ProgressJSON emits a JSON progress event to stderr when running under the GUI.
// When stderr is a terminal (CLI mode), JSON is suppressed so terminal output stays clean.
// The GUI reads JSON from stderr and forwards stdout to the terminal.
func (l *ConsoleLogger) ProgressJSON(label string, percent int, phase string) {
	if term.IsTerminal(int(os.Stderr.Fd())) {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, `{"label":"%s","percent":%d,"phase":"%s"}
`, escapeJSON(label), percent, phase)
}

// escapeJSON escapes special characters in a string for JSON output.
func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

// ProgressDone logs a completion marker (✓) on its own line.
func (l *ConsoleLogger) ProgressDone() {
	_, _ = fmt.Fprintf(os.Stdout, "[%s] ✓\n", time.Now().Format(shared.TimestampFormatShort))
	l.writeToFile("✓")
	l.SetProgressContext("", 0)
}

func (l *ConsoleLogger) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "[%s] %s%s\n", time.Now().Format(shared.TimestampFormatShort), ErrorPrefix, msg)
	l.writeToFile(ErrorPrefix + msg)
}

func (l *ConsoleLogger) Command(line string) {
	if line == "" {
		return
	}
	for _, line := range strings.Split(strings.TrimRight(line, "\n"), "\n") {
		l.mu.Lock()
		label := l.progressLabel
		pct := l.progressPercent
		l.mu.Unlock()

		if label != "" {
			// Append to progress line: ' => command'
			_, _ = fmt.Fprintf(os.Stdout, "=> %s\n", line)
			l.writeToFile(fmt.Sprintf("[%d%%] %s => %s", pct, label, line))
		} else {
			_, _ = fmt.Fprintf(os.Stdout, "%s%s\n", CommandOutputPrefix, line)
			l.writeToFile(CommandOutputPrefix + line)
		}
	}
}

// CommandNoNewline logs a line of command output without a trailing newline.
func (l *ConsoleLogger) CommandNoNewline(line string) {
	if line == "" {
		return
	}
	l.mu.Lock()
	label := l.progressLabel
	pct := l.progressPercent
	l.mu.Unlock()

	if label != "" {
		msg := fmt.Sprintf("[%d%%] %s => %s", pct, label, line)
		_, _ = fmt.Fprintf(os.Stdout, "[%s] %s", time.Now().Format(shared.TimestampFormatShort), msg)
		l.writeToFile(msg)
	} else {
		_, _ = fmt.Fprintf(os.Stdout, "%s%s", CommandOutputPrefix, line)
		l.writeToFile(CommandOutputPrefix + line)
	}
}

// SetProgressContext sets the current step label and percentage.
// Command output will be formatted as: [timestamp] [percent%] label => line
func (l *ConsoleLogger) SetProgressContext(label string, percent int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.progressLabel = label
	l.progressPercent = percent
}

// LogToFile writes a line to the log file only (no console output).
func (l *ConsoleLogger) LogToFile(line string) {
	if line == "" {
		return
	}
	l.writeToFile(line)
}

// Write writes a line to both stdout and the log file (no prefix).
func (l *ConsoleLogger) Write(line string) {
	if line == "" {
		return
	}
	_, _ = fmt.Fprintf(os.Stdout, "%s\n", line)
	l.writeToFile(line)
}

// WriteErrorLog writes a formatted error block to the log file.
func (l *ConsoleLogger) WriteErrorLog(stepLabel, errMsg string) {
	timestamp := time.Now().Format(shared.TimestampFormatFull)
	l.writeToFile("")
	l.writeToFile(fmt.Sprintf("=== INSTALLATION ERROR at %s ===", timestamp))
	l.writeToFile(fmt.Sprintf("Step: %s", stepLabel))
	l.writeToFile(fmt.Sprintf("Error: %s", errMsg))
	l.writeToFile(fmt.Sprintf("Log file: %s", shared.GetLogPath()))
	l.writeToFile("=== END ERROR ===")
	l.writeToFile("")
}

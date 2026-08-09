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
var defaultLogger Logger = NewConsoleLogger()

var mu sync.RWMutex

func GetLogger() Logger {
	mu.RLock()
	defer mu.RUnlock()
	return defaultLogger
}

func SetLogger(l Logger) {
	mu.Lock()
	defer mu.Unlock()
	defaultLogger = l
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
	LogChannelBufferSize = 1000
	LogWriteRetries      = 3
	LogWriteBackoffMs    = 1
	CommandOutputPrefix  = "  > "
	ErrorPrefix          = "ERROR: "
)

// ConsoleLogger writes every log entry atomically to both stdout and the log
// file using the same timestamp. Terminal output is byte-identical to the log.
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
		return
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
			_, _ = fmt.Fprintf(f, "%s\n", msg)
		}
		_ = f.Close()
		close(l.done)
	}()
}

// Close flushes any buffered log entries and waits for the writer goroutine.
func (l *ConsoleLogger) Close() {
	l.mu.Lock()
	ch := l.logChan
	if ch == nil {
		l.mu.Unlock()
		return
	}
	l.logChan = nil
	l.mu.Unlock()

	close(ch)
	<-l.done
}

// sendToLog queues msg for async log writing. Must be called with l.mu held.
func (l *ConsoleLogger) sendToLog(msg string) {
	if l.logChan == nil {
		l.startWriter()
	}
	ch := l.logChan

	for retries := 0; retries < LogWriteRetries; retries++ {
		select {
		case ch <- msg:
			return
		default:
			time.Sleep(time.Duration(LogWriteBackoffMs) * time.Millisecond)
		}
	}

	f := l.getLogFile()
	if f == nil {
		return
	}
	_, _ = fmt.Fprintf(f, "%s\n", msg)
}

// writeLine formats msg with a timestamp and writes the SAME bytes to both
// stdout and the log file. Called with l.mu held.
func (l *ConsoleLogger) writeLine(msg string) {
	ts := time.Now().Format(shared.TimestampFormatFull)
	line := fmt.Sprintf("[%s] %s", ts, msg)
	fmt.Printf("%s\n", line)
	l.sendToLog(line)
}

// --- Public methods ---------------------------------------------------------

func (l *ConsoleLogger) Info(format string, args ...interface{}) {
	l.mu.Lock()
	var msg strings.Builder
	fmt.Fprintf(&msg, format, args...)
	l.writeLine(msg.String())
	l.mu.Unlock()
}

func (l *ConsoleLogger) Error(format string, args ...interface{}) {
	l.mu.Lock()
	var msg strings.Builder
	fmt.Fprintf(&msg, ErrorPrefix+format, args...)
	// Errors go to stderr + log file (same bytes).
	line := fmt.Sprintf("[%s] %s", time.Now().Format(shared.TimestampFormatFull), msg.String())
	fmt.Fprintf(os.Stderr, "%s\n", line)
	l.sendToLog(line)
	l.mu.Unlock()
}

func (l *ConsoleLogger) Command(line string) {
	if line == "" {
		return
	}
	for _, line := range strings.Split(strings.TrimRight(line, "\n"), "\n") {
		l.mu.Lock()
		label := l.progressLabel
		pct := l.progressPercent
		var msg strings.Builder
		if label != "" {
			fmt.Fprintf(&msg, "[%d%%] %s => %s", pct, label, line)
		} else {
			fmt.Fprintf(&msg, "%s%s", CommandOutputPrefix, line)
		}
		l.writeLine(msg.String())
		l.mu.Unlock()
	}
}

func (l *ConsoleLogger) CommandNoNewline(line string) {
	if line == "" {
		return
	}
	l.mu.Lock()
	label := l.progressLabel
	pct := l.progressPercent
	var msg strings.Builder
	if label != "" {
		fmt.Fprintf(&msg, "[%d%%] %s => %s", pct, label, line)
	} else {
		fmt.Fprintf(&msg, "%s%s", CommandOutputPrefix, line)
	}
	l.writeLine(msg.String())
	l.mu.Unlock()
}

func (l *ConsoleLogger) SetProgressContext(label string, percent int) {
	l.mu.Lock()
	l.progressLabel = label
	l.progressPercent = percent
	l.mu.Unlock()
}

func (l *ConsoleLogger) UpdateContext(label string, percent int) {
	l.mu.Lock()
	l.progressLabel = label
	l.progressPercent = percent
	l.mu.Unlock()
}

func (l *ConsoleLogger) Progress(label string, percent int) {
	l.mu.Lock()
	l.progressLabel = label
	l.progressPercent = percent
	l.writeLine(fmt.Sprintf("[%d%%] %s", percent, label))
	l.mu.Unlock()
	l.ProgressJSON(label, percent, "running")
}

func (l *ConsoleLogger) ProgressLine(label string, percent int) {
	l.mu.Lock()
	l.progressLabel = label
	l.progressPercent = percent
	l.writeLine(fmt.Sprintf("[%d%%] %s", percent, label))
	l.mu.Unlock()
	l.ProgressJSON(label, percent, "running")
}

func (l *ConsoleLogger) ProgressDone() {
	l.mu.Lock()
	l.writeLine("✓")
	l.progressLabel = ""
	l.progressPercent = 0
	l.mu.Unlock()
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
	l.mu.Lock()
	l.sendToLog(line)
	l.mu.Unlock()
}

func (l *ConsoleLogger) Write(line string) {
	if line == "" {
		return
	}
	l.mu.Lock()
	l.writeLine(line)
	l.mu.Unlock()
}

func (l *ConsoleLogger) WriteErrorLog(stepLabel, errMsg string) {
	l.mu.Lock()
	l.sendToLog("")
	l.sendToLog(fmt.Sprintf("=== INSTALLATION ERROR at %s ===", time.Now().Format(shared.TimestampFormatFull)))
	l.sendToLog(fmt.Sprintf("Step: %s", stepLabel))
	l.sendToLog(fmt.Sprintf("Error: %s", errMsg))
	l.sendToLog(fmt.Sprintf("Log file: %s", shared.GetLogPath()))
	l.sendToLog("=== END ERROR ===")
	l.sendToLog("")
	l.mu.Unlock()
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

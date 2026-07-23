package install

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"aestheticos-installer/backend/internal/shared"
)

// ConsoleLogger writes logs to the terminal AND to installation.log.
// File writes are asynchronous to avoid blocking the installation process.
// This is the default logger used by both CLI and GUI when launched from a terminal.
type ConsoleLogger struct {
	mu       sync.Mutex
	logFile  *os.File
	logChan  chan string
	done     chan struct{}
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
			f.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, msg))
		}
		f.Close()
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
	if f != nil {
		timestamp := time.Now().Format(shared.TimestampFormatFull)
		f.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, msg))
	}
}

func (l *ConsoleLogger) Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stdout, "[%s] %s\n", time.Now().Format(shared.TimestampFormatShort), msg)
	l.writeToFile(msg)
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
	// Prefix each line with CommandOutputPrefix to indicate it's command output
	for _, line := range strings.Split(strings.TrimRight(line, "\n"), "\n") {
		fmt.Fprintf(os.Stdout, "%s%s\n", CommandOutputPrefix, line)
		l.writeToFile(CommandOutputPrefix + line)
	}
}

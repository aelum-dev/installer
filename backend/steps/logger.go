package steps

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
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

// GetLogPath returns the path to the installation log file.
func GetLogPath() string {
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		return filepath.Join(dir, "installation.log")
	}
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, "installation.log")
	}
	return "installation.log"
}

func (l *ConsoleLogger) getLogFile() *os.File {
	if l.logFile != nil {
		return l.logFile
	}
	logPath := GetLogPath()
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
	l.logChan = make(chan string, 1000)
	l.done = make(chan struct{})

	go func() {
		f := l.getLogFile()
		if f == nil {
			close(l.done)
			return
		}
		for msg := range l.logChan {
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			f.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, msg))
		}
		f.Close()
		close(l.done)
	}()
}

func (l *ConsoleLogger) writeToFile(msg string) {
	l.mu.Lock()
	if l.logChan == nil {
		l.startWriter()
	}
	ch := l.logChan
	l.mu.Unlock()

	select {
	case ch <- msg:
	default:
		// Channel full, write synchronously as fallback
		f := l.getLogFile()
		if f != nil {
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			f.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, msg))
		}
	}
}

func (l *ConsoleLogger) Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stdout, "[%s] %s\n", time.Now().Format("15:04:05"), msg)
	l.writeToFile(msg)
}

func (l *ConsoleLogger) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "[%s] ERROR: %s\n", time.Now().Format("15:04:05"), msg)
	l.writeToFile("ERROR: " + msg)
}

func (l *ConsoleLogger) Command(line string) {
	if line == "" {
		return
	}
	// Prefix each line with "  > " to indicate it's command output
	for _, line := range strings.Split(strings.TrimRight(line, "\n"), "\n") {
		fmt.Fprintf(os.Stdout, "  > %s\n", line)
		l.writeToFile("  > " + line)
	}
}

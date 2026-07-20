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
// This is the default logger used by both CLI and GUI when launched from a terminal.
type ConsoleLogger struct {
	mu      sync.Mutex
	logFile *os.File
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

func (l *ConsoleLogger) writeToFile(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	f := l.getLogFile()
	if f == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	f.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, msg))
}

func (l *ConsoleLogger) Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stdout, "[%s] %s\n", time.Now().Format("15:04:05"), msg)
	l.writeToFile("%s", msg)
}

func (l *ConsoleLogger) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "[%s] ERROR: %s\n", time.Now().Format("15:04:05"), msg)
	l.writeToFile("ERROR: %s", msg)
}

func (l *ConsoleLogger) Command(line string) {
	if line == "" {
		return
	}
	// Prefix each line with "  > " to indicate it's command output
	for _, line := range strings.Split(strings.TrimRight(line, "\n"), "\n") {
		fmt.Fprintf(os.Stdout, "  > %s\n", line)
		l.writeToFile("  > %s", line)
	}
}

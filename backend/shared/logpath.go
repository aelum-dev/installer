package shared

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Log path resolution is cached so every caller (GUI error message, CLI
// logger, installer startup truncation) sees the same filename for one run.
var (
	logPathMu    sync.Mutex
	logPath      string
	logMirror    string
	logPathSet   bool
	logMirrorDir = LogMirrorDir
)

// GetLogPath returns the installation log path. The GUI computes it once and
// passes it to the CLI via --log-path, so both processes report the same file:
// /var/log/aelum-install-<timestamp>.log.
func GetLogPath() string {
	logPathMu.Lock()
	defer logPathMu.Unlock()
	if !logPathSet {
		resolveLogPathLocked()
	}
	return logPath
}

// GetLogMirrorPath returns the /shared copy of the log, or "" when /shared is
// not present (e.g. outside the QEMU test guest).
func GetLogMirrorPath() string {
	logPathMu.Lock()
	defer logPathMu.Unlock()
	if !logPathSet {
		resolveLogPathLocked()
	}
	return logMirror
}

// SetLogPath pins the log path for this process. An empty path re-resolves the
// default (used by tests and standalone CLI runs).
func SetLogPath(path string) {
	logPathMu.Lock()
	defer logPathMu.Unlock()
	if path == "" {
		logPathSet = false
		resolveLogPathLocked()
		return
	}
	logPath = path
	logMirror = mirrorPathFor(path)
	logPathSet = true
}

func resolveLogPathLocked() {
	if env := os.Getenv(LogPathEnv); env != "" {
		logPath = env
	} else {
		name := LogFileNamePrefix + time.Now().Format(LogFileTimeLayout) + ".log"
		logPath = filepath.Join(LogDir, name)
	}
	logMirror = mirrorPathFor(logPath)
	logPathSet = true
}

func mirrorPathFor(path string) string {
	info, err := os.Stat(logMirrorDir)
	if err != nil || !info.IsDir() {
		return ""
	}
	mirror := filepath.Join(logMirrorDir, filepath.Base(path))
	// A log that is already written in the mirror dir has nowhere second to live.
	// The logger writes to the file and its mirror as two handles, so handing back
	// the same path twice would append every line twice — which is what happens
	// when --log-path names a file inside the share. Decided by the paths, not by
	// who set them: the GUI passes its own, and a test or a hand run may pass any.
	if mirror == filepath.Clean(path) {
		return ""
	}
	return mirror
}

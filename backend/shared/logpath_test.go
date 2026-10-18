package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetLogPathDefault(t *testing.T) {
	t.Setenv(LogPathEnv, "")
	SetLogPath("")
	t.Cleanup(func() { SetLogPath("") })

	got := GetLogPath()
	if filepath.Dir(got) != LogDir {
		t.Errorf("GetLogPath() dir = %q, want %q", filepath.Dir(got), LogDir)
	}
	base := filepath.Base(got)
	if !strings.HasPrefix(base, LogFileNamePrefix) || !strings.HasSuffix(base, ".log") {
		t.Errorf("GetLogPath() = %q, want %s<timestamp>.log", got, LogFileNamePrefix)
	}
}

func TestGetLogPathEnvOverride(t *testing.T) {
	want := filepath.Join(t.TempDir(), "custom.log")
	t.Setenv(LogPathEnv, want)
	SetLogPath("")
	t.Cleanup(func() { SetLogPath("") })

	if got := GetLogPath(); got != want {
		t.Errorf("GetLogPath() = %q, want env override %q", got, want)
	}
}

func TestGetLogMirrorPath(t *testing.T) {
	dir := t.TempDir()
	old := logMirrorDir
	logMirrorDir = dir
	t.Cleanup(func() {
		logMirrorDir = old
		SetLogPath("")
	})

	SetLogPath(filepath.Join(t.TempDir(), "run.log"))
	want := filepath.Join(dir, "run.log")
	if got := GetLogMirrorPath(); got != want {
		t.Errorf("GetLogMirrorPath() = %q, want %q", got, want)
	}
}

func TestGetLogMirrorPathAbsent(t *testing.T) {
	old := logMirrorDir
	logMirrorDir = filepath.Join(t.TempDir(), "missing")
	t.Cleanup(func() {
		logMirrorDir = old
		SetLogPath("")
	})

	SetLogPath(filepath.Join(t.TempDir(), "run.log"))
	if got := GetLogMirrorPath(); got != "" {
		t.Errorf("GetLogMirrorPath() = %q, want empty when the mirror dir is absent", got)
	}
}

func TestSetLogPathRoundTrip(t *testing.T) {
	want := filepath.Join(t.TempDir(), "explicit.log")
	SetLogPath(want)
	t.Cleanup(func() { SetLogPath("") })

	if got := GetLogPath(); got != want {
		t.Errorf("GetLogPath() = %q, want %q", got, want)
	}

	// Ensure the process really can write the resolved path.
	if err := os.WriteFile(want, []byte("x"), LogFileMode); err != nil {
		t.Fatalf("write log path: %v", err)
	}
}

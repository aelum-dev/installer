package shared

import (
	"path/filepath"
	"testing"
)

func TestGetLogPath(t *testing.T) {
	got := GetLogPath()
	if got == "" {
		t.Fatal("GetLogPath returned an empty path")
	}
	if filepath.Base(got) != LogFileName {
		t.Errorf("GetLogPath() = %q, want base %q", got, LogFileName)
	}
}

package gui

import (
	"os"
	"path/filepath"
	"testing"
)

// writeShim drops an executable script named name into a fresh temp dir and
// prepends that dir to PATH, so os/exec resolves the shell-out to the shim.
// Returns the dir (for scripts that write next to themselves).
func writeShim(t *testing.T, name, script string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatalf("write shim %s: %v", name, err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

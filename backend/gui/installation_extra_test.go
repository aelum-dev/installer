package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	present := filepath.Join(dir, "present")
	if err := os.WriteFile(present, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if !fileExists(present) {
		t.Errorf("fileExists(%q) = false, want true", present)
	}
	if fileExists(filepath.Join(dir, "absent")) {
		t.Error("fileExists(missing) = true, want false")
	}
}

func TestFindCLIBinary_PrefersExeDir(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	path := filepath.Join(filepath.Dir(exe), "aelum-installer-cli")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write sibling CLI: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	if got := findCLIBinary(); got != path {
		t.Errorf("findCLIBinary() = %q, want the sibling next to the executable (%q)", got, path)
	}
}

func TestFindCLIBinary_FallsBackToPath(t *testing.T) {
	// No sibling next to the test binary and nothing at the fixed absolute
	// locations: the bare name is returned so exec resolves it via PATH.
	if p := filepath.Join("/usr/local/bin", "aelum-installer-cli"); fileExists(p) {
		t.Skipf("a real CLI exists at %s", p)
	}
	if p := filepath.Join("/usr/bin", "aelum-installer-cli"); fileExists(p) {
		t.Skipf("a real CLI exists at %s", p)
	}

	if got := findCLIBinary(); got != "aelum-installer-cli" {
		t.Errorf("findCLIBinary() = %q, want the bare fallback name", got)
	}
}

func TestStartInstallation_AlreadyRunning(t *testing.T) {
	a := NewApp(true)
	a.installationRunning.Store(true)

	err := a.StartInstallation()
	if err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("StartInstallation() error = %v, want already-in-progress", err)
	}
}

func TestStartInstallation_StartFails(t *testing.T) {
	// A non-executable file named like the CLI on PATH makes exec fail to start
	// it, exercising the failure path without spawning a real install. Dry-run
	// skips the sudo prefix, so no privileges are needed.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "aelum-installer-cli"), []byte("not executable"), 0o644); err != nil {
		t.Fatalf("write non-executable CLI: %v", err)
	}
	t.Setenv("PATH", dir)

	a := NewApp(true)
	err := a.StartInstallation()
	if err == nil || !strings.Contains(err.Error(), "failed to start CLI") {
		t.Fatalf("StartInstallation() error = %v, want start failure", err)
	}
}

func TestCancelInstallation_SignalsProcess(t *testing.T) {
	cmd := exec.Command("sleep", "5")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start sleep: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	a := NewApp(false)
	a.installCmd = cmd

	// Must not block or panic; the process should receive SIGINT.
	a.CancelInstallation()
}

func TestCancelInstallation_NoProcessIsSafe(t *testing.T) {
	a := NewApp(false)
	a.CancelInstallation() // installCmd is nil
}

func TestWaitForCleanup_ClosedChannel(t *testing.T) {
	a := NewApp(false)
	a.installDone = make(chan struct{})
	close(a.installDone)

	a.WaitForCleanup() // returns immediately
}

func TestWaitForCleanup_NoChannelIsSafe(t *testing.T) {
	a := NewApp(false)
	a.WaitForCleanup() // installDone is nil
}

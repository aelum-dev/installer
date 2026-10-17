package primitives

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExecRunner exercises the real os/exec-backed CommandRunner (the default
// production implementation) with harmless commands.
func TestExecRunner(t *testing.T) {
	defer SetCommandRunner(execRunner{})()

	if err := Run("true").Run(); err != nil {
		t.Errorf("Run(true) = %v", err)
	}
	if err := Run("false").Run(); err == nil {
		t.Error("Run(false) = nil, want non-zero exit error")
	}

	out, err := Run("echo", "hello").Output()
	if err != nil {
		t.Fatalf("Output() = %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "hello" {
		t.Errorf("Output() = %q, want hello", got)
	}
	if _, err := Run("false").Output(); err == nil {
		t.Error("Output(false) = nil, want error")
	}

	both, err := Run("sh", "-c", "echo out; echo err >&2").CombinedOutput()
	if err != nil {
		t.Fatalf("CombinedOutput() = %v", err)
	}
	body := string(both)
	if !strings.Contains(body, "out") || !strings.Contains(body, "err") {
		t.Errorf("CombinedOutput() = %q, want both streams", body)
	}

	// RunChroot prefixes arch-chroot; an unknown command must surface an error
	// rather than silently succeeding.
	if _, err := RunChroot("this-command-does-not-exist"); err == nil {
		t.Error("RunChroot(unknown) = nil, want error")
	}
}

// TestOsFS_Operations drives the real filesystem implementation directly.
func TestOsFS_Operations(t *testing.T) {
	fs := osFS{}
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "file.txt")

	if err := fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := fs.WriteFile(path, []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := fs.AppendFile(path, []byte("b"), 0o644); err != nil {
		t.Fatalf("AppendFile: %v", err)
	}

	data, err := fs.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "ab" {
		t.Errorf("content = %q, want ab", data)
	}

	info, err := fs.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() != 2 {
		t.Errorf("size = %d, want 2", info.Size())
	}

	if err := fs.Chmod(path, 0o600); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	info, _ = fs.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}

	if err := fs.AppendFile(filepath.Join(dir, "nested", "new.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("AppendFile on missing file should create it: %v", err)
	}
	if got, err := fs.ReadFile(filepath.Join(dir, "nested", "new.txt")); err != nil || string(got) != "x" {
		t.Errorf("created append target = %q, %v; want x", got, err)
	}

	if err := fs.RemoveAll(filepath.Dir(path)); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	if _, err := fs.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Stat after RemoveAll = %v, want not-exist", err)
	}
}

// TestPackageWrappers exercises the package-level file helpers against real
// temp paths (host paths, no MountRoot prefixing).
func TestPackageWrappers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")

	if err := WriteFileMode(path, "hello", 0o644); err != nil {
		t.Fatalf("WriteFileMode: %v", err)
	}
	data, err := ReadFile(path)
	if err != nil || string(data) != "hello" {
		t.Fatalf("ReadFile() = %q, %v", data, err)
	}
	if _, err := Stat(path); err != nil {
		t.Fatalf("Stat() = %v", err)
	}
	if err := Chmod(path, 0o600); err != nil {
		t.Fatalf("Chmod() = %v", err)
	}
	if err := RemoveAll(path); err != nil {
		t.Fatalf("RemoveAll() = %v", err)
	}
}

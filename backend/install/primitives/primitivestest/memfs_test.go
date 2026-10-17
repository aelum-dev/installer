package primitivestest

import (
	"errors"
	"os"
	"testing"
)

// TestMemFS_WriteReadAppend verifies writes create from scratch and appends
// accumulate, and that the recorded mode tracks the last write.
func TestMemFS_WriteReadAppend(t *testing.T) {
	fs := NewMemFS()

	if err := fs.WriteFile("/etc/foo", []byte("one"), 0o644); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}
	if err := fs.AppendFile("/etc/foo", []byte("two"), 0o600); err != nil {
		t.Fatalf("AppendFile() = %v", err)
	}
	if got, ok := fs.Read("/etc/foo"); !ok || got != "onetwo" {
		t.Errorf("Read() = %q ok=%v, want onetwo", got, ok)
	}
	if got := fs.Mode("/etc/foo"); got.Perm() != 0o600 {
		t.Errorf("Mode() = %v, want 0600", got.Perm())
	}
	if got := fs.WriteCount("/etc/foo"); got != 2 {
		t.Errorf("WriteCount() = %d, want 2", got)
	}
}

// TestMemFS_ReadUnknown verifies unknown paths read as os.ErrNotExist.
func TestMemFS_ReadUnknown(t *testing.T) {
	fs := NewMemFS()
	if _, err := fs.ReadFile("/missing"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("ReadFile() err = %v, want ErrNotExist", err)
	}
	if _, err := fs.Stat("/missing"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Stat() err = %v, want ErrNotExist", err)
	}
}

// TestMemFS_ExistsAndStat verifies directory and file existence, including a
// path that only exists because a child file does.
func TestMemFS_ExistsAndStat(t *testing.T) {
	fs := NewMemFS().WithFile("/var/cache/pkg/base.tar", "x")

	if !fs.Exists("/var/cache/pkg") {
		t.Error("parent of a known file should exist")
	}
	info, err := fs.Stat("/var/cache/pkg")
	if err != nil {
		t.Fatalf("Stat(parent) = %v", err)
	}
	if !info.IsDir() {
		t.Error("parent should stat as a directory")
	}
	fileInfo, err := fs.Stat("/var/cache/pkg/base.tar")
	if err != nil {
		t.Fatalf("Stat(file) = %v", err)
	}
	if got := fileInfo.Size(); got != 1 {
		t.Errorf("Size() = %d, want 1", got)
	}
	if got := fs.Paths(); len(got) != 1 || got[0] != "/var/cache/pkg/base.tar" {
		t.Errorf("Paths() = %v", got)
	}
}

// TestMemFS_RemoveAll verifies a tree is removed, including children.
func TestMemFS_RemoveAll(t *testing.T) {
	fs := NewMemFS().
		WithFile("/mnt/etc/fstab", "x").
		WithFile("/mnt/etc/sub/other", "y")

	if err := fs.RemoveAll("/mnt/etc"); err != nil {
		t.Fatalf("RemoveAll() = %v", err)
	}
	if fs.Exists("/mnt/etc/fstab") || fs.Exists("/mnt/etc/sub/other") {
		t.Error("RemoveAll left children behind")
	}
}

// TestMemFS_ErrorInjection verifies every With*Error hook makes the matching
// operation fail.
func TestMemFS_ErrorInjection(t *testing.T) {
	sentinel := errors.New("boom")

	if _, err := NewMemFS().WithReadError("/a", sentinel).ReadFile("/a"); !errors.Is(err, sentinel) {
		t.Errorf("read error not injected: %v", err)
	}
	if err := NewMemFS().WithWriteError("/a", sentinel).WriteFile("/a", nil, 0o644); !errors.Is(err, sentinel) {
		t.Errorf("write error not injected: %v", err)
	}
	if err := NewMemFS().WithAppendError("/a", sentinel).AppendFile("/a", nil, 0o644); !errors.Is(err, sentinel) {
		t.Errorf("append error not injected: %v", err)
	}
	if _, err := NewMemFS().WithStatError("/a", sentinel).Stat("/a"); !errors.Is(err, sentinel) {
		t.Errorf("stat error not injected: %v", err)
	}
	if err := NewMemFS().WithRemoveError("/a", sentinel).RemoveAll("/a"); !errors.Is(err, sentinel) {
		t.Errorf("remove error not injected: %v", err)
	}
	if err := NewMemFS().WithChmodError("/a", sentinel).Chmod("/a", 0o755); !errors.Is(err, sentinel) {
		t.Errorf("chmod error not injected: %v", err)
	}
	if err := NewMemFS().WithMkdirError("/a", sentinel).MkdirAll("/a", 0o755); !errors.Is(err, sentinel) {
		t.Errorf("mkdir error not injected: %v", err)
	}
}

// TestMemFS_SatisfiesSeam is a runtime assertion that MemFS can be installed as
// the primitives.FileSystem (the compile-time guard is in memfs.go).
func TestMemFS_SatisfiesSeam(t *testing.T) {
	fs := NewMemFS().WithFile("/x", "y")
	data, err := fs.ReadFile("/x")
	if err != nil || string(data) != "y" {
		t.Fatalf("ReadFile() = %q, %v", data, err)
	}
}

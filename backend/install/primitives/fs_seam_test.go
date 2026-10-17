package primitives_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// TestWriteFile_PrefixesChrootPaths verifies chroot-relative paths gain the
// MountRoot prefix while host paths (ESP) are used as-is.
func TestWriteFile_PrefixesChrootPaths(t *testing.T) {
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := primitives.WriteFile("/etc/hostname", "aelum\n"); err != nil {
		t.Fatalf("WriteFile(chroot path) = %v", err)
	}
	if err := primitives.WriteFile(shared.BootMountPoint+"/loader/loader.conf", "default aelum\n"); err != nil {
		t.Fatalf("WriteFile(host path) = %v", err)
	}

	if got, ok := fs.Read(shared.MountRoot + "/etc/hostname"); !ok || got != "aelum\n" {
		t.Errorf("chroot write = %q ok=%v, want /mnt/etc/hostname", got, ok)
	}
	if got, ok := fs.Read(shared.BootMountPoint + "/loader/loader.conf"); !ok || got != "default aelum\n" {
		t.Errorf("host write = %q ok=%v", got, ok)
	}
}

// TestAppendFile_PrefixesChrootPaths verifies appends land under MountRoot.
func TestAppendFile_PrefixesChrootPaths(t *testing.T) {
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := primitives.WriteFile("/etc/vconsole.conf", "KEYMAP=us\n"); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}
	if err := primitives.AppendFile("/etc/vconsole.conf", "FONT=ter-132n\n"); err != nil {
		t.Fatalf("AppendFile() = %v", err)
	}
	if got, _ := fs.Read(shared.MountRoot + "/etc/vconsole.conf"); got != "KEYMAP=us\nFONT=ter-132n\n" {
		t.Errorf("vconsole.conf = %q", got)
	}
}

// TestWriteFile_Errors verifies directory-creation and write failures are
// wrapped with the target path.
func TestWriteFile_Errors(t *testing.T) {
	sentinel := errors.New("boom")

	fs := primitivestest.NewMemFS().WithMkdirError(shared.MountRoot+"/etc", sentinel)
	restore := primitives.SetFileSystem(fs)
	if err := primitives.WriteFile("/etc/foo", "x"); err == nil || !strings.Contains(err.Error(), "failed to create directory") {
		t.Errorf("mkdir failure = %v, want create-directory error", err)
	}
	restore()

	fs2 := primitivestest.NewMemFS().WithWriteError(shared.MountRoot+"/etc/foo", sentinel)
	defer primitives.SetFileSystem(fs2)()
	if err := primitives.WriteFile("/etc/foo", "x"); !errors.Is(err, sentinel) {
		t.Errorf("write failure = %v, want sentinel", err)
	}
}

// TestAppendFile_Error verifies append failures are wrapped.
func TestAppendFile_Error(t *testing.T) {
	sentinel := errors.New("boom")
	fs := primitivestest.NewMemFS().WithAppendError(shared.MountRoot+"/etc/foo", sentinel)
	defer primitives.SetFileSystem(fs)()

	if err := primitives.AppendFile("/etc/foo", "x"); !errors.Is(err, sentinel) {
		t.Errorf("AppendFile() err = %v, want sentinel", err)
	}
}

// TestReadStatRemoveChmod verifies the raw accessors delegate to the seam.
func TestReadStatRemoveChmod(t *testing.T) {
	path := shared.MountRoot + "/etc/foo"
	fs := primitivestest.NewMemFS().WithFile(path, "content")
	defer primitives.SetFileSystem(fs)()

	data, err := primitives.ReadFile(path)
	if err != nil || string(data) != "content" {
		t.Fatalf("ReadFile() = %q, %v", data, err)
	}
	info, err := primitives.Stat(path)
	if err != nil || info.Size() != int64(len("content")) {
		t.Fatalf("Stat() = %v, %v", info, err)
	}
	if err := primitives.Chmod(path, 0o755); err != nil {
		t.Fatalf("Chmod() = %v", err)
	}
	if got := fs.Mode(path); got.Perm() != 0o755 {
		t.Errorf("mode = %v, want 0755", got.Perm())
	}
	if err := primitives.RemoveAll(path); err != nil {
		t.Fatalf("RemoveAll() = %v", err)
	}
	if fs.Exists(path) {
		t.Error("RemoveAll did not remove the file")
	}
}

// TestWriteFileMode verifies the raw, no-prefix writer records the given mode.
func TestWriteFileMode(t *testing.T) {
	path := shared.MountRoot + "/etc/sudoers.d/wheel"
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := primitives.WriteFileMode(path, "content", 0o440); err != nil {
		t.Fatalf("WriteFileMode() = %v", err)
	}
	if got, _ := fs.Read(path); got != "content" {
		t.Errorf("content = %q", got)
	}
	if got := fs.Mode(path); got.Perm() != 0o440 {
		t.Errorf("mode = %v, want 0440", got.Perm())
	}
}

// TestDefaultFileSystemIsOS verifies the default seam writes to the real
// filesystem, using a temp dir.
func TestDefaultFileSystemIsOS(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/real.txt"
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if data, err := primitives.ReadFile(path); err != nil || string(data) != "x" {
		t.Fatalf("ReadFile(real) = %q, %v", data, err)
	}
}

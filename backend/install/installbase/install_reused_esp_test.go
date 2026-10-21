package installbase

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"aelum-installer/backend/shared"
)

func writeReusedEspMemtest(t *testing.T, bootMount string) {
	t.Helper()
	dir := filepath.Join(bootMount, "memtest86+")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"memtest86ia32.efi", "memtest86x64.efi"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("old-"+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBackupReusedEspMemtest(t *testing.T) {
	bootMount := t.TempDir()
	writeReusedEspMemtest(t, bootMount)

	// A non-conflicting file in the same directory must be left alone.
	keep := filepath.Join(bootMount, "memtest86+", "keep.txt")
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	backedUp, err := backupReusedEspMemtest(bootMount)
	if err != nil {
		t.Fatalf("backupReusedEspMemtest() = %v, want nil", err)
	}
	if backedUp != 2 {
		t.Fatalf("backedUp = %d, want 2", backedUp)
	}

	for _, name := range []string{"memtest86ia32.efi", "memtest86x64.efi"} {
		original := filepath.Join(bootMount, "memtest86+", name)
		if _, err := os.Lstat(original); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s still exists after backup (err=%v)", original, err)
		}
		backup := original + ".aelum-backup"
		data, err := os.ReadFile(backup)
		if err != nil {
			t.Errorf("read backup %s: %v", backup, err)
			continue
		}
		if want := "old-" + name; string(data) != want {
			t.Errorf("backup %s = %q, want %q", backup, data, want)
		}
	}

	if data, err := os.ReadFile(keep); err != nil || string(data) != "keep" {
		t.Errorf("non-conflicting file changed: data=%q err=%v", data, err)
	}
}

func TestBackupReusedEspMemtest_NoFiles(t *testing.T) {
	backedUp, err := backupReusedEspMemtest(t.TempDir())
	if err != nil {
		t.Fatalf("backupReusedEspMemtest() = %v, want nil", err)
	}
	if backedUp != 0 {
		t.Fatalf("backedUp = %d, want 0", backedUp)
	}
}

func TestBackupReusedEspMemtest_Idempotent(t *testing.T) {
	bootMount := t.TempDir()
	writeReusedEspMemtest(t, bootMount)

	if _, err := backupReusedEspMemtest(bootMount); err != nil {
		t.Fatalf("first backup = %v, want nil", err)
	}
	backedUp, err := backupReusedEspMemtest(bootMount)
	if err != nil {
		t.Fatalf("second backup = %v, want nil", err)
	}
	if backedUp != 0 {
		t.Fatalf("second backedUp = %d, want 0", backedUp)
	}
}

func TestRun_BacksUpReusedEspMemtest(t *testing.T) {
	fakePacstrap(t, `exit 0`)
	mounted(t)

	bootMount := t.TempDir()
	writeReusedEspMemtest(t, bootMount)

	if err := Run(&shared.InstallContext{ESP: bootMount}); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	for _, name := range []string{"memtest86ia32.efi", "memtest86x64.efi"} {
		backup := filepath.Join(bootMount, "memtest86+", name+".aelum-backup")
		if _, err := os.Stat(backup); err != nil {
			t.Errorf("expected backup %s: %v", backup, err)
		}
	}
}

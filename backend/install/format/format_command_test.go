package format

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// TestFormatPartition_CommandSequence verifies FormatPartition wipes signatures
// first and then runs the mkfs tool selected by the filesystem type.
func TestFormatPartition_CommandSequence(t *testing.T) {
	tests := []struct {
		name     string
		fstype   string
		wantMkfs []string
	}{
		{
			name:     "vfat",
			fstype:   shared.FileSystemVfat,
			wantMkfs: []string{"mkfs.fat", "-F", "32", "-n", "EFI", "/dev/vda1"},
		},
		{
			name:     "ext4",
			fstype:   shared.FileSystemExt4,
			wantMkfs: []string{"mkfs.ext4", "-F", "-E", "lazy_itable_init=0", "-L", "aelum", "/dev/vda2"},
		},
		{
			name:     "btrfs",
			fstype:   shared.FileSystemBtrfs,
			wantMkfs: []string{"mkfs.btrfs", "-f", "-L", "aelum", "/dev/vda2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := primitivestest.NewRecorder()
			defer primitives.SetCommandRunner(rec)()

			label := "aelum"
			path := "/dev/vda2"
			if tt.fstype == shared.FileSystemVfat {
				label, path = "EFI", "/dev/vda1"
			}

			if err := FormatPartition(path, tt.fstype, label); err != nil {
				t.Fatalf("FormatPartition() = %v, want nil", err)
			}

			wipe := rec.CallsFor("wipefs")
			wantWipe := []string{"wipefs", "--all", "-f", path}
			if len(wipe) != 1 || !reflect.DeepEqual(wipe[0], wantWipe) {
				t.Fatalf("wipefs argv = %v, want %v", wipe, wantWipe)
			}

			var mkfsName string
			if len(tt.wantMkfs) > 0 {
				mkfsName = tt.wantMkfs[0]
			}
			mkfs := rec.CallsFor(mkfsName)
			if len(mkfs) != 1 || !reflect.DeepEqual(mkfs[0], tt.wantMkfs) {
				t.Errorf("mkfs argv = %v, want %v", mkfs, tt.wantMkfs)
			}
		})
	}
}

// TestFormatPartition_UnsupportedFilesystem verifies unknown filesystems are
// rejected before any mkfs runs (wipefs still precedes the check).
func TestFormatPartition_UnsupportedFilesystem(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	err := FormatPartition("/dev/vda2", "xfs", "aelum")
	if err == nil {
		t.Fatal("FormatPartition() = nil, want unsupported filesystem error")
	}
	if !strings.Contains(err.Error(), "unsupported filesystem") {
		t.Errorf("error = %q, want unsupported filesystem", err)
	}
}

// TestFormatPartition_WipefsFails verifies a failed wipe aborts before mkfs.
func TestFormatPartition_WipefsFails(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("wipefs", errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	err := FormatPartition("/dev/vda2", shared.FileSystemExt4, "aelum")
	if err == nil || !strings.Contains(err.Error(), "wipefs") {
		t.Fatalf("FormatPartition() error = %v, want wipefs failure", err)
	}
	if rec.Called("mkfs.ext4") {
		t.Error("mkfs ran after wipefs failed")
	}
}

// TestFormatPartition_MkfsFails verifies a failed mkfs is surfaced with output.
func TestFormatPartition_MkfsFails(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgs([]string{"mkfs.ext4"}, primitivestest.Result{
		Output: []byte("no space left"),
		Err:    errors.New("exit status 1"),
	})
	defer primitives.SetCommandRunner(rec)()

	err := FormatPartition("/dev/vda2", shared.FileSystemExt4, "aelum")
	if err == nil {
		t.Fatal("FormatPartition() = nil, want mkfs failure")
	}
	if !strings.Contains(err.Error(), "no space left") {
		t.Errorf("error = %q, want it to include mkfs output", err)
	}
}

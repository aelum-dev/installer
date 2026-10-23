package partition

import (
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

const guardMiB = int64(1024 * 1024)

func TestValidate_RejectsUntypedNewRow(t *testing.T) {
	layout := shared.ManualLayout{
		Disk:          "/dev/vda",
		DiskSizeMB:    20480,
		DiskSizeBytes: 20480 * guardMiB,
		Partitions: []shared.ManualPartition{
			{Index: 0, Path: "/dev/vda1", SizeMB: 512, SizeBytes: 512 * guardMiB, Type: shared.TypeESP, ExistingFstype: "vfat"},
			{Index: 1, Path: "", SizeMB: 8192, Type: shared.TypeSystem},
			{Index: 2, Path: "", SizeMB: 1024, Type: ""},
		},
	}
	err := Validate(layout)
	if err == nil || !strings.Contains(err.Error(), "must be assigned") {
		t.Fatalf("Validate() = %v, want untyped-new-row error", err)
	}
	if _, err := buildActions("/dev/vda", &layout, shared.FileSystemExt4, nil); err == nil {
		t.Fatal("buildActions() = nil, want untyped-new-row error")
	}
}

// TestValidate_ExistingSubMiBESPRejected confirms the ESP floor also applies to
// existing partitions: a Boot partition is always formatted, so a sub-1-MiB
// partition cannot serve as the ESP.
func TestValidate_ExistingSubMiBESPRejected(t *testing.T) {
	layout := shared.ManualLayout{
		Disk:          "/dev/vda",
		DiskSizeMB:    20480,
		DiskSizeBytes: 20480 * guardMiB,
		Partitions: []shared.ManualPartition{
			{Index: 0, Path: "/dev/vda1", SizeMB: 0, SizeBytes: 512 * 1024, Type: shared.TypeESP, ExistingFstype: "vfat"},
			{Index: 1, Path: "/dev/vda2", SizeMB: 8192, SizeBytes: 8192 * guardMiB, Type: shared.TypeSystem},
		},
	}
	err := Validate(layout)
	if err == nil || !strings.Contains(err.Error(), "boot partition is too small") {
		t.Fatalf("Validate() = %v, want boot-floor error", err)
	}
}

func TestValidate_ManualDiskMinimum(t *testing.T) {
	layout := shared.ManualLayout{Disk: "/dev/vda", DiskSizeMB: 8191, DiskSizeBytes: 8191 * guardMiB}
	err := Validate(layout)
	if err == nil || !strings.Contains(err.Error(), "too small for manual partitioning") {
		t.Fatalf("Validate() = %v, want manual-disk-minimum error", err)
	}
}

// TestApply_PreflightBeforeDelete verifies the exact placement check runs before
// any destructive sgdisk -d: a plan that passes totals-only Validate but cannot
// be placed after the delete must fail without deleting a partition.
func TestApply_PreflightBeforeDelete(t *testing.T) {
	// 512 MiB ESP + a kept 20 GiB partition leave a 7 GiB front gap and a
	// ~2.4 GiB rear gap; a 14398 MiB replacement root fits in total but not in a
	// single region.
	diskMB := int64(40000)
	espEnd := int64(2048 + 512*2048 - 1)
	keptStart := espEnd + 1 + 7000*2048
	keptEnd := keptStart + 20480*2048 - 1
	sgdiskOut := `Number  Start (sector)    End (sector)  Size       Code  Name
   1            2048         1050623   512.0 MiB  EF00  EFI
   2        15386624        57329663   20.0 GiB   8300  keep`

	rec := primitivestest.NewRecorder().OnArgsOutput([]string{"sgdisk", "-p", "/dev/vda"}, sgdiskOut)
	defer primitives.SetCommandRunner(rec)()

	layout := shared.ManualLayout{
		Disk:            "/dev/vda",
		DiskSizeMB:      diskMB,
		DiskSizeBytes:   diskMB * guardMiB,
		SectorSizeBytes: 512,
		Partitions: []shared.ManualPartition{
			{Index: 0, Path: "/dev/vda1", SizeMB: 512, SizeBytes: 512 * guardMiB, Type: shared.TypeESP, ExistingFstype: "vfat"},
			{Index: 1, Path: "/dev/vda2", SizeMB: 20480, SizeBytes: (keptEnd - keptStart + 1) * 512},
			{Index: 2, Path: "", SizeMB: 14398, Type: shared.TypeSystem},
		},
		DeletedPartitions: []shared.ManualPartition{{Index: 3, Path: "/dev/vda3"}},
	}
	ctx := &shared.InstallContext{DiskID: "/dev/vda", FileSystem: shared.FileSystemExt4}
	err := Apply(ctx, &layout)
	if err == nil {
		t.Fatal("Apply() = nil, want placement error")
	}
	for _, argv := range rec.CallsFor("sgdisk") {
		for _, a := range argv {
			if a == "-d" {
				t.Fatalf("Apply() deleted a partition before the placement preflight: %v", argv)
			}
		}
	}
}

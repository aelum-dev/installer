package partition

import (
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// Manual partitioning writes the plan with sgdisk, which refuses to save any
// change on a disk whose partition table is not a GPT ("Non-GPT disk; not saving
// changes. Use -g to override.", exit 3) but creates a GPT on a disk that has no
// table at all. The preflight has to tell those two apart and reject the first
// before anything on the disk is deleted or created.

// gptLayout is a valid manual plan on a 20 GiB disk: an existing 512 MiB Boot
// partition and an existing 8 GiB System partition, both formatted in place.
func gptLayout() shared.ManualLayout {
	return shared.ManualLayout{
		Disk:            "/dev/vda",
		DiskSizeMB:      20480,
		DiskSizeBytes:   20480 * guardMiB,
		SectorSizeBytes: 512,
		Partitions: []shared.ManualPartition{
			{Index: 0, Path: "/dev/vda1", SizeMB: 512, SizeBytes: 512 * guardMiB, Type: shared.TypeESP, ExistingFstype: "vfat"},
			{Index: 1, Path: "/dev/vda2", SizeMB: 8192, SizeBytes: 8192 * guardMiB, Type: shared.TypeSystem, ExistingFstype: "ext4"},
		},
	}
}

func TestComputeLayout_RejectsNonGPTDisk(t *testing.T) {
	fakeTools(t, map[string]string{
		"lsblk":    lsblkShimBody,
		"blockdev": "echo 512\n",
		"blkid":    "echo dos\n",
	})

	_, err := ComputeLayout("/dev/vda")
	if err == nil || !strings.Contains(err.Error(), "dos partition table") {
		t.Fatalf("ComputeLayout() error = %v, want a non-GPT rejection", err)
	}
}

// A disk with no partition table is the case sgdisk handles by writing one, so it
// must stay usable in manual mode.
func TestComputeLayout_AllowsUnpartitionedDisk(t *testing.T) {
	fakeTools(t, map[string]string{
		"lsblk":    lsblkEmptyShimBody,
		"blockdev": "echo 512\n",
		"blkid":    "exit 2\n",
	})

	if _, err := ComputeLayout("/dev/vda"); err != nil {
		t.Fatalf("ComputeLayout() = %v, want nil for an unpartitioned disk", err)
	}
}

func TestComputeLayout_AllowsGPTDisk(t *testing.T) {
	fakeTools(t, map[string]string{
		"lsblk":    lsblkShimBody,
		"blockdev": "echo 512\n",
		"blkid":    "echo gpt\n",
	})

	if _, err := ComputeLayout("/dev/vda"); err != nil {
		t.Fatalf("ComputeLayout() = %v, want nil for a GPT disk", err)
	}
}

func TestValidateFeasible_RejectsNonGPTDisk(t *testing.T) {
	fakeTools(t, map[string]string{"blkid": "echo dos\n"})
	rec := primitivestest.NewRecorder().OnArgsOutput([]string{"sgdisk", "-p", "/dev/vda"}, "")
	defer primitives.SetCommandRunner(rec)()

	layout := gptLayout()
	if err := ValidateFeasible(layout); err == nil || !strings.Contains(err.Error(), "dos partition table") {
		t.Fatalf("ValidateFeasible() error = %v, want a non-GPT rejection", err)
	}
}

// TestApply_RejectsNonGPTDiskBeforeWriting guards the ordering: the table check
// runs before the first destructive sgdisk, so a rejected plan leaves the disk
// exactly as it was.
func TestApply_RejectsNonGPTDiskBeforeWriting(t *testing.T) {
	fakeTools(t, map[string]string{"blkid": "echo dos\n", "blockdev": "echo 512\n"})
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	ctx := &shared.InstallContext{DiskID: "/dev/vda", FileSystem: shared.FileSystemExt4}
	layout := gptLayout()
	err := Apply(ctx, &layout)
	if err == nil || !strings.Contains(err.Error(), "dos partition table") {
		t.Fatalf("Apply() error = %v, want a non-GPT rejection", err)
	}
	for _, argv := range rec.CallsFor("sgdisk") {
		t.Fatalf("Apply() ran %v on a disk it should have rejected", argv)
	}
}

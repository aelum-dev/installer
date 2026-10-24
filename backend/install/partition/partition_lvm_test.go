package partition

import (
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

const bytesPerMiB = 1024 * 1024

// lvmLayout builds a two-row layout (Boot + System) on a 100 GiB disk.
func lvmLayout(systemMB int64, lvm, luks bool) shared.ManualLayout {
	return shared.ManualLayout{
		Disk:          "/dev/vda",
		DiskSizeMB:    100 * 1024,
		DiskSizeBytes: 100 * 1024 * bytesPerMiB,
		Lvm:           lvm,
		Luks:          luks,
		Partitions: []shared.ManualPartition{
			{Index: 0, Path: "/dev/vda1", SizeMB: 512, Type: shared.TypeESP, ExistingFstype: "vfat"},
			{Index: 1, SizeMB: systemMB, Type: shared.TypeSystem},
		},
	}
}

// TestValidate_LvmSystemFloor verifies the System partition must hold a whole
// volume group once LVM is enabled, and one with a LUKS header under it when
// encryption is also on. A plain root keeps the smaller floor.
func TestValidate_LvmSystemFloor(t *testing.T) {
	plainFloorMB := int64(shared.MinSystemPartitionBytes / bytesPerMiB)
	lvmFloorMB := int64(shared.MinLvmSystemPartitionBytes / bytesPerMiB)
	lvmLuksFloorMB := int64(shared.MinLvmLuksSystemPartitionBytes / bytesPerMiB)

	if lvmFloorMB <= plainFloorMB || lvmLuksFloorMB <= lvmFloorMB {
		t.Fatalf("floors must increase with LVM then LUKS: plain=%d lvm=%d lvm+luks=%d", plainFloorMB, lvmFloorMB, lvmLuksFloorMB)
	}

	tests := []struct {
		name    string
		sizeMB  int64
		lvm     bool
		luks    bool
		wantErr bool
	}{
		{"plain root at the plain floor", plainFloorMB, false, false, false},
		{"plain root just below, no LVM", plainFloorMB - 1, false, false, true},
		{"LVM at the plain floor", plainFloorMB, true, false, true},
		{"LVM just below its floor", lvmFloorMB - 1, true, false, true},
		{"LVM at its floor", lvmFloorMB, true, false, false},
		{"LVM+LUKS below its floor", lvmLuksFloorMB - 1, true, true, true},
		{"LVM+LUKS at its floor", lvmLuksFloorMB, true, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := lvmLayout(tt.sizeMB, tt.lvm, tt.luks)
			if err := Validate(layout); (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidate_LvmUseRemainingFloor applies the same floor to a brand-new
// "use all remaining space" row, whose final size is only known after layout.
func TestValidate_LvmUseRemainingFloor(t *testing.T) {
	// 14 GiB disk: ~13.5 GiB for the System partition. That clears the plain
	// floor but not the LVM one.
	layout := shared.ManualLayout{
		Disk:          "/dev/vda",
		DiskSizeMB:    14 * 1024,
		DiskSizeBytes: 14 * 1024 * bytesPerMiB,
		Lvm:           true,
		Partitions: []shared.ManualPartition{
			{Index: 0, SizeMB: 512, Type: shared.TypeESP},
			{Index: 1, SizeMB: 0, Type: shared.TypeSystem},
		},
	}

	err := Validate(layout)
	if err == nil {
		t.Fatal("Validate() = nil, want the LVM System floor to reject a 14 GiB disk")
	}
}

// TestBuildActions_LvmGUID verifies the System partition is typed as an LVM
// physical volume, except under LUKS where it holds the encrypted container.
func TestBuildActions_LvmGUID(t *testing.T) {
	tests := []struct {
		name string
		lvm  bool
		luks bool
		want string
	}{
		{"plain root", false, false, shared.GPTTypeGUIDs.LinuxRootX86_64},
		{"lvm", true, false, shared.GPTTypeGUIDs.LinuxLVM},
		{"luks", false, true, shared.GPTTypeGUIDs.LinuxRootX86_64},
		{"luks on lvm", true, true, shared.GPTTypeGUIDs.LinuxRootX86_64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := lvmLayout(20*1024, tt.lvm, tt.luks)
			layout.Partitions[0].Path = ""
			layout.Partitions[1].Path = ""

			actions, err := buildActions("/dev/vda", &layout, shared.FileSystemExt4, nil)
			if err != nil {
				t.Fatalf("buildActions: %v", err)
			}
			for _, a := range actions {
				if a.pType == shared.TypeSystem && a.guid != tt.want {
					t.Errorf("System GUID = %s, want %s", a.guid, tt.want)
				}
				if a.pType == shared.TypeESP && a.guid != shared.GPTTypeGUIDs.ESP {
					t.Errorf("Boot GUID = %s, want the ESP GUID", a.guid)
				}
			}
		})
	}
}

// TestBuildActions_LvmResolvedFloor verifies the resolved-size backstop also
// honors the LVM floor for a brand-new "use all remaining space" row.
func TestBuildActions_LvmResolvedFloor(t *testing.T) {
	layout := lvmLayout(0, true, false)
	layout.DiskSizeMB = 14 * 1024
	layout.DiskSizeBytes = 14 * 1024 * bytesPerMiB
	layout.Partitions[0].Path = ""
	layout.Partitions[1].Path = ""

	if _, err := buildActions("/dev/vda", &layout, shared.FileSystemExt4, nil); err == nil {
		t.Fatal("buildActions() = nil, want the resolved LVM floor to reject the plan")
	}
}

// TestReleaseSystemLvm verifies stale LVM is cleared only from the designated
// System partition, and only when LVM is enabled.
func TestReleaseSystemLvm(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	if err := releaseSystemLvm(&shared.InstallContext{}, "/dev/vda2"); err != nil {
		t.Fatalf("releaseSystemLvm without LVM = %v", err)
	}
	if len(rec.Calls()) != 0 {
		t.Fatalf("no LVM command should run when LVM is off, got %v", rec.Calls())
	}

	if err := releaseSystemLvm(&shared.InstallContext{Lvm: true}, "/dev/vda2"); err != nil {
		t.Fatalf("releaseSystemLvm = %v", err)
	}
	pvs := rec.CallsFor("pvs")
	if len(pvs) != 1 || pvs[0][len(pvs[0])-1] != "/dev/vda2" {
		t.Fatalf("expected one PV probe on the System partition, got %v", pvs)
	}
}

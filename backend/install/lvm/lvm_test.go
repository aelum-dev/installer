package lvm

import (
	"reflect"
	"testing"

	"aelum-installer/backend/shared"
)

// TestPath verifies logical volumes resolve to their device-mapper paths.
func TestPath(t *testing.T) {
	if got := Path("aelum", "root"); got != "/dev/mapper/aelum-root" {
		t.Errorf("Path = %q, want /dev/mapper/aelum-root", got)
	}
	if got := Path(shared.LvmVolumeGroup, shared.LvmHomeVolume); got != shared.LvmHomeLvPath {
		t.Errorf("Path = %q, want %q", got, shared.LvmHomeLvPath)
	}
}

// TestPvCreateArgs verifies pvcreate force-creates the volume without prompting.
func TestPvCreateArgs(t *testing.T) {
	got := pvCreateArgs("/dev/vda2")
	want := []string{"pvcreate", "-ff", "-y", "/dev/vda2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("pvCreateArgs() = %v, want %v", got, want)
	}
}

// TestVgCreateArgs verifies the volume group is named and the PV is the final
// argument.
func TestVgCreateArgs(t *testing.T) {
	got := vgCreateArgs(shared.LvmVolumeGroup, "/dev/vda2")
	want := []string{"vgcreate", shared.LvmVolumeGroup, "/dev/vda2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("vgCreateArgs() = %v, want %v", got, want)
	}
}

// TestLvCreateSizeArgs verifies the root LV is created at a fixed size in MiB.
func TestLvCreateSizeArgs(t *testing.T) {
	got := lvCreateSizeArgs(8192, shared.LvmRootVolume, shared.LvmVolumeGroup)
	want := []string{"lvcreate", "-L", "8192M", "-n", shared.LvmRootVolume, shared.LvmVolumeGroup}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lvCreateSizeArgs() = %v, want %v", got, want)
	}
}

// TestLvCreateRestArgs verifies the home LV consumes all remaining free space.
func TestLvCreateRestArgs(t *testing.T) {
	got := lvCreateRestArgs(shared.LvmHomeVolume, shared.LvmVolumeGroup)
	want := []string{"lvcreate", "-l", "100%FREE", "-n", shared.LvmHomeVolume, shared.LvmVolumeGroup}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lvCreateRestArgs() = %v, want %v", got, want)
	}
}

// TestVgChangeArgs verifies activation and deactivation use -ay / -an.
func TestVgChangeArgs(t *testing.T) {
	if got := vgChangeArgs(true, shared.LvmVolumeGroup); !reflect.DeepEqual(got, []string{"vgchange", "-ay", shared.LvmVolumeGroup}) {
		t.Errorf("vgChangeArgs(true) = %v", got)
	}
	if got := vgChangeArgs(false, shared.LvmVolumeGroup); !reflect.DeepEqual(got, []string{"vgchange", "-an", shared.LvmVolumeGroup}) {
		t.Errorf("vgChangeArgs(false) = %v", got)
	}
}

// TestRemoveArgs verifies vgremove/pvremove force-remove without prompting.
func TestRemoveArgs(t *testing.T) {
	if got := vgRemoveArgs(shared.LvmVolumeGroup); !reflect.DeepEqual(got, []string{"vgremove", "-ff", shared.LvmVolumeGroup}) {
		t.Errorf("vgRemoveArgs() = %v", got)
	}
	if got := pvRemoveArgs("/dev/vda2"); !reflect.DeepEqual(got, []string{"pvremove", "-ff", "-y", "/dev/vda2"}) {
		t.Errorf("pvRemoveArgs() = %v", got)
	}
}

// TestComputeRootMiB verifies the fixed 30% / 8 GiB-floor split, including the
// clamp that keeps the home LV at or above the 8 GiB floor.
func TestComputeRootMiB(t *testing.T) {
	const minMiB = 8 * 1024 // 8 GiB in MiB

	tests := []struct {
		name    string
		vgMiB   int64
		want    int64
		wantErr bool
	}{
		{"large disk uses 30%", 100 * 1024, 30 * 1024, false},
		{"small disk clamps root down to keep home floor", 16000, 16000 - minMiB, false},
		{"exactly 16 GiB splits evenly", 16 * 1024, minMiB, false},
		{"just above floor", 20000, minMiB, false},
		{"too small to hold both", 8192, 0, true},
		{"zero", 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := computeRootMiB(tt.vgMiB)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("computeRootMiB(%d) = %d, want error", tt.vgMiB, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("computeRootMiB(%d) unexpected error: %v", tt.vgMiB, err)
			}
			if got != tt.want {
				t.Errorf("computeRootMiB(%d) = %d, want %d", tt.vgMiB, got, tt.want)
			}
			// The home LV takes the rest; it must stay at or above the floor.
			if home := tt.vgMiB - got; home < minMiB {
				t.Errorf("home LV size %d MiB is below the %d MiB floor", home, minMiB)
			}
		})
	}
}

// TestComputeRootMiB_MatchesConstants guards the sizing rule against the shared
// constants it is derived from.
func TestComputeRootMiB_MatchesConstants(t *testing.T) {
	if shared.LvmRootPercent != 30 {
		t.Fatalf("LvmRootPercent changed to %d; update the split tests", shared.LvmRootPercent)
	}
	if shared.LvmRootMinBytes>>20 != 8192 {
		t.Fatalf("LvmRootMinBytes changed to %d; update the split tests", shared.LvmRootMinBytes)
	}
}

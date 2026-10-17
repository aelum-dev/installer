package primitives

import (
	"strings"
	"testing"

	"aelum-installer/backend/shared"
)

// TestBuildMountOpts pins the mount-option string for each subvolume shape:
// the subvol=<name> prefix, then compression when set, then nodatacow when set.
func TestBuildMountOpts(t *testing.T) {
	tests := []struct {
		name string
		sv   shared.BtrfsSubvolume
		want string
	}{
		{"compression only", shared.BtrfsSubvolume{Name: "@", Compression: "zstd"}, "subvol=@,compress=zstd"},
		{"nodatacow only", shared.BtrfsSubvolume{Name: "@cache", NoDataCow: true}, "subvol=@cache,nodatacow"},
		{"compression and nodatacow", shared.BtrfsSubvolume{Name: "@x", Compression: "zstd", NoDataCow: true}, "subvol=@x,compress=zstd,nodatacow"},
		{"plain subvolume", shared.BtrfsSubvolume{Name: "@plain"}, "subvol=@plain"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sv := tt.sv
			if got := BuildMountOpts(&sv); got != tt.want {
				t.Errorf("BuildMountOpts(%+v) = %q, want %q", sv, got, tt.want)
			}
		})
	}
}

// TestBuildMountOpts_CanonicalScheme ensures every canonical subvolume yields a
// usable option string: the subvol= prefix is present and no two subvolumes
// share the same string (which would confuse the fstab/mount steps).
func TestBuildMountOpts_CanonicalScheme(t *testing.T) {
	subvolumes := shared.GetBtrfsSubvolumes()
	if len(subvolumes) == 0 {
		t.Fatal("no subvolumes defined")
	}
	seen := make(map[string]string, len(subvolumes))
	for i := range subvolumes {
		sv := subvolumes[i]
		opts := BuildMountOpts(&sv)
		if !strings.HasPrefix(opts, shared.BtrfsSubvolumeMountPrefix+sv.Name) {
			t.Errorf("subvolume %q: opts %q missing %q prefix", sv.Name, opts, shared.BtrfsSubvolumeMountPrefix+sv.Name)
		}
		if prev, ok := seen[opts]; ok {
			t.Errorf("subvolumes %q and %q share mount opts %q", prev, sv.Name, opts)
		}
		seen[opts] = sv.Name
	}
}

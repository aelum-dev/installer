package shared

import "testing"

// TestGetBtrfsSubvolumes_Invariants pins the canonical subvolume scheme: unique
// names and mount points, a root subvolume at /, and the nodatacow/compression
// split the fstab generator relies on.
func TestGetBtrfsSubvolumes_Invariants(t *testing.T) {
	subs := GetBtrfsSubvolumes()
	if len(subs) != 6 {
		t.Fatalf("expected 6 subvolumes, got %d", len(subs))
	}

	names := map[string]bool{}
	mounts := map[string]bool{}
	for _, sv := range subs {
		if sv.Name == "" || sv.MountPoint == "" {
			t.Errorf("subvolume has empty name/mount point: %+v", sv)
		}
		if names[sv.Name] {
			t.Errorf("duplicate subvolume name %q", sv.Name)
		}
		if mounts[sv.MountPoint] {
			t.Errorf("duplicate mount point %q", sv.MountPoint)
		}
		names[sv.Name] = true
		mounts[sv.MountPoint] = true
	}

	if !names["@"] || !mounts["/"] {
		t.Error("scheme must contain the @ subvolume mounted at /")
	}
	if !mounts["/home"] {
		t.Error("scheme must mount /home")
	}

	// @cache and @tmp are nodatacow and uncompressed; everything else is zstd.
	for _, sv := range subs {
		if sv.Name == "@cache" || sv.Name == "@tmp" {
			if !sv.NoDataCow {
				t.Errorf("%s should be nodatacow", sv.Name)
			}
			if sv.Compression != "" {
				t.Errorf("%s should not be compressed, got %q", sv.Name, sv.Compression)
			}
			continue
		}
		if sv.Compression != DefaultBtrfsCompression {
			t.Errorf("%s compression = %q, want %q", sv.Name, sv.Compression, DefaultBtrfsCompression)
		}
	}
}

// TestGetLvmSubvolumes verifies the LVM split: the root LV carries every
// subvolume except @home, which the home LV carries.
func TestGetLvmSubvolumes(t *testing.T) {
	root := GetLvmBtrfsSubvolumes()
	home := GetLvmHomeSubvolumes()
	all := GetBtrfsSubvolumes()

	if len(home) != 1 || home[0].Name != "@home" || home[0].MountPoint != "/home" {
		t.Fatalf("home LV subvolumes = %+v, want a single @home at /home", home)
	}
	if len(root) != len(all)-1 {
		t.Errorf("root LV should carry all subvolumes but @home: got %d, want %d", len(root), len(all)-1)
	}
	for _, sv := range root {
		if sv.Name == "@home" {
			t.Error("@home must live on the home LV, not the root LV")
		}
	}
	if !subvolumeNamesContain(root, "@") {
		t.Error("root LV must carry @")
	}
}

func subvolumeNamesContain(subs []BtrfsSubvolume, name string) bool {
	for _, sv := range subs {
		if sv.Name == name {
			return true
		}
	}
	return false
}

func TestFindSubvolumeByMountPoint(t *testing.T) {
	subs := GetBtrfsSubvolumes()

	got := FindSubvolumeByMountPoint(subs, "/var/log")
	if got == nil || got.Name != "@log" {
		t.Fatalf("FindSubvolumeByMountPoint(/var/log) = %+v, want @log", got)
	}
	if FindSubvolumeByMountPoint(subs, "/nope") != nil {
		t.Error("expected nil for an unknown mount point")
	}
	if FindSubvolumeByMountPoint(nil, "/") != nil {
		t.Error("expected nil for an empty slice")
	}
}

func TestSubvolumePath(t *testing.T) {
	if got := SubvolumePath("/tmp/btrfs-setup", "@home"); got != "/tmp/btrfs-setup/@home" {
		t.Errorf("SubvolumePath = %q, want /tmp/btrfs-setup/@home", got)
	}
	if got := SubvolumePath("/mnt", "@"); got != "/mnt/@" {
		t.Errorf("SubvolumePath = %q, want /mnt/@", got)
	}
}

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

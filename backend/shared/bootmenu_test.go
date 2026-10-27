package shared

import "testing"

func TestInstallID(t *testing.T) {
	tests := []struct {
		name     string
		rootUUID string
		want     string
	}{
		{"uuid", "1A2B3C4D-5E6F-7890-ABCD-EF0123456789", "1a2b3c4d"},
		{"already short", "abc123", "abc123"},
		{"sanitizes separators", "AB-CD-EF-12", "abcdef12"},
		{"caps at eight", "0123456789abcdef", "01234567"},
		{"empty falls back", "", "install"},
		{"no usable characters falls back", "----", "install"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InstallID(tt.rootUUID); got != tt.want {
				t.Errorf("InstallID(%q) = %q, want %q", tt.rootUUID, got, tt.want)
			}
		})
	}
}

func TestInstallID_StableAndDistinct(t *testing.T) {
	// Reinstalling the same root UUID must reuse its fragment; a different
	// install must get a different one.
	a := InstallID("1111AAAA-0000-0000-0000-000000000000")
	b := InstallID("1111AAAA-0000-0000-0000-000000000000")
	c := InstallID("2222BBBB-0000-0000-0000-000000000000")
	if a != b {
		t.Errorf("InstallID is not stable: %q != %q", a, b)
	}
	if a == c {
		t.Errorf("different root UUIDs share install ID %q", a)
	}
}

func TestRefindEntryPathAndInclude(t *testing.T) {
	// The fragment is stored under EFI/refind/aelum, while the include token is
	// relative to rEFInd's own directory (EFI/refind).
	if got, want := RefindEntryPath("1a2b3c4d"), "EFI/refind/aelum/1a2b3c4d.conf"; got != want {
		t.Errorf("RefindEntryPath() = %q, want %q", got, want)
	}
	if got, want := RefindEntryInclude("1a2b3c4d"), "aelum/1a2b3c4d.conf"; got != want {
		t.Errorf("RefindEntryInclude() = %q, want %q", got, want)
	}
}

func TestInstallBootDirAndFilePath(t *testing.T) {
	if got, want := InstallBootDir("1a2b3c4d"), "aelum/1a2b3c4d"; got != want {
		t.Errorf("InstallBootDir() = %q, want %q", got, want)
	}
	if got, want := InstallBootFilePath("1a2b3c4d", "vmlinuz-linux-lts"), "aelum/1a2b3c4d/vmlinuz-linux-lts"; got != want {
		t.Errorf("InstallBootFilePath() = %q, want %q", got, want)
	}
}

func TestRefindEntryTitle(t *testing.T) {
	tests := []struct {
		name     string
		rootPart string
		want     string
	}{
		{"device path", "/dev/sda2", "Aelum (sda2)"},
		{"nvme path", "/dev/nvme0n1p3", "Aelum (nvme0n1p3)"},
		{"partition without path", "sda2", "Aelum (sda2)"},
		{"empty falls back", "", BootEntryTitle},
		{"root falls back", "/", BootEntryTitle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RefindEntryTitle(tt.rootPart); got != tt.want {
				t.Errorf("RefindEntryTitle(%q) = %q, want %q", tt.rootPart, got, tt.want)
			}
		})
	}
}

// TestRefindSnapshotEntryPaths pins the snapshot fragment's names. The installed
// system rewrites the file the installer created empty, so the spelling has to
// match on both sides: the include line, the ESP path, and the generator's own
// $install_id-snapshots.conf.
func TestRefindSnapshotEntryPaths(t *testing.T) {
	if got, want := RefindSnapshotEntryName("1a2b3c4d"), "1a2b3c4d-snapshots.conf"; got != want {
		t.Errorf("RefindSnapshotEntryName() = %q, want %q", got, want)
	}
	if got, want := RefindSnapshotEntryPath("1a2b3c4d"), "EFI/refind/aelum/1a2b3c4d-snapshots.conf"; got != want {
		t.Errorf("RefindSnapshotEntryPath() = %q, want %q", got, want)
	}
	if got, want := RefindSnapshotEntryInclude("1a2b3c4d"), "aelum/1a2b3c4d-snapshots.conf"; got != want {
		t.Errorf("RefindSnapshotEntryInclude() = %q, want %q", got, want)
	}
	// A snapshot fragment is a fragment of its own: it lives where the include
	// mechanism finds it, and it is never the install's own entry.
	if RefindSnapshotEntryPath("1a2b3c4d") == RefindEntryPath("1a2b3c4d") {
		t.Error("the snapshot fragment and the menu entry are the same file")
	}
}

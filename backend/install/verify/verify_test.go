package verify

import (
	"testing"

	"aelum-installer/backend/shared"
)

// TestRun_SystemNotMounted verifies that Run fails cleanly when the installed
// system is not mounted (the normal state during unit tests).
func TestRun_SystemNotMounted(t *testing.T) {
	ctx := &shared.InstallContext{
		DiskID:   "/dev/vda",
		Username: "testuser",
		Hostname: "testhost",
		Partitions: &shared.PartitionInfo{
			RootUUID: "root-uuid",
			EFIUUID:  "efi-uuid",
		},
	}

	if err := Run(ctx); err == nil {
		t.Error("expected error when /mnt is not mounted, got nil")
	}
}

// TestRequiredPackagesNonEmpty guards against accidentally clearing the list
// of packages that must be present in a bootable system.
func TestExpectedPackagesNonEmpty(t *testing.T) {
	if len(expectedPackages) == 0 {
		t.Error("expected a non-empty expectedPackages list")
	}
}

// TestCapitalizeFirst covers the log-line prettifier applied to diagnostics.
func TestCapitalizeFirst(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"kernel image: vmlinuz-lts", "Kernel image: vmlinuz-lts"},
		{"Already capitalised", "Already capitalised"},
		{"über", "Über"},
	}
	for _, tt := range tests {
		if got := capitalizeFirst(tt.in); got != tt.want {
			t.Errorf("capitalizeFirst(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestLuksEntryMapsMapper covers the LUKS boot-entry check that must accept any
// root= form: a LUKS root selected by UUID (root=UUID=…) never contains a
// literal /dev/mapper path, so requiring one produced a false failure on every
// encrypted install.
func TestLuksEntryMapsMapper(t *testing.T) {
	const uuid = "11111111-2222-3333-4444-555555555555"
	luksOnly := "rd.luks.name=" + uuid + "=aelum root=UUID=aaaa rw quiet splash"
	luksMapper := "rd.luks.name=" + uuid + "=aelum root=/dev/mapper/aelum rw"

	cases := []struct {
		name    string
		options string
		uuid    string
		want    bool
	}{
		{"luks only maps header", luksOnly, uuid, true},
		{"luks mapper root maps header", luksMapper, uuid, true},
		{"wrong mapper name", "rd.luks.name=" + uuid + "=other root=UUID=x", uuid, false},
		{"wrong header uuid", "rd.luks.name=deadbeef=aelum root=UUID=x", uuid, false},
		{"no mapping", "root=UUID=aaaa rw", uuid, false},
		{"unknown uuid accepts any mapping", luksOnly, "", true},
		{"unknown uuid still needs a mapping", "root=UUID=aaaa", "", false},
	}
	for _, tc := range cases {
		if got := luksEntryMapsMapper(tc.options, tc.uuid); got != tc.want {
			t.Errorf("%s: luksEntryMapsMapper(%q, %q) = %v, want %v",
				tc.name, tc.options, tc.uuid, got, tc.want)
		}
	}
}

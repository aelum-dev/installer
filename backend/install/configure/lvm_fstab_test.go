package configure

import (
	"strings"
	"testing"

	"aelum-installer/backend/shared"
)

func lvmContext() *shared.InstallContext {
	return &shared.InstallContext{
		FileSystem: shared.FileSystemBtrfs,
		Lvm:        true,
		Partitions: &shared.PartitionInfo{
			RootUUID: "root-lv-uuid",
			HomeUUID: "home-lv-uuid",
			EFIUUID:  "efi-uuid-456",
		},
	}
}

// TestGenerateLvmFstab_SplitsHomeOntoHomeLV verifies the LVM fstab places every
// subvolume except @home on the root LV and @home on the home LV.
func TestGenerateLvmFstab_SplitsHomeOntoHomeLV(t *testing.T) {
	content := generateLvmFstab(lvmContext())

	// Root LV subvolumes.
	for _, mp := range []string{"/", "/srv", "/var/cache", "/var/tmp", "/var/log"} {
		if !strings.Contains(content, "UUID=root-lv-uuid  "+mp+"  btrfs") {
			t.Errorf("expected root LV entry for %s, got:\n%s", mp, content)
		}
	}

	// Home subvolume is on the home LV, not the root LV.
	if !strings.Contains(content, "UUID=home-lv-uuid  /home  btrfs") {
		t.Errorf("expected /home on the home LV, got:\n%s", content)
	}
	if strings.Contains(content, "UUID=root-lv-uuid  /home  btrfs") {
		t.Errorf("@home must not live on the root LV, got:\n%s", content)
	}

	// EFI entry present.
	if !strings.Contains(content, "UUID=efi-uuid-456  /boot  vfat") {
		t.Errorf("expected EFI entry, got:\n%s", content)
	}

	// @ (root) carries zstd, @cache nodatacow.
	if !strings.Contains(content, "subvol=@,compress=zstd") {
		t.Errorf("expected compress=zstd on @, got:\n%s", content)
	}
	if !strings.Contains(content, "subvol=@cache,nodatacow") {
		t.Errorf("expected nodatacow on @cache, got:\n%s", content)
	}
	if !strings.Contains(content, "subvol=@home,compress=zstd") {
		t.Errorf("expected compression on @home, got:\n%s", content)
	}
}

// TestGenerateLvmFstab_NoDuplicateMountPoints verifies each mount point appears
// exactly once.
func TestGenerateLvmFstab_NoDuplicateMountPoints(t *testing.T) {
	content := generateLvmFstab(lvmContext())
	for _, mp := range []string{"/boot", "/", "/srv", "/var/cache", "/var/tmp", "/var/log", "/home"} {
		// Count data lines whose mount-point column matches.
		count := 0
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[1] == mp {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected exactly 1 fstab entry for %s, got %d:\n%s", mp, count, content)
		}
	}
}

// TestBootEntryOptions_Lvm verifies the boot entry points root at the root LV
// and keeps the btrfs rootflags.
func TestBootEntryOptions_Lvm(t *testing.T) {
	ctx := lvmContext()
	opts := bootEntryOptions(ctx)
	if !strings.Contains(opts, "root="+shared.LvmRootLvPath) {
		t.Errorf("expected root=%s, got: %s", shared.LvmRootLvPath, opts)
	}
	if strings.Contains(opts, "rd.luks.name=") {
		t.Errorf("unexpected rd.luks.name= without LUKS: %s", opts)
	}
	if !strings.Contains(opts, "rootflags=subvol=@") {
		t.Errorf("expected rootflags=subvol=@, got: %s", opts)
	}
}

// TestBootEntryOptions_LuksLvm verifies LUKS+LVM roots at the LV behind the
// decrypted mapper.
func TestBootEntryOptions_LuksLvm(t *testing.T) {
	ctx := lvmContext()
	ctx.Luks = true
	ctx.Partitions.LuksUUID = "luks-header-uuid"
	opts := bootEntryOptions(ctx)
	if !strings.Contains(opts, "rd.luks.name=luks-header-uuid="+shared.LuksMapperName) {
		t.Errorf("expected rd.luks.name mapping, got: %s", opts)
	}
	if !strings.Contains(opts, "root="+shared.LvmRootLvPath) {
		t.Errorf("expected root=%s, got: %s", shared.LvmRootLvPath, opts)
	}
}

// TestBootEntryOptions_PlainBtrfs verifies the non-LVM path is unchanged: root
// by UUID.
func TestBootEntryOptions_PlainBtrfs(t *testing.T) {
	ctx := &shared.InstallContext{
		FileSystem: shared.FileSystemBtrfs,
		Partitions: &shared.PartitionInfo{RootUUID: "root-uuid"},
	}
	opts := bootEntryOptions(ctx)
	if !strings.Contains(opts, "root=UUID=root-uuid") {
		t.Errorf("expected root=UUID=root-uuid, got: %s", opts)
	}
}

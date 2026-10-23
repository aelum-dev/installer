package mount

import (
	"strings"
	"testing"

	"aelum-installer/backend/shared"
)

// baseCtx is a fully-populated context that would pass every precondition, so
// each test can knock out exactly one field and assert the corresponding error.
func baseCtx() *shared.InstallContext {
	return &shared.InstallContext{
		DiskID:     "/dev/vda",
		FileSystem: shared.FileSystemExt4,
		Partitions: &shared.PartitionInfo{
			Disk:        "/dev/vda",
			EFIPart:     "/dev/vda1",
			RootPart:    "/dev/vda2",
			RootUUID:    "ROOT-UUID",
			MountPoints: map[string]string{},
		},
	}
}

func TestRun_RequiresDiskID(t *testing.T) {
	ctx := baseCtx()
	ctx.DiskID = ""
	if err := Run(ctx); err == nil {
		t.Fatal("expected an error for a missing disk ID")
	}
}

func TestRun_RequiresPartitions(t *testing.T) {
	ctx := baseCtx()
	ctx.Partitions = nil
	if err := Run(ctx); err == nil {
		t.Fatal("expected an error for nil partitions")
	}
}

func TestRun_RequiresFileSystem(t *testing.T) {
	ctx := baseCtx()
	ctx.FileSystem = ""
	if err := Run(ctx); err == nil {
		t.Fatal("expected an error for a missing filesystem")
	}
}

func TestRun_RequiresPartitionPaths(t *testing.T) {
	ctx := baseCtx()
	ctx.Partitions.EFIPart = ""
	if err := Run(ctx); err == nil {
		t.Fatal("expected an error for missing partition paths")
	}
}

func TestRun_LuksRequiresMapper(t *testing.T) {
	ctx := baseCtx()
	ctx.Luks = true
	if err := Run(ctx); err == nil {
		t.Fatal("expected an error when LUKS is enabled without a root mapper path")
	}
}

// TestRun_LuksRequiresPassphrase checks that mount refuses to proceed when LUKS
// is enabled but no passphrase was supplied. The exact error depends on the
// host (whether the mapper already exists), so only the failure is asserted.
func TestRun_LuksRequiresPassphrase(t *testing.T) {
	ctx := baseCtx()
	ctx.Luks = true
	ctx.Partitions.RootMapper = shared.LuksMapperPath
	// No LuksPassphrase set.
	if err := Run(ctx); err == nil {
		t.Fatal("expected an error when LUKS is enabled without a passphrase")
	}
}

func TestVerifyMount_NotMounted(t *testing.T) {
	err := verifyMount("/aelum-nonexistent-mountpoint", "/dev/vda2")
	if err == nil {
		t.Fatal("expected an error for a mount point that is not mounted")
	}
	if !strings.Contains(err.Error(), "not mounted") {
		t.Errorf("error = %v, want a 'not mounted' error", err)
	}
}

// TestBuildMountOptsAlias guards the exported alias used by callers that only
// import mount.
func TestBuildMountOptsAlias(t *testing.T) {
	sv := shared.BtrfsSubvolume{Name: "@", Compression: "zstd"}
	if got, want := BuildMountOpts(&sv), "subvol=@,compress=zstd"; got != want {
		t.Errorf("BuildMountOpts = %q, want %q", got, want)
	}
}

package configure

import (
	"os/exec"
	"strings"
	"testing"

	"aelum-installer/backend/shared"
)

// isMounted reports whether /mnt is currently a mount point.
func isMounted() bool {
	out, err := exec.Command("findmnt", "-n", "-o", "SOURCE", shared.MountRoot).Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

// TestRun_NotMounted verifies that Run short-circuits on the very first
// sub-step (GenerateFstab) when the target system is not mounted. The
// precondition check must fail before any privileged command is attempted.
//
// Skipped when /mnt happens to be a real mount point (e.g. a developer with a
// live install target), since the failure mode under test would not apply.
func TestRun_NotMounted(t *testing.T) {
	if isMounted() {
		t.Skip("/mnt is mounted; cannot exercise the not-mounted precondition")
	}

	ctx := &shared.InstallContext{
		DiskID:     "/dev/vda",
		FileSystem: shared.FileSystemExt4,
	}

	err := Run(ctx)
	if err == nil {
		t.Fatal("Run() = nil, want error when system is not mounted")
	}
	if !strings.Contains(err.Error(), "not mounted") {
		t.Errorf("Run() error = %q, want it to mention %q", err, "not mounted")
	}
}

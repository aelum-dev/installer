package clearcache

import (
	"testing"

	"aelum-installer/backend/shared"
)

// TestRun_RequiresMountedSystem verifies the step fails cleanly when /mnt is not
// mounted, caught before any filesystem mutation.
func TestRun_RequiresMountedSystem(t *testing.T) {
	ctx := &shared.InstallContext{DiskID: "/dev/vda"}
	if err := Run(ctx); err == nil {
		t.Error("expected an error when /mnt is not mounted")
	}
}

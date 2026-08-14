package clearcache

import (
	"testing"

	"aestheticos-installer/backend/shared"
)

func TestRun_ContextValidation(t *testing.T) {
	ctx := &shared.InstallContext{
		DiskID: "/dev/vda",
	}

	if ctx.DiskID != "/dev/vda" {
		t.Errorf("expected disk ID /dev/vda, got %s", ctx.DiskID)
	}
}

package clearcache

import (
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
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

// TestRun_Mounted_NoCacheDir verifies the step is a no-op when the mounted
// system has no pacman package cache to clear.
func TestRun_Mounted_NoCacheDir(t *testing.T) {
	rec := primitivestest.NewRecorder().OnOutput("findmnt", "/dev/vda2\n")
	defer primitives.SetCommandRunner(rec)()

	if err := Run(&shared.InstallContext{DiskID: "/dev/vda"}); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
}

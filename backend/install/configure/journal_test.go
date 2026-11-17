package configure

import (
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// ConfigureJournald (services.go) switches the installed system to a persistent
// journal — the log of the boot that broke the system has to be readable from
// the snapshot one rolls back to. These tests hold the drop-in the step writes:
// path and content, against the filesystem seam, no root and no /mnt.

const journaldDropInPath = shared.MountRoot + "/etc/systemd/journald.conf.d/aelum.conf"

func TestConfigureJournald_WritesPersistentStorageDropIn(t *testing.T) {
	defer primitives.SetCommandRunner(mountedRunner())()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := ConfigureJournald(&shared.InstallContext{}); err != nil {
		t.Fatalf("ConfigureJournald() error = %v", err)
	}

	got, ok := fs.Read(journaldDropInPath)
	if !ok {
		t.Fatalf("journald drop-in %s was not written (wrote: %v)", journaldDropInPath, fs.Paths())
	}
	for _, want := range []string{"[Journal]", "Storage=persistent"} {
		if !strings.Contains(got, want) {
			t.Errorf("drop-in does not contain %q:\n%s", want, got)
		}
	}
}

func TestConfigureJournald_RefusesUnMountedTarget(t *testing.T) {
	defer primitives.SetCommandRunner(primitivestest.NewRecorder())()
	defer primitives.SetFileSystem(primitivestest.NewMemFS())()

	if err := ConfigureJournald(&shared.InstallContext{}); err == nil {
		t.Error("ConfigureJournald() = nil, want the not-mounted precondition error")
	}
}

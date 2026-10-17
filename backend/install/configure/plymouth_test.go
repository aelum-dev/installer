package configure

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

const themeDstDir = shared.MountRoot + "/usr/share/plymouth/themes/aelum"

// --- installAelumTheme -------------------------------------------------------

func TestInstallAelumTheme_Success(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	if err := installAelumTheme(&shared.InstallContext{}); err != nil {
		t.Fatalf("installAelumTheme() = %v, want nil", err)
	}

	if got := rec.CallsFor("test"); len(got) != 1 || !reflect.DeepEqual(got[0], []string{"test", "-d", aelumThemeISOPath}) {
		t.Errorf("test calls = %v, want the theme dir probe", got)
	}
	if got := rec.CallsFor("mkdir"); len(got) != 1 || !reflect.DeepEqual(got[0], []string{"mkdir", "-p", themeDstDir}) {
		t.Errorf("mkdir calls = %v, want %s", got, themeDstDir)
	}
	wantCopy := []string{"cp", "-a", aelumThemeISOPath + "/.", themeDstDir + "/"}
	if got := rec.CallsFor("cp"); len(got) != 1 || !reflect.DeepEqual(got[0], wantCopy) {
		t.Errorf("cp calls = %v, want %v", got, wantCopy)
	}
}

func TestInstallAelumTheme_MissingOnISO(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsError([]string{"test", "-d", aelumThemeISOPath}, errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	err := installAelumTheme(&shared.InstallContext{})
	if err == nil || !strings.Contains(err.Error(), "not found on ISO") {
		t.Fatalf("installAelumTheme() error = %v, want missing-theme error", err)
	}
	if rec.Called("mkdir") || rec.Called("cp") {
		t.Error("theme install continued after the theme was found missing")
	}
}

func TestInstallAelumTheme_MkdirFails(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("mkdir", errors.New("permission denied"))
	defer primitives.SetCommandRunner(rec)()

	err := installAelumTheme(&shared.InstallContext{})
	if err == nil || !strings.Contains(err.Error(), "create plymouth theme dir") {
		t.Fatalf("installAelumTheme() error = %v, want mkdir failure", err)
	}
}

func TestInstallAelumTheme_CopyFails(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgs([]string{"cp", "-a"}, primitivestest.Result{
		Output: []byte("no space left"),
		Err:    errors.New("exit status 1"),
	})
	defer primitives.SetCommandRunner(rec)()

	err := installAelumTheme(&shared.InstallContext{})
	if err == nil || !strings.Contains(err.Error(), "copy Aelum plymouth theme") {
		t.Fatalf("installAelumTheme() error = %v, want copy failure", err)
	}
}

// --- createHushlogin ---------------------------------------------------------

func TestCreateHushlogin_Success(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	if err := createHushlogin("alice"); err != nil {
		t.Fatalf("createHushlogin() = %v, want nil", err)
	}

	wantTouch := []string{"touch", "/mnt/home/alice/.hushlogin"}
	if got := rec.CallsFor("touch"); len(got) != 1 || !reflect.DeepEqual(got[0], wantTouch) {
		t.Errorf("touch calls = %v, want %v", got, wantTouch)
	}
	wantChown := []string{"arch-chroot", shared.MountRoot, "chown", "alice:alice", "/home/alice/.hushlogin"}
	if got := rec.CallsFor("arch-chroot"); len(got) != 1 || !reflect.DeepEqual(got[0], wantChown) {
		t.Errorf("chown calls = %v, want %v", got, wantChown)
	}
}

func TestCreateHushlogin_TouchFails(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("touch", errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	err := createHushlogin("alice")
	if err == nil || !strings.Contains(err.Error(), ".hushlogin") {
		t.Fatalf("createHushlogin() error = %v, want touch failure", err)
	}
	if rec.Called("arch-chroot") {
		t.Error("chown ran after touch failed")
	}
}

func TestCreateHushlogin_ChownFails(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("arch-chroot", errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	err := createHushlogin("alice")
	if err == nil || !strings.Contains(err.Error(), "ownership") {
		t.Fatalf("createHushlogin() error = %v, want chown failure", err)
	}
}

// --- ConfigurePlymouth -------------------------------------------------------

func TestConfigurePlymouth_NotMounted(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("findmnt", errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	err := ConfigurePlymouth(&shared.InstallContext{})
	if err == nil || !strings.Contains(err.Error(), "not mounted") {
		t.Fatalf("ConfigurePlymouth() error = %v, want not mounted", err)
	}
}

// TestConfigurePlymouth_MissingTheme verifies the step aborts before touching
// the target when the theme is absent from the live ISO.
func TestConfigurePlymouth_MissingTheme(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnOutput("findmnt", "/dev/vda2\n").
		OnArgsError([]string{"test", "-d", aelumThemeISOPath}, errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	err := ConfigurePlymouth(&shared.InstallContext{})
	if err == nil || !strings.Contains(err.Error(), "not found on ISO") {
		t.Fatalf("ConfigurePlymouth() error = %v, want missing-theme error", err)
	}
}

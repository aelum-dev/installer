package users

import (
	"encoding/json"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// seededSession returns the file the live ISO's skeleton carries, in the shape
// Dux Shell writes it: the wallpaper and menu state a fresh Aelum home starts
// with, plus the session version. The weather keys are absent, exactly as in
// live-iso/airootfs/etc/skel/.local/state/dms/session.json.
const seededSession = `{
  "wallpaperPath": "/usr/share/aelum/wallpaper-dark.jpg",
  "hiddenApps": [
    "btop",
    "mpv"
  ],
  "configVersion": 7
}
`

// withSeededSkeleton returns a target whose skeleton carries the Dux Shell session
// state the live ISO ships.
func withSeededSkeleton() *primitivestest.MemFS {
	return mountedFS().WithFile(shared.MountRoot+shared.SkelDmsSessionPath, seededSession)
}

// sessionState reads the installed skeleton's session state back as JSON, so the
// assertions are about the file Dux Shell will parse and not about a substring.
func sessionState(t *testing.T, fs *primitivestest.MemFS) map[string]any {
	t.Helper()
	got, ok := fs.Read(shared.MountRoot + shared.SkelDmsSessionPath)
	if !ok {
		t.Fatalf("%s was not written", shared.SkelDmsSessionPath)
	}
	state := map[string]any{}
	if err := json.Unmarshal([]byte(got), &state); err != nil {
		t.Fatalf("%s is not valid JSON: %v\n%s", shared.SkelDmsSessionPath, err, got)
	}
	return state
}

func stringKey(t *testing.T, state map[string]any, key string) string {
	t.Helper()
	value, _ := state[key].(string)
	return value
}

// TestSeedWeatherLocation_SetTheCapitalOfTheChosenLocation is the feature: the
// location picked on the Welcome page decides what the desktop's weather widget
// reports, in the two keys Dux Shell reads.
func TestSeedWeatherLocation_SetsTheCapitalOfTheChosenLocation(t *testing.T) {
	for _, code := range []string{"ua", "de", "us"} {
		fs := withSeededSkeleton()
		defer primitives.SetFileSystem(fs)()
		defer primitives.SetCommandRunner(mountedRunner())()

		ctx := &shared.InstallContext{Region: shared.GetRegion(code)}
		seedWeatherLocation(ctx)

		state := sessionState(t, fs)
		want, _ := shared.WeatherForRegion(code)
		if got := stringKey(t, state, shared.DmsWeatherLocationKey); got != want.City {
			t.Errorf("%s: %s = %q, want %q", code, shared.DmsWeatherLocationKey, got, want.City)
		}
		if got := stringKey(t, state, shared.DmsWeatherCoordinatesKey); got != want.Coordinates {
			t.Errorf("%s: %s = %q, want %q", code, shared.DmsWeatherCoordinatesKey, got, want.Coordinates)
		}
	}
}

// TestSeedWeatherLocation_KeepsEverythingElseInThatFile: the file is the live
// ISO's, and the wallpaper, the hidden menu entries and the session version are
// set by other decisions. Rewriting it for the weather must not lose them.
func TestSeedWeatherLocation_KeepsEverythingElseInThatFile(t *testing.T) {
	fs := withSeededSkeleton()
	defer primitives.SetFileSystem(fs)()
	defer primitives.SetCommandRunner(mountedRunner())()

	seedWeatherLocation(&shared.InstallContext{Region: shared.GetRegion("ua")})

	state := sessionState(t, fs)
	if got := stringKey(t, state, "wallpaperPath"); got != "/usr/share/aelum/wallpaper-dark.jpg" {
		t.Errorf("wallpaperPath = %q, want the seeded value", got)
	}
	hidden, _ := state["hiddenApps"].([]any)
	if len(hidden) != 2 || hidden[0] != "btop" || hidden[1] != "mpv" {
		t.Errorf("hiddenApps = %v, want the seeded list", hidden)
	}
	version, _ := state["configVersion"].(float64)
	if version != 7 {
		t.Errorf("configVersion = %v, want 7 unchanged", state["configVersion"])
	}
	if mode := fs.Mode(shared.MountRoot + shared.SkelDmsSessionPath); mode.Perm() != 0o644 {
		t.Errorf("session state mode = %v, want 0644 like the shipped skeleton", mode.Perm())
	}
}

// TestSeedWeatherLocation_WorldwideWritesNothing: no country, no capital, and the
// desktop's own default is the right answer. The seeded file must stay untouched.
func TestSeedWeatherLocation_WorldwideWritesNothing(t *testing.T) {
	fs := withSeededSkeleton()
	defer primitives.SetFileSystem(fs)()
	defer primitives.SetCommandRunner(mountedRunner())()

	seedWeatherLocation(&shared.InstallContext{Region: shared.GetRegion(shared.DefaultRegionCode)})

	got, ok := fs.Read(shared.MountRoot + shared.SkelDmsSessionPath)
	if !ok {
		t.Fatal("the seeded session state was removed")
	}
	if strings.Contains(got, shared.DmsWeatherLocationKey) {
		t.Errorf("the worldwide location wrote a weather location: %s", got)
	}
}

// TestSeedWeatherLocation_SkeletonProblemsAreNotFatal: a weather widget is not a
// reason to fail an install. A skeleton the live ISO did not seed, or one that
// cannot be parsed, is the same case as an optional copied file — a warning.
func TestSeedWeatherLocation_SkeletonProblemsAreNotFatal(t *testing.T) {
	for name, content := range map[string]string{
		"no skeleton file at all": "",
		"not JSON":                "{not json",
	} {
		fs := mountedFS()
		if content != "" {
			fs = fs.WithFile(shared.MountRoot+shared.SkelDmsSessionPath, content)
		}
		defer primitives.SetFileSystem(fs)()
		defer primitives.SetCommandRunner(mountedRunner())()

		seedWeatherLocation(&shared.InstallContext{Region: shared.GetRegion("ua")})

		if got, ok := fs.Read(shared.MountRoot + shared.SkelDmsSessionPath); ok && strings.Contains(got, shared.DmsWeatherLocationKey) {
			t.Errorf("%s: a weather location was written anyway: %s", name, got)
		}
	}
}

// TestRun_WritesTheWeatherIntoTheSkeletonTheHomeIsSeededFrom: the file is written
// in Run, after isocopy has copied the skeleton and before useradd -m seeds a home
// out of it. That ordering is the feature — written after useradd, the install
// user's own home would not carry it, and only accounts created later would.
func TestRun_WritesTheWeatherIntoTheSkeletonTheHomeIsSeededFrom(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := withSeededSkeleton()
	defer primitives.SetFileSystem(fs)()

	ctx := &shared.InstallContext{Username: "alice", Region: shared.GetRegion("ua")}
	if err := Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	if !hasArgv(rec.CallsFor("arch-chroot"), "arch-chroot", shared.MountRoot, "useradd", "-m", "-G", "wheel", "--", "alice") {
		t.Fatalf("useradd never ran: %v", rec.CallsFor("arch-chroot"))
	}

	state := sessionState(t, fs)
	want, _ := shared.WeatherForRegion("ua")
	if got := stringKey(t, state, shared.DmsWeatherLocationKey); got != want.City {
		t.Errorf("%s = %q, want %q", shared.DmsWeatherLocationKey, got, want.City)
	}
}

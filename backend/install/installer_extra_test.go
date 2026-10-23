package install

import (
	"errors"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

var errTest = errors.New("boom")

func TestInstaller_Cancel_NilChannel(t *testing.T) {
	(&Installer{}).Cancel() // must not panic when no channel is set
}

func TestInstaller_Cancel_ClosesOnce(t *testing.T) {
	inst := &Installer{cancel: make(chan struct{})}

	inst.Cancel()
	select {
	case <-inst.cancel:
	default:
		t.Fatal("Cancel() did not close the cancel channel")
	}

	inst.Cancel() // sync.Once: must not close twice or panic
}

func TestInstaller_SetManualLayout(t *testing.T) {
	inst := &Installer{}
	layout := &shared.ManualLayout{Disk: "/dev/vda"}

	inst.SetManualLayout(layout)

	if !inst.manualMode {
		t.Error("SetManualLayout should enable manual mode")
	}
	if inst.manualLayout != layout {
		t.Error("SetManualLayout should store the layout pointer")
	}
}

func TestInstaller_InterruptedRoundTrip(t *testing.T) {
	inst := &Installer{}
	if inst.Interrupted() {
		t.Error("fresh installer should not be interrupted")
	}

	inst.SetInterrupted()
	if !inst.Interrupted() {
		t.Error("SetInterrupted should be observable via Interrupted")
	}
}

func TestInstaller_LogHelpers(t *testing.T) {
	// Non-nil logger: exercise the logging branches.
	inst := NewInstaller(InstallerConfig{DiskID: "/dev/vda"})
	inst.logInfo("info %d", 1)
	inst.logError("error %d", 2)

	// Nil logger: the helpers must be safe no-ops.
	bare := &Installer{}
	bare.logInfo("ignored")
	bare.logError("ignored")
}

func TestInstaller_RunCleanup(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	inst := NewInstaller(InstallerConfig{DiskID: "/dev/vda", Luks: true})
	inst.RunCleanup()
	inst.RunCleanup() // cleanupOnce: the second call is a no-op

	if !rec.Called("umount") {
		t.Error("RunCleanup should unmount the installed system")
	}
	if !rec.Called("cryptsetup") {
		t.Error("RunCleanup should close the LUKS mapper")
	}
}

// flakyCloseRunner fails the first `failures` cryptsetup calls. It exercises
// the LUKS close retry without real devices.
type flakyCloseRunner struct {
	failures int
	calls    int
	output   []byte
}

func (r *flakyCloseRunner) Run(*primitives.Command) error { return nil }
func (r *flakyCloseRunner) Output(*primitives.Command) ([]byte, error) {
	return nil, nil
}
func (r *flakyCloseRunner) CombinedOutput(*primitives.Command) ([]byte, error) {
	r.calls++
	if r.calls <= r.failures {
		out := r.output
		if len(out) == 0 {
			out = []byte("Device aelum is still in use.")
		}
		return out, errors.New("exit status 5")
	}
	return nil, nil
}

func withFastLUKSRetry(t *testing.T) {
	t.Helper()
	old := luksCloseRetryDelay
	luksCloseRetryDelay = 0
	t.Cleanup(func() { luksCloseRetryDelay = old })
}

func TestCloseLuksMapperWithRetry_TransientThenSuccess(t *testing.T) {
	withFastLUKSRetry(t)
	runner := &flakyCloseRunner{failures: 2}
	defer primitives.SetCommandRunner(runner)()

	if err := closeLuksMapperWithRetry(); err != nil {
		t.Fatalf("closeLuksMapperWithRetry() = %v, want nil after retries", err)
	}
	if runner.calls != 3 {
		t.Errorf("close attempts = %d, want 3 (2 transient failures + success)", runner.calls)
	}
}

func TestCloseLuksMapperWithRetry_NonTransientFailsFast(t *testing.T) {
	withFastLUKSRetry(t)
	runner := &flakyCloseRunner{failures: 10, output: []byte("Device aelum does not exist.")}
	defer primitives.SetCommandRunner(runner)()

	if err := closeLuksMapperWithRetry(); err == nil {
		t.Fatal("closeLuksMapperWithRetry() = nil, want the non-transient error")
	}
	if runner.calls != 1 {
		t.Errorf("close attempts = %d, want 1 (no retry for a non-transient error)", runner.calls)
	}
}

func TestCloseLuksMapperWithRetry_GivesUp(t *testing.T) {
	withFastLUKSRetry(t)
	runner := &flakyCloseRunner{failures: 100}
	defer primitives.SetCommandRunner(runner)()

	if err := closeLuksMapperWithRetry(); err == nil {
		t.Fatal("closeLuksMapperWithRetry() = nil, want an error after exhausting retries")
	}
	if runner.calls != luksCloseRetryAttempts {
		t.Errorf("close attempts = %d, want %d", runner.calls, luksCloseRetryAttempts)
	}
}

func TestDiscoverPartitions(t *testing.T) {
	rec := primitivestest.NewRecorder().OnOutput("lsblk",
		"vda      disk\n"+
			"vda1     part\n"+
			"/dev/sda2 part\n"+
			"garbage\n",
	)
	defer primitives.SetCommandRunner(rec)()

	got := discoverPartitions("/dev/vda")
	want := []string{"/dev/vda1", "/dev/sda2"}
	if len(got) != len(want) {
		t.Fatalf("discoverPartitions() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("discoverPartitions()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDiscoverPartitions_Error(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("lsblk", errTest)
	defer primitives.SetCommandRunner(rec)()

	if got := discoverPartitions("/dev/vda"); got != nil {
		t.Errorf("discoverPartitions() = %v, want nil on lsblk failure", got)
	}
}

func TestFormatDisk_AutomaticModeRequiresDisk(t *testing.T) {
	defer primitives.SetCommandRunner(primitivestest.NewRecorder())()

	err := formatDisk(&shared.InstallContext{})
	if err == nil {
		t.Error("formatDisk() without a disk ID should fail")
	}
}

func TestFormatDisk_ManualModeDelegates(t *testing.T) {
	defer primitives.SetCommandRunner(primitivestest.NewRecorder())()

	ctx := &shared.InstallContext{
		ManualMode:   true,
		ManualLayout: &shared.ManualLayout{},
	}
	if err := formatDisk(ctx); err == nil {
		t.Error("formatDisk() should surface partition.Apply's error for an empty layout")
	}
}

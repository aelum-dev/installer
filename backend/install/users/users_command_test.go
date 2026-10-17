package users

import (
	"errors"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
)

// TestExecuteCopyJobs_RunsEveryJob verifies every ISO copy job produces a mkdir
// for its parent directory and a cp of its source to /mnt.
func TestExecuteCopyJobs_RunsEveryJob(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	if err := ExecuteCopyJobs(); err != nil {
		t.Fatalf("ExecuteCopyJobs() = %v, want nil", err)
	}

	if got := len(rec.CallsFor("mkdir")); got != len(ISOCopyJobs) {
		t.Errorf("mkdir calls = %d, want %d", got, len(ISOCopyJobs))
	}
	if got := len(rec.CallsFor("cp")); got != len(ISOCopyJobs) {
		t.Errorf("cp calls = %d, want %d", got, len(ISOCopyJobs))
	}

	// Every copy destination must land inside the installed system at /mnt.
	for _, argv := range rec.CallsFor("cp") {
		dst := argv[len(argv)-1]
		if !strings.HasPrefix(dst, "/mnt") {
			t.Errorf("cp destination %q does not start with /mnt (argv %v)", dst, argv)
		}
	}
}

// TestExecuteCopyJobs_MkdirFailure verifies a per-job mkdir failure is counted
// and reported, and that the copy for that job is skipped.
func TestExecuteCopyJobs_MkdirFailure(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("mkdir", errors.New("permission denied"))
	defer primitives.SetCommandRunner(rec)()

	err := ExecuteCopyJobs()
	if err == nil {
		t.Fatal("ExecuteCopyJobs() = nil, want error")
	}
	if !strings.Contains(err.Error(), "copy job") {
		t.Errorf("error = %q, want it to mention copy jobs", err)
	}
	if rec.Called("cp") {
		t.Error("cp ran even though mkdir failed")
	}
}

// TestExecuteCopyJobs_CopyFailure verifies a failing cp is reported after the
// remaining directories are prepared.
func TestExecuteCopyJobs_CopyFailure(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("cp", errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	err := ExecuteCopyJobs()
	if err == nil {
		t.Fatal("ExecuteCopyJobs() = nil, want error")
	}
	if !strings.Contains(err.Error(), "copy job") {
		t.Errorf("error = %q, want it to mention copy jobs", err)
	}
	if got := len(rec.CallsFor("mkdir")); got != len(ISOCopyJobs) {
		t.Errorf("mkdir calls = %d, want %d (all parents attempted)", got, len(ISOCopyJobs))
	}
}

package users

import (
	"errors"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
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

// TestExecuteCopyJobs_SkipsMissingOptionalSource verifies a cosmetic file that
// is absent from the live ISO is skipped instead of aborting the install.
func TestExecuteCopyJobs_SkipsMissingOptionalSource(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsError([]string{"test", "-e", shared.WallpaperSourceDark}, errors.New("missing"))
	defer primitives.SetCommandRunner(rec)()

	if err := ExecuteCopyJobs(); err != nil {
		t.Fatalf("ExecuteCopyJobs() = %v, want nil for a missing optional file", err)
	}
	for _, argv := range rec.CallsFor("cp") {
		if len(argv) >= 2 && argv[len(argv)-2] == shared.WallpaperSourceDark {
			t.Error("cp ran for a source that does not exist")
		}
	}
}

// TestExecuteCopyJobs_MissingRequiredSourceFails verifies a required file (the
// skel dotfiles) still fails loudly when the ISO does not ship it.
func TestExecuteCopyJobs_MissingRequiredSourceFails(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsError([]string{"test", "-e", "/etc/skel/."}, errors.New("missing"))
	defer primitives.SetCommandRunner(rec)()

	err := ExecuteCopyJobs()
	if err == nil || !strings.Contains(err.Error(), "copy job") || !strings.Contains(err.Error(), "required file missing") {
		t.Fatalf("ExecuteCopyJobs() error = %v, want a required-file failure", err)
	}
}

// TestExecuteCopyJobs_CopyFailureOfOptionalFileFails verifies an optional file
// that exists but cannot be copied is still a real error (the optional flag
// only covers absence, not copy failures).
func TestExecuteCopyJobs_CopyFailureOfOptionalFileFails(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgsError([]string{"cp"}, errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	if err := ExecuteCopyJobs(); err == nil {
		t.Fatal("ExecuteCopyJobs() = nil, want a copy failure")
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

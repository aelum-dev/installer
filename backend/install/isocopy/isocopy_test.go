package isocopy

import (
	"errors"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// btrfsCtx is a mounted btrfs install: the shape every copy test needs, and the one
// that gets the whole job list. The snapshot-restore group is btrfs-only, so the ext4
// case is its own test.
func btrfsCtx() *shared.InstallContext {
	return &shared.InstallContext{Username: "aelum", FileSystem: shared.FileSystemBtrfs}
}

func TestJobs_NotEmpty(t *testing.T) {
	if len(Jobs) == 0 {
		t.Error("expected a non-empty job list")
	}
}

// TestJobs_Invariants pins the safety properties of every copy job: the source is
// absolute and the destination lands inside the installed system at /mnt. A job that
// failed either would copy to the live ISO or crash the step.
func TestJobs_Invariants(t *testing.T) {
	for _, job := range Jobs {
		if !strings.HasPrefix(job.Source, "/") {
			t.Errorf("copy job source %q is not absolute", job.Source)
		}
		if !strings.HasPrefix(job.Dest, shared.MountRoot+"/") {
			t.Errorf("copy job destination %q is not under %s", job.Dest, shared.MountRoot)
		}
	}
}

// snapshotJobPaths is the restore path for a booted snapshot: the helper, its polkit
// action and grant, the dialog with its translated copy, and the unit that runs it.
// They are source in the live ISO (live-iso/airootfs/...), so the installer's whole
// part of this feature is these jobs - which is why the paths they copy to are
// asserted against the shared constants here: a shipped file that lands anywhere else
// is a file pkexec, systemd and Quickshell will never find.
var snapshotJobPaths = []string{
	shared.SnapshotRestoreScriptPath,
	shared.SnapshotRestorePolicyPath,
	shared.SnapshotRestoreRulesPath,
	shared.SnapshotPromptConfigDir,
	shared.SnapshotPromptUnitDir + "/" + shared.SnapshotPromptUnitName,
}

// findJobFor returns the job carrying `path`, if there is one. A directory job names
// its source with a trailing "/." (the whole contents, not the directory), so the name
// is compared without it.
func findJobFor(jobs []Job, path string) *Job {
	for i, job := range jobs {
		if strings.TrimSuffix(job.Source, "/.") == path {
			return &jobs[i]
		}
	}
	return nil
}

// TestJobs_CarrysSnapshotRestore verifies the restore path is carried by a job for
// every one of its files, that the job's source is the path the live ISO ships it at,
// and that its destination is that same path under the mounted system.
func TestJobs_CarrysSnapshotRestore(t *testing.T) {
	for _, want := range snapshotJobPaths {
		found := findJobFor(Jobs, want)
		if found == nil {
			t.Errorf("no copy job carries %s: the snapshot prompt would not be installed", want)
			continue
		}
		wantDest := shared.MountRoot + strings.TrimSuffix(found.Source, "/.")
		if strings.HasSuffix(found.Source, "/.") {
			wantDest += "/"
		}
		if found.Dest != wantDest {
			t.Errorf("%s copies to %q, want %q", want, found.Dest, wantDest)
		}
		// -a is the mode-preserving copy. The helper's 0755 is the one thing in this
		// group a plain copy would lose, and pkexec refuses to exec a program that is
		// not root-owned and executable.
		if !hasFlag(found.Flags, "-a") {
			t.Errorf("%s is copied without -a; its mode would not survive", want)
		}
		if !found.BtrfsOnly {
			t.Errorf("%s is not marked btrfs-only; an ext4 install has no snapshot to restore", want)
		}
		if !found.Optional {
			t.Errorf("%s is required; an installer newer than its ISO would fail the install over it", want)
		}
	}
}

// TestFor_NoSnapshotFilesForExt4: an ext4 install gets no snapshots, so it gets none
// of the files that only serve them. verify-installed.sh asserts the same absence on
// the real disk.
func TestFor_NoSnapshotFilesForExt4(t *testing.T) {
	jobs := For(&shared.InstallContext{Username: "aelum", FileSystem: shared.FileSystemExt4})
	for _, job := range jobs {
		for _, want := range snapshotJobPaths {
			if strings.TrimSuffix(job.Source, "/.") == want {
				t.Errorf("an ext4 install would copy %s", want)
			}
		}
	}
	if len(jobs) >= len(Jobs) {
		t.Errorf("ext4 got %d of %d jobs; the snapshot group should be excluded", len(jobs), len(Jobs))
	}

	// The files every install needs are still there: this is an exclusion, not a
	// different list.
	for _, job := range Jobs {
		if job.BtrfsOnly {
			continue
		}
		if findJobFor(jobs, strings.TrimSuffix(job.Source, "/.")) == nil {
			t.Errorf("%s is missing from an ext4 install", job.Source)
		}
	}
}

// TestFor_BtrfsGetsTheSnapshotRestore: the same list, with the restore path on a
// filesystem that has snapshots.
func TestFor_BtrfsGetsTheSnapshotRestore(t *testing.T) {
	jobs := For(btrfsCtx())
	if len(jobs) != len(Jobs) {
		t.Errorf("a btrfs install got %d of %d jobs", len(jobs), len(Jobs))
	}
	for _, want := range snapshotJobPaths {
		if findJobFor(jobs, want) == nil {
			t.Errorf("a btrfs install would not copy %s", want)
		}
	}
}

func hasFlag(flags []string, want string) bool {
	for _, flag := range flags {
		if flag == want {
			return true
		}
	}
	return false
}

// TestCopy_RunsEveryJob verifies every job produces a mkdir for its parent directory
// and a cp of its source to /mnt.
func TestCopy_RunsEveryJob(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	if err := copy(btrfsCtx()); err != nil {
		t.Fatalf("copy() = %v, want nil", err)
	}

	if got := len(rec.CallsFor("mkdir")); got != len(Jobs) {
		t.Errorf("mkdir calls = %d, want %d", got, len(Jobs))
	}
	if got := len(rec.CallsFor("cp")); got != len(Jobs) {
		t.Errorf("cp calls = %d, want %d", got, len(Jobs))
	}

	// Every copy destination must land inside the installed system at /mnt.
	for _, argv := range rec.CallsFor("cp") {
		dst := argv[len(argv)-1]
		if !strings.HasPrefix(dst, "/mnt") {
			t.Errorf("cp destination %q does not start with /mnt (argv %v)", dst, argv)
		}
	}
}

// TestCopy_MkdirFailure verifies a per-job mkdir failure is counted and reported, and
// that the copy for that job is skipped.
func TestCopy_MkdirFailure(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("mkdir", errors.New("permission denied"))
	defer primitives.SetCommandRunner(rec)()

	err := copy(btrfsCtx())
	if err == nil {
		t.Fatal("copy() = nil, want error")
	}
	if !strings.Contains(err.Error(), "copy job") {
		t.Errorf("error = %q, want it to mention copy jobs", err)
	}
	if rec.Called("cp") {
		t.Error("cp ran even though mkdir failed")
	}
}

// TestCopy_SkipsMissingOptionalSource verifies a cosmetic file that is absent from the
// live ISO is skipped instead of aborting the install.
func TestCopy_SkipsMissingOptionalSource(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsError([]string{"test", "-e", shared.WallpaperSourceDark}, errors.New("missing"))
	defer primitives.SetCommandRunner(rec)()

	if err := copy(btrfsCtx()); err != nil {
		t.Fatalf("copy() = %v, want nil for a missing optional file", err)
	}
	for _, argv := range rec.CallsFor("cp") {
		if len(argv) >= 2 && argv[len(argv)-2] == shared.WallpaperSourceDark {
			t.Error("cp ran for a source that does not exist")
		}
	}
}

// TestCopy_SkipsMissingSnapshotFiles verifies the shipped restore path is optional in
// the same sense the branding files are: an installer newer than the ISO it is running
// from warns and finishes, instead of failing an install over files that ISO never
// carried. verify-installed.sh is where that absence becomes a hard failure.
func TestCopy_SkipsMissingSnapshotFiles(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsError([]string{"test", "-e", shared.SnapshotRestoreScriptPath}, errors.New("missing")).
		OnArgsError([]string{"test", "-e", shared.SnapshotPromptConfigDir + "/."}, errors.New("missing"))
	defer primitives.SetCommandRunner(rec)()

	if err := copy(btrfsCtx()); err != nil {
		t.Fatalf("copy() = %v, want nil for files the ISO did not ship", err)
	}
	for _, argv := range rec.CallsFor("cp") {
		if len(argv) >= 2 && argv[len(argv)-2] == shared.SnapshotRestoreScriptPath {
			t.Error("cp ran for a source that does not exist")
		}
	}
}

// TestCopy_MissingRequiredSourceFails verifies a required file (the skel dotfiles)
// still fails loudly when the ISO does not ship it.
func TestCopy_MissingRequiredSourceFails(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsError([]string{"test", "-e", "/etc/skel/."}, errors.New("missing"))
	defer primitives.SetCommandRunner(rec)()

	err := copy(btrfsCtx())
	if err == nil || !strings.Contains(err.Error(), "copy job") || !strings.Contains(err.Error(), "required file missing") {
		t.Fatalf("copy() error = %v, want a required-file failure", err)
	}
}

// TestCopy_CopyFailureOfOptionalFileFails verifies an optional file that exists but
// cannot be copied is still a real error (the optional flag only covers absence, not
// copy failures).
func TestCopy_CopyFailureOfOptionalFileFails(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgsError([]string{"cp"}, errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	if err := copy(btrfsCtx()); err == nil {
		t.Fatal("copy() = nil, want a copy failure")
	}
}

// TestCopy_CopyFailure verifies a failing cp is reported after the remaining
// directories are prepared.
func TestCopy_CopyFailure(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("cp", errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	err := copy(btrfsCtx())
	if err == nil {
		t.Fatal("copy() = nil, want error")
	}
	if !strings.Contains(err.Error(), "copy job") {
		t.Errorf("error = %q, want it to mention copy jobs", err)
	}
	if got := len(rec.CallsFor("mkdir")); got != len(Jobs) {
		t.Errorf("mkdir calls = %d, want %d (all parents attempted)", got, len(Jobs))
	}
}

// TestRun_CompilesTheCopiedDconf: the copied keyfiles are inert until dconf builds
// the databases from them, so the compile belongs with the copy - and it is
// best-effort, because dconf is only a transitive dependency of the desktop packages
// and a missing one is not a reason to abort an install that already copied everything.
func TestRun_CompilesTheCopiedDconf(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	if err := Run(btrfsCtx()); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if !rec.Called("arch-chroot") {
		t.Errorf("dconf was never run; the copied keyfiles would stay inert; calls = %v", rec.Calls())
	}
}

// TestRun_DconfFailureIsNotAFailure is the same rule the copy of an optional file
// follows: a desktop that works with its defaults is a working desktop.
func TestRun_DconfFailureIsNotAFailure(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("arch-chroot", errors.New("dconf: command not found"))
	defer primitives.SetCommandRunner(rec)()

	if err := Run(btrfsCtx()); err != nil {
		t.Fatalf("Run() = %v, want nil when only dconf failed", err)
	}
}

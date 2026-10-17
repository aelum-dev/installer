package primitives_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// TestValidateMountedSystem_NotMounted verifies the precondition fails when
// findmnt reports no source for /mnt, and that it probes the mount root.
func TestValidateMountedSystem_NotMounted(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("findmnt", errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	err := primitives.ValidateMountedSystem()
	if err == nil {
		t.Fatal("ValidateMountedSystem() = nil, want error")
	}
	if !strings.Contains(err.Error(), "not mounted") {
		t.Errorf("error = %q, want it to mention %q", err, "not mounted")
	}

	calls := rec.CallsFor("findmnt")
	if len(calls) != 1 {
		t.Fatalf("findmnt called %d times, want 1", len(calls))
	}
	want := []string{"findmnt", "-n", "-o", "SOURCE", shared.MountRoot}
	if !reflect.DeepEqual(calls[0], want) {
		t.Errorf("findmnt argv = %v, want %v", calls[0], want)
	}
}

// TestValidateMountedSystem_Mounted verifies a resolvable source passes.
func TestValidateMountedSystem_Mounted(t *testing.T) {
	rec := primitivestest.NewRecorder().OnOutput("findmnt", "/dev/vda2\n")
	defer primitives.SetCommandRunner(rec)()

	if err := primitives.ValidateMountedSystem(); err != nil {
		t.Fatalf("ValidateMountedSystem() = %v, want nil", err)
	}
}

// TestCleanupMounts_NoHolders verifies the happy path: no processes hold the
// mount, so only a recursive umount runs (no kill is issued).
func TestCleanupMounts_NoHolders(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnOutput("fuser", "").
		OnOutput("umount", "")
	defer primitives.SetCommandRunner(rec)()

	if err := primitives.CleanupMounts(nil); err != nil {
		t.Fatalf("CleanupMounts() = %v, want nil", err)
	}
	if rec.Called("kill") {
		t.Error("kill was invoked when no processes held the mount")
	}
	calls := rec.CallsFor("umount")
	if len(calls) != 1 {
		t.Fatalf("umount called %d times, want 1", len(calls))
	}
	want := []string{"umount", "-R", shared.MountRoot}
	if !reflect.DeepEqual(calls[0], want) {
		t.Errorf("umount argv = %v, want %v", calls[0], want)
	}
}

// TestCleanupMounts_KillsHolders verifies holding PIDs are killed before the
// umount. kill -0 is scripted to fail so the wait loop exits immediately.
func TestCleanupMounts_KillsHolders(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnOutput("fuser", "4242 5555\n").
		OnArgsError([]string{"kill", "-0"}, errors.New("No such process")).
		OnOutput("umount", "")
	defer primitives.SetCommandRunner(rec)()

	if err := primitives.CleanupMounts(nil); err != nil {
		t.Fatalf("CleanupMounts() = %v, want nil", err)
	}

	killed := make(map[string]bool)
	for _, argv := range rec.CallsFor("kill") {
		if len(argv) >= 3 && argv[1] == "-KILL" {
			killed[argv[2]] = true
		}
	}
	for _, pid := range []string{"4242", "5555"} {
		if !killed[pid] {
			t.Errorf("kill -KILL was not issued for PID %s", pid)
		}
	}
}

// TestCleanupMounts_UnmountFails verifies a genuine umount failure is surfaced.
func TestCleanupMounts_UnmountFails(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnOutput("fuser", "").
		OnArgs([]string{"umount", "-R"}, primitivestest.Result{
			Output: []byte("device is busy"),
			Err:    errors.New("exit status 32"),
		})
	defer primitives.SetCommandRunner(rec)()

	err := primitives.CleanupMounts(nil)
	if err == nil {
		t.Fatal("CleanupMounts() = nil, want error")
	}
	if !strings.Contains(err.Error(), "umount -R") {
		t.Errorf("error = %q, want it to mention %q", err, "umount -R")
	}
}

// TestCleanupMounts_AlreadyUnmounted verifies an "not mounted" umount error is
// treated as success (idempotent cleanup).
func TestCleanupMounts_AlreadyUnmounted(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnOutput("fuser", "").
		OnArgs([]string{"umount", "-R"}, primitivestest.Result{
			Output: []byte("umount: /mnt: not mounted"),
			Err:    errors.New("exit status 32"),
		})
	defer primitives.SetCommandRunner(rec)()

	if err := primitives.CleanupMounts(nil); err != nil {
		t.Fatalf("CleanupMounts() = %v, want nil for not-mounted", err)
	}
}

// TestRunChroot verifies the chroot command is assembled as
// arch-chroot /mnt <args...> and its combined output is returned.
func TestRunChroot(t *testing.T) {
	rec := primitivestest.NewRecorder().OnOutput("arch-chroot", "ok\n")
	defer primitives.SetCommandRunner(rec)()

	out, err := primitives.RunChroot("pacman", "-Q", "base")
	if err != nil {
		t.Fatalf("RunChroot() = %v, want nil", err)
	}
	if string(out) != "ok\n" {
		t.Errorf("RunChroot() output = %q, want %q", out, "ok\n")
	}
	calls := rec.CallsFor("arch-chroot")
	if len(calls) != 1 {
		t.Fatalf("arch-chroot called %d times, want 1", len(calls))
	}
	want := []string{"arch-chroot", shared.MountRoot, "pacman", "-Q", "base"}
	if !reflect.DeepEqual(calls[0], want) {
		t.Errorf("arch-chroot argv = %v, want %v", calls[0], want)
	}
}

// TestSetCommandRunner_Restores verifies the restore function reinstates the
// real exec runner so production behavior is unaffected after a test.
func TestSetCommandRunner_Restores(t *testing.T) {
	rec := primitivestest.NewRecorder().OnOutput("true", "fake")
	restore := primitives.SetCommandRunner(rec)

	out, err := primitives.Run("true").Output()
	if err != nil || string(out) != "fake" {
		t.Fatalf("with fake runner: out=%q err=%v, want %q", out, err, "fake")
	}

	restore()

	// After restore the default runner shells out for real.
	if err := primitives.Run("true").Run(); err != nil {
		t.Fatalf("after restore, Run(\"true\") = %v, want nil", err)
	}
}

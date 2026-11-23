package gui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	present := filepath.Join(dir, "present")
	if err := os.WriteFile(present, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if !fileExists(present) {
		t.Errorf("fileExists(%q) = false, want true", present)
	}
	if fileExists(filepath.Join(dir, "absent")) {
		t.Error("fileExists(missing) = true, want false")
	}
}

func TestFindCLIBinary_PrefersExeDir(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	path := filepath.Join(filepath.Dir(exe), "aelum-installer-cli")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write sibling CLI: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	if got := findCLIBinary(); got != path {
		t.Errorf("findCLIBinary() = %q, want the sibling next to the executable (%q)", got, path)
	}
}

func TestFindCLIBinary_FallsBackToPath(t *testing.T) {
	// No sibling next to the test binary and nothing at the fixed absolute
	// locations: the bare name is returned so exec resolves it via PATH.
	if p := filepath.Join("/usr/local/bin", "aelum-installer-cli"); fileExists(p) {
		t.Skipf("a real CLI exists at %s", p)
	}
	if p := filepath.Join("/usr/bin", "aelum-installer-cli"); fileExists(p) {
		t.Skipf("a real CLI exists at %s", p)
	}

	if got := findCLIBinary(); got != "aelum-installer-cli" {
		t.Errorf("findCLIBinary() = %q, want the bare fallback name", got)
	}
}

// TestStartInstallation_NoDisk verifies the GUI refuses to start a real
// (non-dry-run) install without a target disk instead of spawning a CLI that
// exits immediately with usage text.
func TestStartInstallation_NoDisk(t *testing.T) {
	a := NewApp(false)

	err := a.StartInstallation()
	if err == nil || !strings.Contains(err.Error(), "no target disk selected") {
		t.Fatalf("StartInstallation() error = %v, want no-disk rejection", err)
	}
}

func TestStartInstallation_AlreadyRunning(t *testing.T) {
	a := NewApp(true)
	a.installMu.Lock()
	a.installationRunning = true
	a.installMu.Unlock()

	err := a.StartInstallation()
	if err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("StartInstallation() error = %v, want already-in-progress", err)
	}
}

func TestStartInstallation_StartFails(t *testing.T) {
	// A non-executable file named like the CLI on PATH makes exec fail to start
	// it, exercising the failure path without spawning a real install. Dry-run
	// skips the sudo prefix, so no privileges are needed.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "aelum-installer-cli"), []byte("not executable"), 0o644); err != nil {
		t.Fatalf("write non-executable CLI: %v", err)
	}
	t.Setenv("PATH", dir)

	a := NewApp(true)
	err := a.StartInstallation()
	if err == nil || !strings.Contains(err.Error(), "failed to start CLI") {
		t.Fatalf("StartInstallation() error = %v, want start failure", err)
	}
}

func TestCancelInstallation_SignalsProcess(t *testing.T) {
	cmd := exec.Command("sleep", "5")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start sleep: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	a := NewApp(false)
	a.installMu.Lock()
	a.installCmd = cmd
	a.installMu.Unlock()

	// Must not block or panic; the process should receive SIGINT.
	a.CancelInstallation()
}

func TestCancelInstallation_NoProcessIsSafe(t *testing.T) {
	a := NewApp(false)
	a.CancelInstallation() // installCmd is nil
}

// TestCancelInstallation_MarksTheRunCancelled: the flag is how the completion
// goroutine tells a stop the person asked for from a CLI that died on its own.
func TestCancelInstallation_MarksTheRunCancelled(t *testing.T) {
	cmd := exec.Command("sleep", "5")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start sleep: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	a := NewApp(false)
	a.installMu.Lock()
	a.installCmd = cmd
	a.installMu.Unlock()

	a.CancelInstallation()

	if !a.installCancelled.Load() {
		t.Error("signalling the CLI must record the run as cancelled")
	}
}

// TestCancelInstallation_WithoutAProcessRecordsNothing: a CLI that has already
// exited failed on its own, and there is no stop to attribute to the person.
func TestCancelInstallation_WithoutAProcessRecordsNothing(t *testing.T) {
	a := NewApp(false)
	a.CancelInstallation()

	if a.installCancelled.Load() {
		t.Error("nothing was signalled, so nothing should read as cancelled")
	}
}

func TestWaitForCleanup_ClosedChannel(t *testing.T) {
	a := NewApp(false)
	a.installMu.Lock()
	a.installDone = make(chan struct{})
	close(a.installDone)
	a.installMu.Unlock()

	a.WaitForCleanup() // returns immediately
}

// TestWaitForCleanup_HungInstallStillReleasesTheCaller: OnBeforeClose calls this on
// the way to a screen the person must be able to read, so an install that never
// exits may not turn that call into a window that never answers.
func TestWaitForCleanup_HungInstallStillReleasesTheCaller(t *testing.T) {
	previous := cleanupWaitTimeout
	cleanupWaitTimeout = 50 * time.Millisecond
	t.Cleanup(func() { cleanupWaitTimeout = previous })

	a := NewApp(false)
	a.installMu.Lock()
	a.installDone = make(chan struct{}) // never closed: the CLI never exits
	a.installMu.Unlock()

	returned := make(chan struct{})
	go func() {
		a.WaitForCleanup()
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForCleanup stayed blocked on an install that never finished")
	}
}

func TestWaitForCleanup_NoChannelIsSafe(t *testing.T) {
	a := NewApp(false)
	a.WaitForCleanup() // installDone is nil
}

// TestReadCLILines_KeepsReadingPastALongLine guards the freeze: a reader that
// stops at a long line is a pipe nobody drains, so the CLI blocks on its next log
// write and the install halts mid-flight with the bar stopped and no error to
// read. bufio.Scanner ended its loop past its 64 KiB token cap, and the CLI's
// output holds lines that grow — it echoes what a command printed; the line
// waiting behind a long one can be the 100% progress event.
func TestReadCLILines_KeepsReadingPastALongLine(t *testing.T) {
	long := strings.Repeat("p", 200*1024)
	event := `{"stepId":"install_desktop","label":"desktop system","percent":100,"phase":"done"}`

	var lengths []int
	var last string
	readCLILines("stderr", strings.NewReader(long+"\n"+event+"\n"), func(line string) {
		lengths = append(lengths, len(line))
		last = line
	})

	if len(lengths) != 2 {
		t.Fatalf("lines delivered = %d (%v), want the long line and the event after it", len(lengths), lengths)
	}
	if lengths[0] != len(long) {
		t.Errorf("first line = %d bytes, want the whole line of %d", lengths[0], len(long))
	}
	if last != event {
		t.Errorf("last line = %q, want the progress event that followed the long line", last)
	}
}

// TestReadCLILines_DeliversALineWithNoTrailingNewline: a CLI that dies mid-write
// ends its last line at EOF, and ReadString hands that fragment back together with
// the error. Dropping it would drop the reason the install ended.
func TestReadCLILines_DeliversALineWithNoTrailingNewline(t *testing.T) {
	var got []string
	readCLILines("stderr", strings.NewReader("first\r\nlast"), func(line string) {
		got = append(got, line)
	})

	if len(got) != 2 || got[0] != "first" || got[1] != "last" {
		t.Errorf("lines delivered = %q, want [first last]", got)
	}
}

// TestStartInstallation_ConcurrentCallsSpawnOneCLI is the regression for the
// unguarded spawn: the running flag used to be set only after cmd.Start, so
// overlapping calls all got past the guard and a second sudo-ed root CLI ran
// against the same disk. The shim CLI logs every spawn, so the test counts
// what was really started — not just what the bookkeeping claims.
func TestStartInstallation_ConcurrentCallsSpawnOneCLI(t *testing.T) {
	for _, dir := range []string{"/usr/local/bin", "/usr/bin"} {
		if fileExists(filepath.Join(dir, "aelum-installer-cli")) {
			t.Skipf("a real CLI exists at %s", dir)
		}
	}

	bin := t.TempDir()
	spawns := filepath.Join(bin, "spawns")
	// A stand-in CLI that never exits: a run that finished during the test
	// would release the guard and let another call start legitimately, and
	// what is under test is the guard. exec replaces the shell with sleep
	// so the SIGINT of the teardown reaches the live process.
	script := "#!/bin/sh\necho started >> " + spawns + "\nexec sleep 30\n"
	if err := os.WriteFile(filepath.Join(bin, "aelum-installer-cli"), []byte(script), 0o755); err != nil {
		t.Fatalf("write CLI shim: %v", err)
	}
	// bin first so the shim resolves ahead of any installed CLI, and the
	// real PATH stays behind it — the shim itself runs sleep.
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	a := NewApp(true) // dry-run: no sudo prefix, synthetic disk

	// The completion goroutine emits an event on the CLI's exit; a Wails
	// context is not there in a test, so record instead.
	var eventsMu sync.Mutex
	var events []string
	originalEmit := emitEvent
	emitEvent = func(_ context.Context, name string, _ ...interface{}) {
		eventsMu.Lock()
		events = append(events, name)
		eventsMu.Unlock()
	}
	// The emit seam goes back only after the stand-in CLI is released —
	// cleanups run LIFO — so even a test that fails early hands the
	// completion event to the recorder, not to the real EventsEmit that
	// demands a running Wails app.
	t.Cleanup(func() { emitEvent = originalEmit })
	t.Cleanup(func() {
		a.CancelInstallation()
		deadline := time.Now().Add(10 * time.Second)
		for a.installActive() && time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
		}
	})

	const calls = 8
	errs := make(chan error, calls)
	for i := 0; i < calls; i++ {
		go func() { errs <- a.StartInstallation() }()
	}
	started := 0
	for i := 0; i < calls; i++ {
		err := <-errs
		if err == nil {
			started++
		} else if !strings.Contains(err.Error(), "already in progress") {
			t.Errorf("StartInstallation() error = %v, want success or already-in-progress", err)
		}
	}
	if started != 1 {
		t.Errorf("%d calls started a CLI, want exactly 1", started)
	}

	// cmd.Start only forks, so wait for the shim to log its spawn. Every
	// losing call has already returned once the channel drained, so from
	// that point on the set of spawned processes is closed — whatever has
	// not written after the grace period never will.
	spawnCount := 0
	deadline := time.Now().Add(5 * time.Second)
	for spawnCount < 1 && time.Now().Before(deadline) {
		if data, err := os.ReadFile(spawns); err == nil {
			spawnCount = strings.Count(string(data), "started")
		}
		if spawnCount < 1 {
			time.Sleep(20 * time.Millisecond)
		}
	}
	time.Sleep(300 * time.Millisecond)
	if data, err := os.ReadFile(spawns); err == nil {
		spawnCount = strings.Count(string(data), "started")
	}
	if spawnCount != 1 {
		t.Errorf("the CLI was spawned %d times, want exactly 1", spawnCount)
	}

	// Release the stand-in CLI and wait for the completion goroutine to
	// clear the run state.
	a.CancelInstallation()
	releaseDeadline := time.Now().Add(10 * time.Second)
	for a.installActive() && time.Now().Before(releaseDeadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if a.installActive() {
		t.Error("the cancelled CLI never released the run state")
	}

	eventsMu.Lock()
	defer eventsMu.Unlock()
	if !slices.Contains(events, EventInstallationCancelled) {
		t.Errorf("events = %v, want the cancelled event for the stop the test asked for", events)
	}
}

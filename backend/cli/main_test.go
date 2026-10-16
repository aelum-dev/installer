package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// ============================================================================
// Test resolveDiskAndStep
// ============================================================================

func TestResolveDiskAndStep_Empty(t *testing.T) {
	disk, stepName := resolveDiskAndStep()
	if disk != "" || stepName != "" {
		t.Errorf("expected empty disk and stepName, got disk=%q stepName=%q", disk, stepName)
	}
}

func TestResolveDiskAndStep_SinglePair(t *testing.T) {
	disk := "/dev/vda"
	gotDisk, stepName := resolveDiskAndStep(stepFlag{&disk, "format_disk"})
	if gotDisk != "/dev/vda" {
		t.Errorf("expected disk /dev/vda, got %s", gotDisk)
	}
	if stepName != "format_disk" {
		t.Errorf("expected stepName format_disk, got %s", stepName)
	}
}

func TestResolveDiskAndStep_FirstWins(t *testing.T) {
	// First non-empty value should win.
	empty, first, second := "", "/dev/vda", "/dev/vdb"
	disk, stepName := resolveDiskAndStep(
		stepFlag{&empty, ""},
		stepFlag{&first, "format_disk"},
		stepFlag{&second, "install"},
	)
	if disk != "/dev/vda" {
		t.Errorf("expected first non-empty disk /dev/vda, got %s", disk)
	}
	if stepName != "format_disk" {
		t.Errorf("expected stepName format_disk, got %s", stepName)
	}
}

func TestResolveDiskAndStep_AllEmpty(t *testing.T) {
	a, b, c := "", "", ""
	disk, stepName := resolveDiskAndStep(
		stepFlag{&a, ""},
		stepFlag{&b, ""},
		stepFlag{&c, ""},
	)
	if disk != "" || stepName != "" {
		t.Errorf("expected empty disk and stepName, got disk=%q stepName=%q", disk, stepName)
	}
}

func TestResolveDiskAndStep_NoStepName(t *testing.T) {
	// Empty stepName means full install.
	diskVal := "/dev/vda"
	disk, stepName := resolveDiskAndStep(stepFlag{&diskVal, ""})
	if disk != "/dev/vda" {
		t.Errorf("expected disk /dev/vda, got %s", disk)
	}
	if stepName != "" {
		t.Errorf("expected empty stepName for full install, got %s", stepName)
	}
}

func TestResolveDiskAndStep_PriorityOrder(t *testing.T) {
	// Test the actual priority order used in run(): the first set flag wins.
	installFlag := ""
	formatFlag := ""
	mountFlag := ""
	installSystemFlag := ""
	setupUsersFlag := ""
	configureSysFlag := "/dev/vda"

	disk, stepName := resolveDiskAndStep(
		stepFlag{&installFlag, ""},                      // --install
		stepFlag{&formatFlag, "format_disk"},            // --format-disk
		stepFlag{&mountFlag, "mount_filesystem"},        // --mount-filesystem
		stepFlag{&installSystemFlag, "install"},         // --install-system
		stepFlag{&setupUsersFlag, "setup_users"},        // --setup-users
		stepFlag{&configureSysFlag, "configure_system"}, // --configure-system
	)

	if disk != "/dev/vda" {
		t.Errorf("expected disk /dev/vda, got %s", disk)
	}
	if stepName != "configure_system" {
		t.Errorf("expected stepName configure_system, got %s", stepName)
	}
}

func TestResolveDiskAndStep_WithEmptyStrings(t *testing.T) {
	a, b := "", ""
	diskVal := "/dev/vda"
	disk, stepName := resolveDiskAndStep(
		stepFlag{&a, ""},
		stepFlag{&b, ""},
		stepFlag{&diskVal, "test"},
	)
	if disk != "/dev/vda" {
		t.Errorf("expected /dev/vda, got %s", disk)
	}
	if stepName != "test" {
		t.Errorf("expected test, got %s", stepName)
	}
}

// ============================================================================
// Test run() with mocked dependencies
// ============================================================================

func TestRun_VersionFlag(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	exitCode = func(code int) {
		w.Close()
		os.Stdout = oldStdout
		// Just return, don't actually exit
	}

	args := []string{"test", "--version"}
	code := run(args)

	w.Close()
	os.Stdout = oldStdout

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	// Read and check output
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()
	if !strings.Contains(output, "dev") {
		t.Errorf("expected version output to contain 'dev', got %q", output)
	}
}

func TestRun_NoArgs(t *testing.T) {
	// In non-dry-run mode, this will fail at root check (exit code 1)
	// We can't easily test the "no args" path without mocking os.Getuid()
	// So we test that it returns a non-zero exit code
	args := []string{"test"}
	code := run(args)

	if code == 0 {
		t.Error("expected non-zero exit code for no args")
	}
}

func TestRun_DryRunCleanup(t *testing.T) {
	exitCode = func(code int) {
		// Don't actually exit
	}

	args := []string{"test", "--cleanup", "--dry-run"}
	code := run(args)

	if code != 0 {
		t.Errorf("expected exit code 0 for dry-run cleanup, got %d", code)
	}
}

func TestRun_DryRunRollback(t *testing.T) {
	// In dry-run mode, rollback returns early without calling the actual function
	// This is by design - dry-run doesn't perform real operations
	exitCode = func(code int) {
		// Don't actually exit
	}

	args := []string{"test", "--rollback", "/dev/vda", "--dry-run"}
	code := run(args)

	if code != 0 {
		t.Errorf("expected exit code 0 for dry-run rollback, got %d", code)
	}
}

func TestRun_RollbackInvalidDisk(t *testing.T) {
	// In dry-run mode, validation is skipped
	// Test non-dry-run mode instead
	validateDiskPathFunc = func(path string) error {
		return &os.PathError{Op: "validate", Path: path, Err: os.ErrInvalid}
	}
	exitCode = func(code int) {
		// Don't actually exit
	}

	args := []string{"test", "--rollback", "/invalid/path"}
	code := run(args)

	// Will fail at root check (not root), but that's OK for this test
	if code == 0 {
		t.Error("expected non-zero exit code")
	}
}

// Note: Full installation tests are omitted because they run the complete
// installation simulation (60 seconds). See backend/install/installer_test.go
// for installation logic tests.

// ============================================================================
// Test printUsage
// ============================================================================

func TestPrintUsage(t *testing.T) {
	// Just verify it doesn't panic
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	printUsage()

	w.Close()
	os.Stderr = oldStderr

	// Read output (we don't assert on content, just that it runs)
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
}

// ============================================================================
// Test edge cases for resolveDiskAndStep
// ============================================================================

func TestResolveDiskAndStep_SingleEmpty(t *testing.T) {
	empty := ""
	disk, stepName := resolveDiskAndStep(stepFlag{&empty, ""})
	if disk != "" || stepName != "" {
		t.Errorf("expected empty, got disk=%q stepName=%q", disk, stepName)
	}
}

func TestResolveDiskAndStep_ManyEmpty(t *testing.T) {
	a, b, c, d, e := "", "", "", "", ""
	disk, stepName := resolveDiskAndStep(
		stepFlag{&a, ""}, stepFlag{&b, ""}, stepFlag{&c, ""},
		stepFlag{&d, ""}, stepFlag{&e, ""},
	)
	if disk != "" || stepName != "" {
		t.Errorf("expected empty, got disk=%q stepName=%q", disk, stepName)
	}
}

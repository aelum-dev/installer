package main

import (
	"errors"
	"os"
	"strings"
	"testing"

	"aelum-installer/backend/shared"
)

// withStdin points os.Stdin at a temp file holding input and resets the shared
// buffered reader, so the non-interactive stdin helpers read from it. It returns
// the previous reader for restoration via t.Cleanup.
func withStdin(t *testing.T, input string) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatalf("create temp stdin: %v", err)
	}
	if _, err := f.WriteString(input); err != nil {
		t.Fatalf("write temp stdin: %v", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatalf("seek temp stdin: %v", err)
	}
	oldStdin := os.Stdin
	oldReader := stdinReader
	os.Stdin = f
	stdinReader = nil
	t.Cleanup(func() {
		os.Stdin = oldStdin
		stdinReader = oldReader
		_ = f.Close()
	})
}

// --- parseManualLayout -------------------------------------------------------

func TestParseManualLayout(t *testing.T) {
	layout, err := parseManualLayout(`{"disk":"/dev/vda","diskSizeMB":20480,"partitions":[{"path":"/dev/vda1","type":"esp","sizeMB":512}]}`)
	if err != nil {
		t.Fatalf("parseManualLayout() = %v", err)
	}
	if layout.Disk != "/dev/vda" || layout.DiskSizeMB != 20480 || len(layout.Partitions) != 1 {
		t.Errorf("layout = %+v", layout)
	}
	if layout.Partitions[0].Type != shared.TypeESP {
		t.Errorf("partition type = %q", layout.Partitions[0].Type)
	}
}

func TestParseManualLayout_Invalid(t *testing.T) {
	if _, err := parseManualLayout("{not json"); err == nil || !strings.Contains(err.Error(), "failed to parse") {
		t.Fatalf("parseManualLayout(invalid) error = %v", err)
	}
}

// --- resolveUsername ---------------------------------------------------------

func TestResolveUsername_FlagWins(t *testing.T) {
	old := readUsernameFunc
	readUsernameFunc = func() (string, error) { return "should-not-be-used", nil }
	defer func() { readUsernameFunc = old }()

	got, err := resolveUsername("bob", false)
	if err != nil || got != "bob" {
		t.Fatalf("resolveUsername() = %q, %v; want bob", got, err)
	}
}

func TestResolveUsername_DryRunSkipsRead(t *testing.T) {
	old := readUsernameFunc
	readUsernameFunc = func() (string, error) { return "should-not-be-used", nil }
	defer func() { readUsernameFunc = old }()

	got, err := resolveUsername("", true)
	if err != nil || got != "" {
		t.Fatalf("resolveUsername(dry-run) = %q, %v; want empty", got, err)
	}
}

func TestResolveUsername_ReadsStdin(t *testing.T) {
	old := readUsernameFunc
	readUsernameFunc = func() (string, error) { return "alice", nil }
	defer func() { readUsernameFunc = old }()

	got, err := resolveUsername("", false)
	if err != nil || got != "alice" {
		t.Fatalf("resolveUsername() = %q, %v; want alice", got, err)
	}
}

func TestResolveUsername_ReadError(t *testing.T) {
	old := readUsernameFunc
	readUsernameFunc = func() (string, error) { return "", errors.New("boom") }
	defer func() { readUsernameFunc = old }()

	if _, err := resolveUsername("", false); err == nil {
		t.Fatal("resolveUsername() = nil, want error")
	}
}

// --- stdin helpers -----------------------------------------------------------

func TestReadStdinLine_Sequential(t *testing.T) {
	withStdin(t, "alice\nsecret\n")

	if got, err := readStdinLine(); err != nil || got != "alice" {
		t.Fatalf("first read = %q, %v", got, err)
	}
	if got, err := readStdinLine(); err != nil || got != "secret" {
		t.Fatalf("second read = %q, %v", got, err)
	}
	// Clean EOF with no data returns ("", nil).
	if got, err := readStdinLine(); err != nil || got != "" {
		t.Fatalf("EOF read = %q, %v", got, err)
	}
}

func TestReadStdinLine_NoTrailingNewline(t *testing.T) {
	withStdin(t, "last")

	if got, err := readStdinLine(); err != nil || got != "last" {
		t.Fatalf("read = %q, %v; want last", got, err)
	}
}

func TestReadPassword_NonTerminal(t *testing.T) {
	withStdin(t, "mypassword\n")
	got, err := readPassword()
	if err != nil || got != "mypassword" {
		t.Fatalf("readPassword() = %q, %v", got, err)
	}
}

func TestReadPassword_PreservesSpaces(t *testing.T) {
	withStdin(t, "  pass word  \n")
	got, err := readPassword()
	if err != nil || got != "  pass word  " {
		t.Fatalf("readPassword() = %q, %v; want spaces preserved", got, err)
	}
}

func TestReadUsername_NonTerminal(t *testing.T) {
	withStdin(t, "alice\n")
	got, err := readUsername()
	if err != nil || got != "alice" {
		t.Fatalf("readUsername() = %q, %v", got, err)
	}
}

// --- handleCleanup / handleRollback ------------------------------------------

func TestHandleCleanup(t *testing.T) {
	old := cleanupMountsFunc
	defer func() { cleanupMountsFunc = old }()

	dry := true
	if code := handleCleanup(&dry); code != 0 {
		t.Errorf("dry-run cleanup = %d, want 0", code)
	}

	cleanupMountsFunc = func(shared.Logger) error { return nil }
	live := false
	if code := handleCleanup(&live); code != 0 {
		t.Errorf("cleanup = %d, want 0", code)
	}

	cleanupMountsFunc = func(shared.Logger) error { return errors.New("busy") }
	if code := handleCleanup(&live); code != 1 {
		t.Errorf("failing cleanup = %d, want 1", code)
	}
}

func TestHandleRollback(t *testing.T) {
	oldValidate := validateDiskPathFunc
	oldRollback := rollbackFunc
	defer func() {
		validateDiskPathFunc = oldValidate
		rollbackFunc = oldRollback
	}()

	dry := true
	if code := handleRollback("/dev/vda", &dry); code != 0 {
		t.Errorf("dry-run rollback = %d, want 0", code)
	}

	live := false
	validateDiskPathFunc = func(string) error { return errors.New("bad path") }
	if code := handleRollback("/dev/nope", &live); code != 1 {
		t.Errorf("invalid disk rollback = %d, want 1", code)
	}

	validateDiskPathFunc = func(string) error { return nil }
	rollbackFunc = func(shared.Logger, string) error { return errors.New("wipe failed") }
	if code := handleRollback("/dev/vda", &live); code != 1 {
		t.Errorf("failing rollback = %d, want 1", code)
	}

	rollbackFunc = func(shared.Logger, string) error { return nil }
	if code := handleRollback("/dev/vda", &live); code != 0 {
		t.Errorf("rollback = %d, want 0", code)
	}
}

// --- runInstallation ---------------------------------------------------------

func TestRunInstallation_DiskPathInvalid(t *testing.T) {
	old := validateDiskPathFunc
	validateDiskPathFunc = func(string) error { return errors.New("invalid disk") }
	defer func() { validateDiskPathFunc = old }()

	if code := runInstallation("/dev/nope", "", "alice", "ext4", "", false, "", false, false); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}

func TestRunInstallation_DiskSizeInvalid(t *testing.T) {
	oldPath := validateDiskPathFunc
	oldSize := validateDiskSizeFunc
	validateDiskPathFunc = func(string) error { return nil }
	validateDiskSizeFunc = func(string) error { return errors.New("exit status 1: no such device") }
	defer func() {
		validateDiskPathFunc = oldPath
		validateDiskSizeFunc = oldSize
	}()

	if code := runInstallation("/dev/nope", "", "alice", "ext4", "", false, "", false, false); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}

func TestRunInstallation_PasswordFailures(t *testing.T) {
	oldPath := validateDiskPathFunc
	oldSize := validateDiskSizeFunc
	oldPw := readPasswordFunc
	validateDiskPathFunc = func(string) error { return nil }
	validateDiskSizeFunc = func(string) error { return nil }
	defer func() {
		validateDiskPathFunc = oldPath
		validateDiskSizeFunc = oldSize
		readPasswordFunc = oldPw
	}()

	readPasswordFunc = func() (string, error) { return "", errors.New("tty gone") }
	if code := runInstallation("/dev/vda", "install", "alice", "ext4", "", false, "", false, false); code != 1 {
		t.Errorf("password read error exit = %d, want 1", code)
	}

	readPasswordFunc = func() (string, error) { return "", nil }
	if code := runInstallation("/dev/vda", "install", "alice", "ext4", "", false, "", false, false); code != 1 {
		t.Errorf("empty password exit = %d, want 1", code)
	}
}

func TestRunInstallation_InvalidUsername(t *testing.T) {
	oldPath := validateDiskPathFunc
	oldSize := validateDiskSizeFunc
	oldPw := readPasswordFunc
	validateDiskPathFunc = func(string) error { return nil }
	validateDiskSizeFunc = func(string) error { return nil }
	readPasswordFunc = func() (string, error) { return "x", nil }
	defer func() {
		validateDiskPathFunc = oldPath
		validateDiskSizeFunc = oldSize
		readPasswordFunc = oldPw
	}()

	for _, username := range []string{"123", "root", "-bad"} {
		if code := runInstallation("/dev/vda", "", username, "ext4", "", false, "", false, false); code != 1 {
			t.Errorf("runInstallation(username=%q) = %d, want 1", username, code)
		}
	}
}

func TestRunInstallation_LuksPassphraseFailure(t *testing.T) {
	oldPath := validateDiskPathFunc
	oldSize := validateDiskSizeFunc
	oldPw := readPasswordFunc
	validateDiskPathFunc = func(string) error { return nil }
	validateDiskSizeFunc = func(string) error { return nil }
	defer func() {
		validateDiskPathFunc = oldPath
		validateDiskSizeFunc = oldSize
		readPasswordFunc = oldPw
	}()

	calls := 0
	readPasswordFunc = func() (string, error) {
		calls++
		if calls == 1 {
			return "account-pw", nil
		}
		return "", errors.New("no luks tty")
	}
	if code := runInstallation("/dev/vda", "install", "alice", "ext4", "", false, "", true, false); code != 1 {
		t.Errorf("luks passphrase error exit = %d, want 1", code)
	}

	calls = 0
	readPasswordFunc = func() (string, error) {
		calls++
		if calls == 1 {
			return "account-pw", nil
		}
		return "", nil
	}
	if code := runInstallation("/dev/vda", "install", "alice", "ext4", "", false, "", true, false); code != 1 {
		t.Errorf("empty luks passphrase exit = %d, want 1", code)
	}
}

func TestRunInstallation_InvalidManualLayout(t *testing.T) {
	oldPath := validateDiskPathFunc
	validateDiskPathFunc = func(string) error { return nil }
	defer func() { validateDiskPathFunc = oldPath }()

	// Dry-run skips size/password reads, so the layout parse is the first failure.
	if code := runInstallation("/dev/vda", "", "", "ext4", "", true, "{bad json", false, false); code != 1 {
		t.Errorf("invalid layout exit = %d, want 1", code)
	}
}

func TestRunInstallation_InfeasibleManualLayout(t *testing.T) {
	oldPath := validateDiskPathFunc
	validateDiskPathFunc = func(string) error { return nil }
	defer func() { validateDiskPathFunc = oldPath }()

	// No ESP/System designations → ValidateFeasible rejects it.
	layout := `{"disk":"/dev/vda","diskSizeMB":20480,"diskSizeBytes":21474836480,"sectorSizeBytes":512,"partitions":[]}`
	if code := runInstallation("/dev/vda", "", "", "ext4", "", true, layout, false, false); code != 1 {
		t.Errorf("infeasible layout exit = %d, want 1", code)
	}
}

func TestRunInstallation_DryRunSingleStep(t *testing.T) {
	oldPath := validateDiskPathFunc
	validateDiskPathFunc = func(string) error { return nil }
	defer func() { validateDiskPathFunc = oldPath }()

	// A single step in dry-run simulates the step without touching a disk.
	if code := runInstallation("/dev/vda", "format_disk", "", "ext4", "", true, "", false, false); code != 0 {
		t.Errorf("dry-run single step exit = %d, want 0", code)
	}
}

// --- main --------------------------------------------------------------------

func TestMain_DelegatesToExitCode(t *testing.T) {
	oldExit := exitCode
	oldArgs := os.Args
	var got = -1
	exitCode = func(code int) { got = code }
	os.Args = []string{"aelum-installer-cli", "--version"}
	defer func() {
		exitCode = oldExit
		os.Args = oldArgs
	}()

	main()
	if got != 0 {
		t.Errorf("main() exit code = %d, want 0", got)
	}
}

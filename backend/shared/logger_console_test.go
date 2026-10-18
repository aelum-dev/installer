package shared

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureOutput redirects os.Stdout and os.Stderr to pipes, runs fn, and
// returns what each received. Callers must not run in parallel.
func captureOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stdout, os.Stderr = wOut, wErr
	defer func() {
		os.Stdout, os.Stderr = oldOut, oldErr
		_ = wOut.Close()
		_ = wErr.Close()
	}()

	fn()

	_ = wOut.Close()
	_ = wErr.Close()
	outB, _ := io.ReadAll(rOut)
	errB, _ := io.ReadAll(rErr)
	return string(outB), string(errB)
}

func outputTags(s string) string {
	var tags []string
	for _, line := range strings.Split(s, "\n") {
		idx := strings.Index(line, "] ")
		if idx < 0 {
			if line != "" {
				tags = append(tags, line)
			}
			continue
		}
		tags = append(tags, line[idx+2:])
	}
	return strings.Join(tags, "\n")
}

func TestGetLogger(t *testing.T) {
	if l := GetLogger(); l == nil {
		t.Fatal("GetLogger() = nil")
	}
}

func TestConsoleLogger_Info(t *testing.T) {
	l := NewConsoleLogger()
	defer l.Close()

	out, _ := captureOutput(t, func() { l.Info("hello %s", "world") })
	if !strings.Contains(out, "hello world") {
		t.Errorf("Info output = %q, want it to contain %q", out, "hello world")
	}
}

func TestConsoleLogger_Error(t *testing.T) {
	l := NewConsoleLogger()
	defer l.Close()

	_, errOut := captureOutput(t, func() { l.Error("boom %d", 7) })
	if !strings.Contains(errOut, ErrorPrefix+"boom 7") {
		t.Errorf("Error stderr = %q, want %q", errOut, ErrorPrefix+"boom 7")
	}
	// Errors go to stderr, not stdout.
	// (writeLine is not used by Error.)
}

func TestConsoleLogger_Command_NoProgressContext(t *testing.T) {
	l := NewConsoleLogger()
	defer l.Close()

	out, _ := captureOutput(t, func() { l.Command("pacstrap /mnt base") })
	if !strings.Contains(out, CommandOutputPrefix+"pacstrap /mnt base") {
		t.Errorf("Command output = %q, want prefixed line", out)
	}
}

func TestConsoleLogger_Command_WithProgressContext(t *testing.T) {
	l := NewConsoleLogger()
	defer l.Close()
	l.SetProgressContext("Installing", 42)

	out, _ := captureOutput(t, func() { l.Command("pacstrap /mnt base") })
	if !strings.Contains(out, "[42%] Installing => pacstrap /mnt base") {
		t.Errorf("Command output = %q, want progress-prefixed line", out)
	}
}

func TestConsoleLogger_Command_SplitsLines(t *testing.T) {
	l := NewConsoleLogger()
	defer l.Close()

	out, _ := captureOutput(t, func() { l.Command("first\nsecond\n") })
	tags := outputTags(out)
	if tags != CommandOutputPrefix+"first\n"+CommandOutputPrefix+"second" {
		t.Errorf("Command output tags = %q", tags)
	}
}

func TestConsoleLogger_Command_EmptyIsSilent(t *testing.T) {
	l := NewConsoleLogger()
	defer l.Close()

	out, errOut := captureOutput(t, func() {
		l.Command("")
		l.CommandNoNewline("")
		l.Write("")
		l.LogToFile("")
	})
	if out != "" || errOut != "" {
		t.Errorf("empty inputs produced output: stdout=%q stderr=%q", out, errOut)
	}
}

func TestConsoleLogger_CommandNoNewline(t *testing.T) {
	l := NewConsoleLogger()
	defer l.Close()
	l.UpdateContext("Configuring", 17)

	out, _ := captureOutput(t, func() { l.CommandNoNewline("mkinitcpio") })
	if !strings.Contains(out, "[17%] Configuring => mkinitcpio") {
		t.Errorf("CommandNoNewline output = %q", out)
	}
}

func TestConsoleLogger_Progress(t *testing.T) {
	l := NewConsoleLogger()
	defer l.Close()

	out, errOut := captureOutput(t, func() { l.Progress("installing base", 42) })
	if !strings.Contains(out, "[42%] installing base") {
		t.Errorf("Progress stdout = %q", out)
	}
	if !strings.Contains(errOut, `{"label":"installing base","percent":42,"phase":"running"}`) {
		t.Errorf("Progress stderr = %q, want progress JSON", errOut)
	}
	// Progress context is now current for subsequent Command lines.
	out2, _ := captureOutput(t, func() { l.Command("pacstrap") })
	if !strings.Contains(out2, "[42%] installing base => pacstrap") {
		t.Errorf("Command after Progress = %q", out2)
	}
}

func TestConsoleLogger_ProgressLine(t *testing.T) {
	l := NewConsoleLogger()
	defer l.Close()

	out, _ := captureOutput(t, func() { l.ProgressLine("halfway", 50) })
	if !strings.Contains(out, "[50%] halfway") {
		t.Errorf("ProgressLine stdout = %q", out)
	}
}

func TestConsoleLogger_ProgressDone(t *testing.T) {
	l := NewConsoleLogger()
	defer l.Close()
	l.SetProgressContext("installing", 90)

	out, _ := captureOutput(t, func() { l.ProgressDone() })
	if !strings.Contains(out, "✓") {
		t.Errorf("ProgressDone stdout = %q, want check mark", out)
	}
	// Context is cleared: the next Command line drops the progress prefix.
	out2, _ := captureOutput(t, func() { l.Command("done") })
	if !strings.Contains(out2, CommandOutputPrefix+"done") {
		t.Errorf("Command after ProgressDone = %q", out2)
	}
}

func TestConsoleLogger_ProgressJSON_EscapesLabel(t *testing.T) {
	l := NewConsoleLogger()
	defer l.Close()

	_, errOut := captureOutput(t, func() { l.ProgressJSON(`say "hi"`, 10, "running") })
	want := `{"label":"say \"hi\"","percent":10,"phase":"running"}`
	if !strings.Contains(errOut, want) {
		t.Errorf("ProgressJSON stderr = %q, want %q", errOut, want)
	}
}

func TestConsoleLogger_Write(t *testing.T) {
	l := NewConsoleLogger()
	defer l.Close()

	out, _ := captureOutput(t, func() { l.Write("plain line") })
	if !strings.Contains(out, "plain line") {
		t.Errorf("Write output = %q", out)
	}
}

func TestConsoleLogger_FileWrites(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "install.log")
	SetLogPath(logPath)
	t.Cleanup(func() { SetLogPath("") })

	marker := "coverage-marker-9f3c1a"
	l := NewConsoleLogger()
	l.LogToFile(marker)
	l.WriteErrorLog("FormatDisk", "boom")
	l.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	body := string(data)
	for _, want := range []string{marker, "=== INSTALLATION ERROR", "Step: FormatDisk", "Error: boom", "=== END ERROR ==="} {
		if !strings.Contains(body, want) {
			t.Errorf("log file missing %q", want)
		}
	}

	// Close is idempotent.
	l.Close()
}

func TestConsoleLogger_WriteLineMirrorsToFile(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "install.log")
	SetLogPath(logPath)
	t.Cleanup(func() { SetLogPath("") })

	marker := "mirror-line-4d2b"
	l := NewConsoleLogger()
	defer l.Close()

	_, _ = captureOutput(t, func() { l.Info("hello %s", marker) })

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(data), "hello "+marker) {
		t.Errorf("log file does not mirror stdout line %q", marker)
	}
}

func TestConsoleLogger_MirrorsToSharedDir(t *testing.T) {
	dir := t.TempDir()
	old := logMirrorDir
	logMirrorDir = dir
	t.Cleanup(func() {
		logMirrorDir = old
		SetLogPath("")
	})

	primary := filepath.Join(t.TempDir(), "install.log")
	SetLogPath(primary)

	l := NewConsoleLogger()
	l.LogToFile("shared-marker-7a1d")
	l.Close()

	data, err := os.ReadFile(filepath.Join(dir, "install.log"))
	if err != nil {
		t.Fatalf("read shared mirror: %v", err)
	}
	if !strings.Contains(string(data), "shared-marker-7a1d") {
		t.Errorf("shared mirror missing log line")
	}
}

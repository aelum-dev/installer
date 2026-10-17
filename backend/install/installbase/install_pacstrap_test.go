package installbase

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// fakePacstrap writes an executable "pacstrap" shim onto PATH so InstallPackages
// exercises its real process/streaming path without a real pacstrap. It returns
// the shim's directory (for scripts that drop files next to themselves).
func fakePacstrap(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "pacstrap")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatalf("write fake pacstrap: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

// mounted installs a CommandRunner whose findmnt check passes, so
// ValidateMountedSystem succeeds without a real mount.
func mounted(t *testing.T) {
	t.Helper()
	rec := primitivestest.NewRecorder().OnOutput("findmnt", "/dev/vda2\n")
	restore := primitives.SetCommandRunner(rec)
	t.Cleanup(restore)
}

func TestInstallPackages_NoPackages(t *testing.T) {
	mounted(t)

	err := InstallPackages(&shared.InstallContext{}, nil, "base system")
	if err == nil || !strings.Contains(err.Error(), "no packages") {
		t.Fatalf("InstallPackages() error = %v, want no-packages error", err)
	}
}

func TestInstallPackages_StartFails(t *testing.T) {
	mounted(t)
	// An empty PATH makes exec.Command unable to resolve pacstrap, so Start fails.
	t.Setenv("PATH", "")

	err := InstallPackages(&shared.InstallContext{}, []string{"base"}, "base system")
	if err == nil || !strings.Contains(err.Error(), "failed to start pacstrap") {
		t.Fatalf("InstallPackages() error = %v, want start failure", err)
	}
}

func TestInstallPackages_Success(t *testing.T) {
	fakePacstrap(t, `echo ":: Packages (1) base"; echo "  base downloading..."; echo "installing base..."; exit 0`)
	mounted(t)

	var pcts []int
	ctx := &shared.InstallContext{ProgressCallback: func(p int) { pcts = append(pcts, p) }}

	if err := InstallPackages(ctx, []string{"base"}, "base system"); err != nil {
		t.Fatalf("InstallPackages() = %v, want nil", err)
	}
	if len(pcts) == 0 {
		t.Error("progress callback never fired during pacstrap output")
	}
}

// TestRun_PacstrapArgs verifies Run drives pacstrap with the offline flag, the
// install root, and the base package list.
func TestRun_PacstrapArgs(t *testing.T) {
	dir := fakePacstrap(t, `printf '%s\n' "$@" > "$(dirname "$0")/args"; exit 0`)
	mounted(t)

	if err := Run(&shared.InstallContext{}); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "args"))
	if err != nil {
		t.Fatalf("read captured args: %v", err)
	}
	args := strings.Fields(string(data))
	if len(args) < 2 || args[0] != "-K" || args[1] != shared.MountRoot {
		t.Fatalf("pacstrap args = %v, want [-K %s ...]", args, shared.MountRoot)
	}
	if !contains(args, "base") || !contains(args, "linux-lts") {
		t.Errorf("pacstrap args = %v, want the base package list", args)
	}
}

func TestInstallPackages_PacstrapFails(t *testing.T) {
	fakePacstrap(t, `echo "error: boom" >&2; exit 1`)
	mounted(t)

	err := InstallPackages(&shared.InstallContext{}, []string{"base"}, "base system")
	if err == nil || !strings.Contains(err.Error(), "pacstrap failed") {
		t.Fatalf("InstallPackages() error = %v, want pacstrap failure", err)
	}
}

func TestInstallPackages_Interrupted(t *testing.T) {
	// `exec` replaces the shell with sleep so killing cmd.Process closes the
	// stdout pipe immediately (a plain `sleep` child would hold it open).
	fakePacstrap(t, `exec sleep 5`)
	mounted(t)

	ctx := &shared.InstallContext{Cancel: make(chan struct{})}
	go func() {
		time.Sleep(150 * time.Millisecond)
		close(ctx.Cancel)
	}()

	start := time.Now()
	err := InstallPackages(ctx, []string{"base"}, "base system")
	if err == nil || !strings.Contains(err.Error(), "interrupted") {
		t.Fatalf("InstallPackages() error = %v, want interrupted", err)
	}
	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Errorf("cancellation took %s; the child was not killed promptly", elapsed)
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

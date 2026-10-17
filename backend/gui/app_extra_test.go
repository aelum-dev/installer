package gui

import (
	"context"
	"strings"
	"testing"

	"aelum-installer/backend/shared"
)

func TestCheckRoot_Authorized(t *testing.T) {
	writeShim(t, "sudo", "exit 0\n")
	a := NewApp(false)

	res := a.CheckRoot()
	if !res.Authorized {
		t.Errorf("CheckRoot() = %+v, want authorized", res)
	}
}

func TestCheckRoot_Denied(t *testing.T) {
	writeShim(t, "sudo", "exit 1\n")
	a := NewApp(false)

	res := a.CheckRoot()
	if res.Authorized {
		t.Errorf("CheckRoot() = %+v, want not authorized", res)
	}
	if !strings.Contains(res.Message, "sudo") {
		t.Errorf("CheckRoot() message = %q, want it to mention sudo", res.Message)
	}
}

func TestStartUp_AuthorizedKeepsMode(t *testing.T) {
	writeShim(t, "sudo", "exit 0\n")
	a := NewApp(false)

	a.StartUp(context.Background())

	if a.ctx == nil {
		t.Error("StartUp should store the context")
	}
	if a.GetDryRun() {
		t.Error("StartUp should not enable dry-run when sudo is available")
	}
}

func TestStartUp_NotAuthorizedEnablesDryRun(t *testing.T) {
	writeShim(t, "sudo", "exit 1\n")
	old := false
	shared.SetFakeDisks(old)
	t.Cleanup(func() { shared.SetFakeDisks(old) })

	a := NewApp(false)
	a.StartUp(context.Background())

	if !a.GetDryRun() {
		t.Error("StartUp should enable dry-run without sudo")
	}
	// Fake disks are enabled so the unprivileged UI can still list disks.
	disks, err := shared.GetDisks()
	if err != nil {
		t.Fatalf("GetDisks: %v", err)
	}
	if len(disks) == 0 {
		t.Error("dry-run StartUp should enable fake disks")
	}
}

func TestSimpleGetters(t *testing.T) {
	a := NewApp(true)
	if !a.GetDryRun() {
		t.Error("GetDryRun() = false, want true")
	}
	if a.GetInstallationRunning() {
		t.Error("GetInstallationRunning() = true, want false")
	}

	a.diskSelection.DiskID = "/dev/sda"
	if got := a.GetDiskID(); got != "/dev/sda" {
		t.Errorf("GetDiskID() = %q, want /dev/sda", got)
	}
}

func TestOnBeforeClose_Idle(t *testing.T) {
	a := NewApp(false)
	if a.OnBeforeClose(context.Background()) {
		t.Error("OnBeforeClose should return false")
	}
}

func TestOnBeforeClose_DuringInstallation(t *testing.T) {
	a := NewApp(false)
	// Simulate a running install with no live process/cleanup channel: Cancel
	// and Wait must tolerate the nil handles, and GetLogger().Close is safe.
	a.installationRunning.Store(true)

	if a.OnBeforeClose(context.Background()) {
		t.Error("OnBeforeClose should return false")
	}
}

func TestRebootAndClose_SudoMissing(t *testing.T) {
	// An empty PATH makes the `sudo reboot` lookup fail, so RebootAndClose
	// reports an error instead of rebooting the test machine.
	t.Setenv("PATH", "")
	a := NewApp(false)

	if err := a.RebootAndClose(); err == nil || !strings.Contains(err.Error(), "failed to reboot") {
		t.Fatalf("RebootAndClose() error = %v, want reboot failure", err)
	}
}

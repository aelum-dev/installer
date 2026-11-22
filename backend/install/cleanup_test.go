package install

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// Rollback's contract is that it reports what it could not do: a wiped disk and a
// disk nobody could touch look identical to the next install attempt, so the only
// thing telling them apart is the error. Every test here drives that contract
// through the command seam — no root, no device.

// twoPartitions is the lsblk answer for a rolled-back disk that still holds its
// partitions and was never encrypted; it feeds discoverPartitions, lvm's device
// lookup and the mapper's disk check.
const twoPartitions = "vda      disk\nvda1     part\nvda2     part\n"

// mapperOnDisk is the same disk with the root mapper on it. lsblk names a
// device-mapper device by its mapper name, so this row is what makes an open mapper
// belong to the disk being rolled back rather than to some other disk.
const mapperOnDisk = twoPartitions + shared.LuksMapperName + "    crypt\n"

// baseRecorder scripts a rollback where nothing is left over: no volume group, no
// LUKS mapper on this disk, two partitions to wipe. Each test adds the one failure
// it is about.
func baseRecorder() *primitivestest.Recorder {
	return primitivestest.NewRecorder().
		OnArgsOutput([]string{"lsblk"}, twoPartitions).
		OnArgsError([]string{"cryptsetup", "status"}, errors.New("Device aelum is not active."))
}

// mapperRecorder scripts the killed-LUKS-run: the mapper is open and it is on this
// disk. Unmatched cryptsetup status succeeds, which is how IsOpen reads "open".
func mapperRecorder() *primitivestest.Recorder {
	return primitivestest.NewRecorder().OnArgsOutput([]string{"lsblk"}, mapperOnDisk)
}

// indexOfCall returns the position of the first recorded command with exactly this
// argv, or -1 when no command ran with it.
func indexOfCall(rec *primitivestest.Recorder, argv ...string) int {
	for i, call := range rec.Calls() {
		if reflect.DeepEqual(call.Args, argv) {
			return i
		}
	}
	return -1
}

// TestRollback_AllCommandsSucceed is the other half of the contract: a rollback
// that got through every command reports no error, so the report is never noise.
func TestRollback_AllCommandsSucceed(t *testing.T) {
	defer primitives.SetCommandRunner(baseRecorder())()

	if err := Rollback(nil, "/dev/vda"); err != nil {
		t.Errorf("Rollback() = %v, want nil when every command succeeded", err)
	}
}

// TestRollback_ReportsEveryFailedWipe is the bug this function used to have: all
// four destructive commands failed, every failure was logged and dropped, and the
// user was told the disk is ready for a fresh install.
func TestRollback_ReportsEveryFailedWipe(t *testing.T) {
	rec := baseRecorder().
		OnError("wipefs", errors.New("exit status 1")).
		OnArgsError([]string{"sgdisk"}, errors.New("exit status 3"))
	defer primitives.SetCommandRunner(rec)()

	err := Rollback(nil, "/dev/vda")
	if err == nil {
		t.Fatal("Rollback() = nil, want an error naming every command that failed")
	}
	for _, want := range []string{"wipefs /dev/vda1", "wipefs /dev/vda2", "wipefs /dev/vda", "sgdisk -Z /dev/vda"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not name %s: %v", want, err)
		}
	}
}

// TestRollback_ReportsTheCommandOutput: an exit status alone says nothing about
// why a disk refused a wipe; the tool's own line is the part that says EBUSY.
func TestRollback_ReportsTheCommandOutput(t *testing.T) {
	defer primitives.SetCommandRunner(baseRecorder().
		OnArgs([]string{"wipefs"}, primitivestest.Result{
			Output: []byte("wipefs: error: /dev/vda: probing initialization failed: Device or resource busy\n"),
			Err:    errors.New("exit status 1"),
		}))()

	err := Rollback(nil, "/dev/vda")
	if err == nil {
		t.Fatal("Rollback() = nil, want the busy-disk failure")
	}
	if !strings.Contains(err.Error(), "Device or resource busy") {
		t.Errorf("error does not carry wipefs' own reason: %v", err)
	}
}

// TestRollback_OneWipeFailsIsStillReported: the disk usually ends up blank anyway,
// but a rollback that could not finish must not claim it did.
func TestRollback_OneWipeFailsIsStillReported(t *testing.T) {
	rec := baseRecorder().
		OnArgsError([]string{"wipefs", "--all", "-f", "/dev/vda1"}, errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	if err := Rollback(nil, "/dev/vda"); err == nil {
		t.Fatal("Rollback() = nil, want the failed partition reported")
	}
	// A failure must not stop the rest of the work: everything else ran.
	for _, argv := range [][]string{
		{"wipefs", "--all", "-f", "/dev/vda2"},
		{"wipefs", "--all", "-f", "/dev/vda"},
		{"sgdisk", "-Z", "/dev/vda"},
	} {
		if indexOfCall(rec, argv...) < 0 {
			t.Errorf("%v did not run after an earlier wipe failed", argv)
		}
	}
}

// TestRollback_ClosesAnOpenMapperBeforeWiping: a run killed after cryptsetup open
// leaves the mapper holding its partition, and wipefs/sgdisk are both refused on a
// busy device. So the mapper goes first, or nothing below it can succeed.
func TestRollback_ClosesAnOpenMapperBeforeWiping(t *testing.T) {
	withFastLUKSRetry(t)
	rec := mapperRecorder()
	defer primitives.SetCommandRunner(rec)()

	if err := Rollback(nil, "/dev/vda"); err != nil {
		t.Fatalf("Rollback() = %v, want nil", err)
	}
	closed := indexOfCall(rec, "cryptsetup", "close", shared.LuksMapperName)
	if closed < 0 {
		t.Fatal("Rollback did not close the LUKS mapper; the wipes below it would fail EBUSY")
	}
	if wiped := indexOfCall(rec, "wipefs", "--all", "-f", "/dev/vda1"); wiped < 0 || closed > wiped {
		t.Errorf("mapper closed at %d, first wipe at %d: the close must come before the wipes", closed, wiped)
	}
}

// TestRollback_NoMapperIsNotAFailure: a plain, unencrypted disk has no mapper and
// cryptsetup says so. That is the normal case, not a rollback that failed.
func TestRollback_NoMapperIsNotAFailure(t *testing.T) {
	rec := baseRecorder()
	defer primitives.SetCommandRunner(rec)()

	if err := Rollback(nil, "/dev/vda"); err != nil {
		t.Errorf("Rollback() = %v, want nil for a disk without a LUKS mapper", err)
	}
	if indexOfCall(rec, "cryptsetup", "close", shared.LuksMapperName) >= 0 {
		t.Error("Rollback ran cryptsetup close for a mapper that is not open")
	}
}

// TestRollback_BusyMapperIsReported: the mapper will not close, so the disk is
// still held and nothing after it can have worked. Silent here is the same false
// success the wipe failures were.
func TestRollback_BusyMapperIsReported(t *testing.T) {
	withFastLUKSRetry(t)
	rec := mapperRecorder().
		OnArgs([]string{"cryptsetup", "close"}, primitivestest.Result{
			Output: []byte("Device aelum is still in use."),
			Err:    errors.New("exit status 5"),
		})
	defer primitives.SetCommandRunner(rec)()

	err := Rollback(nil, "/dev/vda")
	if err == nil {
		t.Fatal("Rollback() = nil, want the mapper that would not close")
	}
	if !strings.Contains(err.Error(), shared.LuksMapperName) {
		t.Errorf("error does not name the mapper: %v", err)
	}
	// The first cryptsetup call is the status probe; the rest are close attempts.
	if attempts := len(rec.CallsFor("cryptsetup")) - 1; attempts < 2 {
		t.Errorf("close attempts = %d, want a busy mapper retried before it is given up on", attempts)
	}
}

// TestRollback_LeavesAnotherDisksMapperOpen: the mapper's name is the same on every
// install, so an open one is not automatically this disk's. Closing a mapper another
// disk holds would pull the device out from under whatever is mounted on it, and a
// rollback of one disk has no business doing that.
func TestRollback_LeavesAnotherDisksMapperOpen(t *testing.T) {
	// Unmatched cryptsetup status succeeds: a mapper called "aelum" is open — but
	// this disk's tree does not carry it (no crypt row).
	rec := primitivestest.NewRecorder().OnArgsOutput([]string{"lsblk"}, twoPartitions)
	defer primitives.SetCommandRunner(rec)()

	if err := Rollback(nil, "/dev/vda"); err != nil {
		t.Errorf("Rollback() = %v, want nil: the open mapper is not on this disk", err)
	}
	if indexOfCall(rec, "cryptsetup", "close", shared.LuksMapperName) >= 0 {
		t.Error("Rollback closed a LUKS mapper belonging to another disk")
	}
}

// TestRollback_KeepsTheSharedMountRoot: /mnt is the installer's target root, not
// part of this disk. The next install creates its mount points again, so deleting
// a path another install or the live session may be using buys a rollback nothing.
func TestRollback_KeepsTheSharedMountRoot(t *testing.T) {
	rec := baseRecorder()
	defer primitives.SetCommandRunner(rec)()

	if err := Rollback(nil, "/dev/vda"); err != nil {
		t.Fatalf("Rollback() = %v, want nil", err)
	}
	if rec.Called("rmdir") {
		t.Errorf("Rollback removed the shared mount root: %v", rec.CallsFor("rmdir"))
	}
}

func TestReleaseLuksMapper_NothingToClose(t *testing.T) {
	defer primitives.SetCommandRunner(baseRecorder())()

	if err := releaseLuksMapper("/dev/vda", nil); err != nil {
		t.Errorf("releaseLuksMapper() = %v, want nil when this disk has no open mapper", err)
	}
}

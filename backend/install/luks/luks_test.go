package luks

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// TestCryptoArgs verifies the pinned LUKS2 parameters match current best
// practices (luks.md §11): LUKS2, aes-xts-plain64, 512-bit key (AES-256 under
// XTS), argon2id KDF, sha256 hash, 2000ms iter time.
func TestCryptoArgs(t *testing.T) {
	want := []string{
		"--batch-mode",
		"--type", shared.LuksType,
		"--cipher", shared.LuksCipher,
		"--key-size", "512",
		"--hash", shared.LuksHash,
		"--pbkdf", shared.LuksPBKDF,
		"--iter-time", "2000",
	}
	got := cryptoArgs()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cryptoArgs() = %v, want %v", got, want)
	}
}

// TestFormatArgs verifies luksFormat is invoked with --key-file=- (passphrase
// comes via stdin, never the args) and the target path is the final argument.
func TestFormatArgs(t *testing.T) {
	got := formatArgs("/dev/vda2")
	want := []string{"cryptsetup", "luksFormat", "--key-file", "-"}
	want = append(want, cryptoArgs()...)
	want = append(want, "/dev/vda2")
	if !reflect.DeepEqual(got, want) {
		t.Errorf("formatArgs() = %v, want %v", got, want)
	}
	// The passphrase must never appear in the argument list.
	for _, a := range got {
		if a == "secret" {
			t.Errorf("passphrase leaked into format args: %v", got)
		}
	}
}

// TestOpenArgs verifies open uses --key-file=- (stdin) and appends path + mapper.
func TestOpenArgs(t *testing.T) {
	got := openArgs("/dev/vda2", shared.LuksMapperName)
	want := []string{"cryptsetup", "open", "--type", "luks", "--key-file", "-", "/dev/vda2", shared.LuksMapperName}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("openArgs() = %v, want %v", got, want)
	}
}

// TestCloseArgs verifies close targets the mapper name only.
func TestCloseArgs(t *testing.T) {
	got := closeArgs(shared.LuksMapperName)
	want := []string{"cryptsetup", "close", shared.LuksMapperName}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("closeArgs() = %v, want %v", got, want)
	}
}

// TestUUIDArgs verifies uuid reads the header UUID from the raw partition.
func TestUUIDArgs(t *testing.T) {
	got := uuidArgs("/dev/vda2")
	want := []string{"cryptsetup", "luksUUID", "/dev/vda2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("uuidArgs() = %v, want %v", got, want)
	}
}

// TestIsLuksArgs verifies the isLuks check targets the raw partition.
func TestIsLuksArgs(t *testing.T) {
	got := isLuksArgs("/dev/vda2")
	want := []string{"cryptsetup", "isLuks", "/dev/vda2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("isLuksArgs() = %v, want %v", got, want)
	}
}

// --- functional tests via the injected CommandRunner -------------------------

// TestIsLuks verifies the boolean mapping of cryptsetup's exit status.
func TestIsLuks(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	if !IsLuks("/dev/vda2") {
		t.Error("IsLuks() = false, want true when cryptsetup succeeds")
	}
	calls := rec.CallsFor("cryptsetup")
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], []string{"cryptsetup", "isLuks", "/dev/vda2"}) {
		t.Fatalf("cryptsetup calls = %v, want isLuks /dev/vda2", calls)
	}

	rec.Reset()
	rec.OnError("cryptsetup", errors.New("not a luks device"))
	if IsLuks("/dev/vda2") {
		t.Error("IsLuks() = true, want false when cryptsetup fails")
	}
}

// TestLuksFormat_Success verifies the passphrase is fed via stdin (never argv)
// and the correct luksFormat invocation is built.
func TestLuksFormat_Success(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	const passphrase = "correct horse battery staple"
	if err := LuksFormat(nil, "/dev/vda2", passphrase); err != nil {
		t.Fatalf("LuksFormat() = %v, want nil", err)
	}

	call, ok := rec.LastCall()
	if !ok {
		t.Fatal("no command recorded")
	}
	if call.Stdin != passphrase {
		t.Errorf("stdin = %q, want the passphrase", call.Stdin)
	}
	if call.Args[0] != "cryptsetup" || call.Args[1] != "luksFormat" {
		t.Errorf("argv = %v, want cryptsetup luksFormat ...", call.Args)
	}
	if !reflect.DeepEqual(call.Args, formatArgs("/dev/vda2")) {
		t.Errorf("argv = %v, want %v", call.Args, formatArgs("/dev/vda2"))
	}
	for _, a := range call.Args {
		if a == passphrase {
			t.Errorf("passphrase leaked into argv: %v", call.Args)
		}
	}
}

// TestLuksFormat_Failure verifies command output is folded into the error.
func TestLuksFormat_Failure(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgs([]string{"cryptsetup", "luksFormat"}, primitivestest.Result{
		Output: []byte("device is busy"),
		Err:    errors.New("exit status 1"),
	})
	defer primitives.SetCommandRunner(rec)()

	err := LuksFormat(nil, "/dev/vda2", "pw")
	if err == nil {
		t.Fatal("LuksFormat() = nil, want error")
	}
	if !strings.Contains(err.Error(), "luksFormat") || !strings.Contains(err.Error(), "device is busy") {
		t.Errorf("error = %q, want it to mention luksFormat and the output", err)
	}
}

// TestLuksOpen verifies the mapper is opened with the passphrase on stdin.
func TestLuksOpen(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	if err := LuksOpen("/dev/vda2", shared.LuksMapperName, "pw"); err != nil {
		t.Fatalf("LuksOpen() = %v, want nil", err)
	}
	call, _ := rec.LastCall()
	if call.Stdin != "pw" {
		t.Errorf("stdin = %q, want %q", call.Stdin, "pw")
	}
	if !reflect.DeepEqual(call.Args, openArgs("/dev/vda2", shared.LuksMapperName)) {
		t.Errorf("argv = %v, want %v", call.Args, openArgs("/dev/vda2", shared.LuksMapperName))
	}
}

// TestLuksOpen_Failure verifies a failed open is reported.
func TestLuksOpen_Failure(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("cryptsetup", errors.New("exit status 2"))
	defer primitives.SetCommandRunner(rec)()

	if err := LuksOpen("/dev/vda2", shared.LuksMapperName, "pw"); err == nil {
		t.Fatal("LuksOpen() = nil, want error")
	}
}

// TestLuksClose verifies close targets the mapper.
func TestLuksClose(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	if err := LuksClose(shared.LuksMapperName); err != nil {
		t.Fatalf("LuksClose() = %v, want nil", err)
	}
	calls := rec.CallsFor("cryptsetup")
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], closeArgs(shared.LuksMapperName)) {
		t.Fatalf("argv = %v, want %v", calls, closeArgs(shared.LuksMapperName))
	}

	rec.Reset()
	rec.OnError("cryptsetup", errors.New("exit status 2"))
	if err := LuksClose(shared.LuksMapperName); err == nil {
		t.Fatal("LuksClose() = nil, want error")
	}
}

// TestLuksUUID verifies the header UUID is returned trimmed.
func TestLuksUUID(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgsOutput([]string{"cryptsetup", "luksUUID"}, "aa11bb22\n")
	defer primitives.SetCommandRunner(rec)()

	uuid, err := LuksUUID("/dev/vda2")
	if err != nil {
		t.Fatalf("LuksUUID() = %v, want nil", err)
	}
	if uuid != "aa11bb22" {
		t.Errorf("LuksUUID() = %q, want %q", uuid, "aa11bb22")
	}

	rec.Reset()
	rec.OnError("cryptsetup", errors.New("no header"))
	if _, err := LuksUUID("/dev/vda2"); err == nil {
		t.Fatal("LuksUUID() = nil error, want error")
	}
}

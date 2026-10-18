package users

import (
	"errors"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

func mountedRunner() *primitivestest.Recorder {
	return primitivestest.NewRecorder().OnOutput("findmnt", "/dev/vda2\n")
}

// mountedFS returns an in-memory target with passwd/group files, which Run
// reads to reject collisions before useradd.
func mountedFS() *primitivestest.MemFS {
	return primitivestest.NewMemFS().
		WithFile(shared.MountRoot+"/etc/passwd", "root:x:0:0:root:/root:/bin/bash\n").
		WithFile(shared.MountRoot+"/etc/group", "root:x:0:\nwheel:x:10:\n")
}

func TestRun_Full(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := mountedFS()
	defer primitives.SetFileSystem(fs)()

	ctx := &shared.InstallContext{Username: "alice", Password: []byte("secret")}
	if err := Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	if !hasArgv(rec.CallsFor("arch-chroot"), "arch-chroot", shared.MountRoot, "useradd", "-m", "-G", "wheel", "--", "alice") {
		t.Errorf("user not created: %v", rec.CallsFor("arch-chroot"))
	}
	// The passphrase is fed to chpasswd on stdin, never as an argument.
	for _, c := range rec.Calls() {
		for _, a := range c.Args {
			if strings.Contains(a, "secret") {
				t.Errorf("password leaked into argv: %v", c.Args)
			}
		}
	}
	scrubbed := true
	for _, b := range ctx.Password {
		if b != 0 {
			scrubbed = false
		}
	}
	if !scrubbed {
		t.Error("password was not scrubbed after use")
	}

	const sudoersPath = shared.MountRoot + "/etc/sudoers.d/wheel"
	if got, ok := fs.Read(sudoersPath); !ok || !strings.Contains(got, "%wheel ALL=(ALL:ALL) ALL") {
		t.Errorf("sudoers not written correctly: %q ok=%v", got, ok)
	}
	if got := fs.Mode(sudoersPath); got.Perm() != 0o440 {
		t.Errorf("sudoers mode = %v, want 0440", got.Perm())
	}
}

func TestRun_NoUsername(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()

	if err := Run(&shared.InstallContext{}); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if rec.Called("arch-chroot") {
		t.Error("no commands should run when no username is set")
	}
}

func TestRun_ExistingAccountRejected(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := mountedFS().WithFile(shared.MountRoot+"/etc/passwd", "root:x:0:0:root:/root:/bin/bash\nalice:x:1000:1000::/home/alice:/bin/bash\n")
	defer primitives.SetFileSystem(fs)()

	err := Run(&shared.InstallContext{Username: "alice"})
	if err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("Run() error = %v, want existing-account rejection", err)
	}
	if rec.Called("arch-chroot") {
		t.Error("useradd must not run for an existing account")
	}
}

func TestRun_UseraddFails(t *testing.T) {
	rec := mountedRunner().OnArgs(
		[]string{"arch-chroot", shared.MountRoot, "useradd"},
		primitivestest.Result{Output: []byte("invalid username"), Err: errors.New("exit status 1")},
	)
	defer primitives.SetCommandRunner(rec)()
	defer primitives.SetFileSystem(mountedFS())()

	err := Run(&shared.InstallContext{Username: "alice"})
	if err == nil || !strings.Contains(err.Error(), "failed to setup user") {
		t.Fatalf("Run() error = %v, want useradd failure", err)
	}
}

func TestRun_ChpasswdFails(t *testing.T) {
	rec := mountedRunner().OnArgsError([]string{"arch-chroot", shared.MountRoot, "chpasswd"}, errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()
	defer primitives.SetFileSystem(mountedFS())()

	err := Run(&shared.InstallContext{Username: "alice", Password: []byte("secret")})
	if err == nil || !strings.Contains(err.Error(), "passwd failed") {
		t.Fatalf("Run() error = %v, want chpasswd failure", err)
	}
}

func TestRun_SudoersWriteFails(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := mountedFS().WithWriteError(shared.MountRoot+"/etc/sudoers.d/wheel", errors.New("read-only fs"))
	defer primitives.SetFileSystem(fs)()

	err := Run(&shared.InstallContext{Username: "alice"})
	if err == nil || !strings.Contains(err.Error(), "failed to write sudoers") {
		t.Fatalf("Run() error = %v, want sudoers write failure", err)
	}
}

func TestRun_CopyJobsFail(t *testing.T) {
	rec := mountedRunner().OnError("cp", errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()
	defer primitives.SetFileSystem(mountedFS())()

	err := Run(&shared.InstallContext{Username: "alice"})
	if err == nil || !strings.Contains(err.Error(), "copy job") {
		t.Fatalf("Run() error = %v, want copy-job failure", err)
	}
}

func TestRun_DconfUpdateFailureIsNonFatal(t *testing.T) {
	rec := mountedRunner().OnArgsError([]string{"arch-chroot", shared.MountRoot, "dconf"}, errors.New("dconf: not found"))
	defer primitives.SetCommandRunner(rec)()
	defer primitives.SetFileSystem(mountedFS())()

	// dconf is only a transitive desktop dependency; losing it must not abort
	// the install after both pacstrap steps have completed.
	if err := Run(&shared.InstallContext{Username: "alice", Password: []byte("secret")}); err != nil {
		t.Fatalf("Run() = %v, want nil when dconf update fails", err)
	}
}

// hasArgv reports whether calls contains argv exactly.
func hasArgv(calls [][]string, want ...string) bool {
	for _, argv := range calls {
		if len(argv) != len(want) {
			continue
		}
		match := true
		for i := range want {
			if argv[i] != want[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

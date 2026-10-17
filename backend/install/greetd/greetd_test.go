package greetd

import (
	"errors"
	"strings"
	"testing"
)

func TestConfigToml_Contents(t *testing.T) {
	conf := ConfigToml()
	wants := []string{
		"[terminal]",
		`user = "greeter"`,
		"dms-greeter",
		"--cache-dir /var/cache/dms-greeter",
	}
	for _, want := range wants {
		if !strings.Contains(conf, want) {
			t.Errorf("ConfigToml() missing %q:\n%s", want, conf)
		}
	}
}

func TestMemoryJSON_Placeholder(t *testing.T) {
	if !strings.Contains(MemoryJSON(), "{{USERNAME}}") {
		t.Error("MemoryJSON() should contain the {{USERNAME}} placeholder")
	}
}

// TestConfigureGreetd_Success drives the whole greetd setup through the fake
// executor and checks the externally visible results: the config file, the
// per-user greeter slot, and the substituted memory.json.
func TestConfigureGreetd_Success(t *testing.T) {
	fs := newFakeFS()
	ctx := &InstallContext{Username: "aelum"}

	if err := ConfigureGreetd(fs, ctx); err != nil {
		t.Fatalf("ConfigureGreetd: %v", err)
	}

	if got := fs.files["/etc/greetd/config.toml"]; got != ConfigToml() {
		t.Error("expected /etc/greetd/config.toml written with the embedded template")
	}

	memPath := "/var/cache/dms-greeter/.local/state/memory.json"
	mem, ok := fs.files[memPath]
	if !ok {
		t.Fatalf("memory.json not written to %s", memPath)
	}
	if !strings.Contains(mem, "aelum") || strings.Contains(mem, "{{USERNAME}}") {
		t.Errorf("memory.json should substitute the username, got: %s", mem)
	}

	// SyncDmsGreeter must have created the per-user greeter slot.
	if _, ok := fs.files["/var/cache/dms-greeter/users/aelum/settings.json"]; !ok {
		t.Error("expected the per-user greeter slot to be created")
	}
}

func TestConfigureGreetd_EmptyUsernameSkipsSync(t *testing.T) {
	fs := newFakeFS()

	if err := ConfigureGreetd(fs, &InstallContext{}); err != nil {
		t.Fatalf("ConfigureGreetd: %v", err)
	}
	if _, ok := fs.files["/etc/greetd/config.toml"]; !ok {
		t.Error("config.toml should be written even without a username")
	}
	if _, ok := fs.files["/var/cache/dms-greeter/users/aelum/settings.json"]; ok {
		t.Error("no user slot should be created when the username is empty")
	}
}

// failingFS fails RunChroot on a chosen command to check error propagation.
type failingFS struct {
	*fakeFS
	failOn string
}

func (f *failingFS) RunChroot(args ...string) ([]byte, error) {
	if len(args) > 0 && args[0] == f.failOn {
		return []byte("boom"), errors.New("command failed")
	}
	return f.fakeFS.RunChroot(args...)
}

func TestConfigureGreetd_PropagatesRunError(t *testing.T) {
	fs := &failingFS{fakeFS: newFakeFS(), failOn: "systemd-sysusers"}

	err := ConfigureGreetd(fs, &InstallContext{Username: "aelum"})
	if err == nil {
		t.Fatal("expected an error from the failing executor")
	}
	if !strings.Contains(err.Error(), "systemd-sysusers") {
		t.Errorf("error = %v, want it to mention systemd-sysusers", err)
	}
}

package configure

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"aelum-installer/backend/shared"
)

// checkBashSyntax fails if the content is not valid bash. It catches quoting and
// heredoc mistakes in the embedded initramfs scripts, which only ever run in the
// initramfs and would otherwise go unnoticed until a boot.
func checkBashSyntax(t *testing.T, name, content string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	if out, err := exec.Command("bash", "-n", path).CombinedOutput(); err != nil {
		t.Fatalf("%s is not valid bash: %v\n%s", name, err, out)
	}
}

func TestLuksHintScriptSyntax(t *testing.T) {
	checkBashSyntax(t, "luks-hint.sh", luksHintScript)
}

func TestLuksHintInstallHookSyntax(t *testing.T) {
	hook := fmt.Sprintf(luksHintInstallHook, shared.LuksMapperName)
	checkBashSyntax(t, "aelum-luks-hint", hook)
	if !strings.Contains(hook, "systemd-cryptsetup@"+shared.LuksMapperName+".service.d") {
		t.Errorf("hook does not bake the %q root-instance drop-in:\n%s", shared.LuksMapperName, hook)
	}
}

func TestLuksCryptsetupDropInPath(t *testing.T) {
	want := "/etc/systemd/system/systemd-cryptsetup@" + shared.LuksMapperName + ".service.d/aelum-luks.conf"
	if got := luksCryptsetupDropInPath(); got != want {
		t.Errorf("luksCryptsetupDropInPath() = %q, want %q", got, want)
	}
}

// TestLuksHintWiring pins the three signals the theme depends on: the drop-in
// must announce unlock and pull in the poweroff unit, and the poweroff unit must
// actually power off.
func TestLuksHintWiring(t *testing.T) {
	checks := []struct {
		name    string
		content string
		want    string
	}{
		{"cryptsetup drop-in signals unlock", luksCryptsetupDropIn, "luks:unlocked"},
		{"cryptsetup drop-in fails over to poweroff", luksCryptsetupDropIn, "OnFailure=aelum-luks-poweroff.service"},
		{"poweroff unit powers off", luksPoweroffUnit, "systemctl poweroff"},
		{"retry hint message is set", luksHintScript, "Incorrect passphrase"},
	}
	for _, c := range checks {
		if !strings.Contains(c.content, c.want) {
			t.Errorf("%s: missing %q", c.name, c.want)
		}
	}
}

package installbase

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aelum-installer/backend/shared"
)

// TestAppendBtrfsOnly pins the gate: the extra packages appear only for a btrfs
// install, and neither the caller's slice nor the package list it was handed
// grows.
func TestAppendBtrfsOnly(t *testing.T) {
	packages := []string{"base", "linux-lts"}
	extra := []string{"snapper"}

	ext4 := AppendBtrfsOnly(&shared.InstallContext{FileSystem: shared.FileSystemExt4}, packages, extra)
	if strings.Join(ext4, " ") != "base linux-lts" {
		t.Errorf("ext4 package list = %v, want the list unchanged", ext4)
	}

	none := AppendBtrfsOnly(&shared.InstallContext{}, packages, extra)
	if strings.Join(none, " ") != "base linux-lts" {
		t.Errorf("unset filesystem package list = %v, want the list unchanged", none)
	}

	btrfs := AppendBtrfsOnly(&shared.InstallContext{FileSystem: shared.FileSystemBtrfs}, packages, extra)
	if !contains(btrfs, "snapper") || strings.Join(btrfs, " ") != "base linux-lts snapper" {
		t.Errorf("btrfs package list = %v, want the extras appended", btrfs)
	}

	if strings.Join(packages, " ") != "base linux-lts" {
		t.Errorf("AppendBtrfsOnly grew the slice it was handed: %v", packages)
	}
}

// TestBasePackages_UnchangedAfterBtrfsRun guards the aliasing rule the append
// above depends on: BasePackages is package state shared by every install in the
// process, so a btrfs install must not leave snapper in it for the next one.
func TestBasePackages_UnchangedAfterBtrfsRun(t *testing.T) {
	before := strings.Join(BasePackages, " ")

	AppendBtrfsOnly(&shared.InstallContext{FileSystem: shared.FileSystemBtrfs}, BasePackages, BtrfsOnlyBasePackages)

	if got := strings.Join(BasePackages, " "); got != before {
		t.Errorf("BasePackages changed after a btrfs install:\n%s", got)
	}
}

// pacstrapArgs runs the base step against a fake pacstrap and returns the args
// it was called with.
func pacstrapArgs(t *testing.T, ctx *shared.InstallContext) []string {
	t.Helper()
	dir := fakePacstrap(t, `printf '%s\n' "$@" > "$(dirname "$0")/args"; exit 0`)
	mounted(t)

	if err := Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "args"))
	if err != nil {
		t.Fatalf("read captured args: %v", err)
	}
	return strings.Fields(string(data))
}

func TestRun_BtrfsAddsSnapper(t *testing.T) {
	args := pacstrapArgs(t, &shared.InstallContext{FileSystem: shared.FileSystemBtrfs})
	if !contains(args, "snapper") {
		t.Errorf("pacstrap args for a btrfs install omit snapper: %v", args)
	}
	if !contains(args, "base") {
		t.Errorf("pacstrap args lost the base list: %v", args)
	}
}

func TestRun_Ext4OmitsSnapper(t *testing.T) {
	args := pacstrapArgs(t, &shared.InstallContext{FileSystem: shared.FileSystemExt4})
	if contains(args, "snapper") {
		t.Errorf("pacstrap args for an ext4 install include snapper: %v", args)
	}
	if !contains(args, "base") {
		t.Errorf("pacstrap args lost the base list: %v", args)
	}
}

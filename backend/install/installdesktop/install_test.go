package installdesktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aelum-installer/backend/install/installbase"
	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

func TestDesktopPackages_NotEmpty(t *testing.T) {
	if len(DesktopPackages) == 0 {
		t.Error("expected non-empty DesktopPackages")
	}
}

func TestDesktopPackages_ContainsEssential(t *testing.T) {
	expected := []string{"hyprland", "greetd", "kitty", "firefox", "plymouth", "pipewire"}
	for _, pkg := range expected {
		found := false
		for _, p := range DesktopPackages {
			if p == pkg {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected DesktopPackages to contain %q", pkg)
		}
	}
}

func TestDesktopPackages_NoDuplicatesOrEmpty(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range DesktopPackages {
		if p == "" {
			t.Error("DesktopPackages contains an empty entry")
			continue
		}
		if seen[p] {
			t.Errorf("DesktopPackages contains duplicate %q", p)
		}
		seen[p] = true
	}
}

// TestRun_NotMounted checks the step fails cleanly when the installed system is
// not mounted (the normal state during unit tests).
func TestRun_NotMounted(t *testing.T) {
	if err := Run(&shared.InstallContext{}); err == nil {
		t.Error("expected an error when /mnt is not mounted")
	}
}

// desktopArgs runs the desktop step against a fake pacstrap and returns the args
// it was called with. ctx carries a recorded DriverPackages so the run does not
// depend on the test host's PCI bus; pass none to exercise detection itself.
func desktopArgs(t *testing.T, ctx *shared.InstallContext) []string {
	t.Helper()

	dir := t.TempDir()
	shim := filepath.Join(dir, "pacstrap")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$(dirname \"$0\")/args\"; exit 0\n"
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake pacstrap: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	rec := primitivestest.NewRecorder().OnOutput("findmnt", "/dev/vda2\n")
	defer primitives.SetCommandRunner(rec)()

	if err := Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "args"))
	if err != nil {
		t.Fatalf("read captured args: %v", err)
	}
	return strings.Fields(string(data))
}

func hasPackage(args []string, pkg string) bool {
	for _, a := range args {
		if a == pkg {
			return true
		}
	}
	return false
}

// TestRun_BtrfsAssistantIsGated checks the btrfs-only gate at the step itself:
// the GUI for the subvolume layout ships on btrfs installs and nowhere else.
func TestRun_BtrfsAssistantIsGated(t *testing.T) {
	withDrivers := func(fs string) *shared.InstallContext {
		return &shared.InstallContext{
			FileSystem:     fs,
			DriverPackages: []string{"vulkan-swrast", "lib32-vulkan-swrast", "vulkan-virtio", "lib32-mesa"},
		}
	}

	btrfs := desktopArgs(t, withDrivers(shared.FileSystemBtrfs))
	if !hasPackage(btrfs, "btrfs-assistant") {
		t.Errorf("btrfs desktop install omits btrfs-assistant: %v", btrfs)
	}
	if !hasPackage(btrfs, "hyprland") {
		t.Errorf("btrfs desktop install lost the desktop list: %v", btrfs)
	}

	ext4 := desktopArgs(t, withDrivers(shared.FileSystemExt4))
	if hasPackage(ext4, "btrfs-assistant") {
		t.Errorf("ext4 desktop install includes btrfs-assistant: %v", ext4)
	}
	if !hasPackage(ext4, "hyprland") {
		t.Errorf("ext4 desktop install lost the desktop list: %v", ext4)
	}
}

// TestRun_InstallsTheChosenDriverSet pins the wiring: the set recorded on the
// context (or chosen at the step's start) goes to pacstrap with the desktop
// set, and the old hardcoded vendor trio is gone — it is what made the
// lib32-vulkan-driver provider question possible.
func TestRun_InstallsTheChosenDriverSet(t *testing.T) {
	args := desktopArgs(t, &shared.InstallContext{
		FileSystem:     shared.FileSystemExt4,
		DriverPackages: []string{"vulkan-radeon", "lib32-vulkan-radeon", "lib32-mesa"},
	})
	for _, pkg := range []string{"vulkan-radeon", "lib32-vulkan-radeon", "lib32-mesa"} {
		if !hasPackage(args, pkg) {
			t.Errorf("the chosen driver set is missing %q from the pacstrap args: %v", pkg, args)
		}
	}
	for _, old := range []string{"vulkan-nouveau", "vulkan-intel", "nvidia-utils"} {
		if hasPackage(args, old) {
			t.Errorf("%q is still hardcoded into the desktop set: %v", old, args)
		}
	}
}

// TestRun_DetectsWhenNothingRecorded proves the step itself runs detection:
// with nothing on the context, the FAKE_GPU knob (the same synthetic-data
// pattern as FAKE_DISKS) decides what pacstrap is asked for, and the choice is
// recorded on the context for the verify step.
func TestRun_DetectsWhenNothingRecorded(t *testing.T) {
	t.Setenv("FAKE_GPU", "amd")
	ctx := &shared.InstallContext{FileSystem: shared.FileSystemExt4}
	args := desktopArgs(t, ctx)
	if !hasPackage(args, "vulkan-radeon") || !hasPackage(args, "lib32-vulkan-radeon") {
		t.Errorf("detected set did not reach pacstrap: %v", args)
	}
	if len(ctx.DriverPackages) == 0 {
		t.Error("the desktop step did not record its driver choice on the context")
	}
}

// TestRun_InstallsTheChosenIMEAddons pins the input method wiring the same way
// TestRun_InstallsTheChosenDriverSet pins the drivers: the set recorded on the
// context goes to pacstrap with the desktop set.
func TestRun_InstallsTheChosenIMEAddons(t *testing.T) {
	args := desktopArgs(t, &shared.InstallContext{
		FileSystem: shared.FileSystemExt4,
		Locale:     "en_US.UTF-8",
		IMPackages: []string{"fcitx5-hangul"},
	})
	if !hasPackage(args, "fcitx5-hangul") {
		t.Errorf("the chosen addon set is missing from the pacstrap args: %v", args)
	}
	for _, core := range []string{"fcitx5", "fcitx5-gtk", "fcitx5-qt", "fcitx5-configtool"} {
		if !hasPackage(args, core) {
			t.Errorf("the fcitx5 core %q is missing from the pacstrap args: %v", core, args)
		}
	}
}

// TestRun_ChoosesIMEAddonsFromTheLanguage proves the step itself runs the
// table: with nothing recorded, the chosen language decides what pacstrap is
// asked for, and the choice is recorded on the context.
func TestRun_ChoosesIMEAddonsFromTheLanguage(t *testing.T) {
	ctx := &shared.InstallContext{FileSystem: shared.FileSystemExt4, Locale: "zh_CN.UTF-8"}
	args := desktopArgs(t, ctx)
	if !hasPackage(args, "fcitx5-chinese-addons") {
		t.Errorf("the language's addon set did not reach pacstrap: %v", args)
	}
	if len(ctx.IMPackages) == 0 {
		t.Error("the desktop step did not record its input method choice on the context")
	}
	// A language the keyboard covers gets the core and no addons — and the
	// empty decision is still recorded.
	plain := &shared.InstallContext{FileSystem: shared.FileSystemExt4, Locale: "de_DE.UTF-8"}
	plainArgs := desktopArgs(t, plain)
	for _, addon := range IMCandidatePackages {
		if hasPackage(plainArgs, addon) {
			t.Errorf("a Latin-language install was asked for %q: %v", addon, plainArgs)
		}
	}
	if !hasPackage(plainArgs, "fcitx5") {
		t.Errorf("a Latin-language install lost the fcitx5 core: %v", plainArgs)
	}
	if plain.IMPackages == nil {
		t.Error("the empty input method decision was not recorded on the context")
	}
}

// TestRun_InputMethodNoneAddsNothing checks the override at the step: an
// IME-language install with --input-method none gets the core and no engine.
func TestRun_InputMethodNoneAddsNothing(t *testing.T) {
	ctx := &shared.InstallContext{
		FileSystem:  shared.FileSystemExt4,
		Locale:      "zh_TW.UTF-8",
		InputMethod: InputMethodOverrideNone,
	}
	args := desktopArgs(t, ctx)
	for _, addon := range IMCandidatePackages {
		if hasPackage(args, addon) {
			t.Errorf("--input-method none still asked for %q: %v", addon, args)
		}
	}
	if !hasPackage(args, "fcitx5") {
		t.Errorf("--input-method none dropped the fcitx5 core: %v", args)
	}
}

// TestBtrfsOnlyPackagesAreNotInDesktopPackages keeps btrfs-assistant out of the
// unconditional list: it is added by the gate, and a copy in both places would
// install it on ext4 too.
func TestBtrfsOnlyPackagesAreNotInDesktopPackages(t *testing.T) {
	for _, pkg := range BtrfsOnlyDesktopPackages {
		if hasPackage(DesktopPackages, pkg) {
			t.Errorf("%q is in DesktopPackages and in BtrfsOnlyDesktopPackages", pkg)
		}
	}
	if got := installbase.AppendBtrfsOnly(&shared.InstallContext{FileSystem: shared.FileSystemExt4}, DesktopPackages, BtrfsOnlyDesktopPackages); len(got) != len(DesktopPackages) {
		t.Errorf("ext4 install grew the desktop list by %d", len(got)-len(DesktopPackages))
	}
}

// TestDesktopPackages_CarryNoGPUDrivers keeps the whole driver table out of the
// unconditional list: the set depends on the hardware, and a name in
// DesktopPackages would be installed on every machine regardless.
func TestDesktopPackages_CarryNoGPUDrivers(t *testing.T) {
	for _, pkg := range DriverCandidatePackages {
		if hasPackage(DesktopPackages, pkg) {
			t.Errorf("%q is hardcoded into DesktopPackages; drivers.go must be the only chooser", pkg)
		}
	}
}

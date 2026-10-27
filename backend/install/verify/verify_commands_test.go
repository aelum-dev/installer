package verify

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// TestRun_Mounted_CollectsIssues drives Run with /mnt reported mounted. The host
// has no /mnt/etc/fstab, so the run must collect that issue and fail — covering
// the mounted path end to end.
func TestRun_Mounted_CollectsIssues(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnOutput("findmnt", "/dev/vda2\n").
		OnArgsOutput([]string{"find", shared.BootMountPoint}, "vmlinuz-linux-lts\n")
	defer primitives.SetCommandRunner(rec)()

	err := Run(&shared.InstallContext{DiskID: "/dev/vda", FileSystem: shared.FileSystemExt4})
	if err == nil {
		t.Fatal("Run() = nil, want verification failure")
	}
	if !strings.Contains(err.Error(), "installed system verification failed") {
		t.Errorf("error = %q, want verification failure summary", err)
	}
	if !strings.Contains(err.Error(), "fstab") {
		t.Errorf("error = %q, want it to report the missing fstab", err)
	}
}

// --- checkKernelImages -------------------------------------------------------

func TestCheckKernelImages_Success(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"find", shared.BootMountPoint}, "vmlinuz-linux-lts\n")
	defer primitives.SetCommandRunner(rec)()

	var issues, info []string
	checkKernelImages(&shared.InstallContext{}, &issues, &info)
	if len(issues) != 0 {
		t.Fatalf("issues = %v, want none", issues)
	}
	joined := strings.Join(info, "\n")
	if !strings.Contains(joined, "vmlinuz-linux-lts") || !strings.Contains(joined, "initramfs-linux-lts.img") {
		t.Errorf("info = %v, want kernel and initrd", info)
	}
}

func TestCheckKernelImages_PrefersAelumKernelOnSharedESP(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"find", shared.BootMountPoint}, "vmlinuz-linux\nvmlinuz-linux-lts\n")
	defer primitives.SetCommandRunner(rec)()

	var issues, info []string
	checkKernelImages(&shared.InstallContext{}, &issues, &info)
	if len(issues) != 0 {
		t.Fatalf("issues = %v, want none", issues)
	}
	joined := strings.Join(info, "\n")
	if !strings.Contains(joined, "vmlinuz-linux-lts") || !strings.Contains(joined, "initramfs-linux-lts.img") {
		t.Errorf("info = %v, want the Aelum kernel and its initrd", info)
	}
}

func TestCheckKernelImages_NoKernel(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"find", shared.BootMountPoint}, "")
	defer primitives.SetCommandRunner(rec)()

	var issues, info []string
	checkKernelImages(&shared.InstallContext{}, &issues, &info)
	if len(issues) != 1 || !strings.Contains(issues[0], "no kernel") {
		t.Fatalf("issues = %v, want a no-kernel issue", issues)
	}
}

func TestCheckKernelImages_FindFails(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("find", errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	var issues, info []string
	checkKernelImages(&shared.InstallContext{}, &issues, &info)
	if len(issues) != 1 || !strings.Contains(issues[0], "failed to list kernel files") {
		t.Fatalf("issues = %v, want a list-failure issue", issues)
	}
}

func TestCheckKernelImages_FallbackInitrd(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"find", shared.BootMountPoint}, "vmlinuz-linux\n").
		OnArgsError([]string{"test", "-f", shared.BootMountPoint + "/initramfs-linux.img"}, errors.New("missing"))
	defer primitives.SetCommandRunner(rec)()

	var issues, info []string
	checkKernelImages(&shared.InstallContext{}, &issues, &info)
	if len(issues) != 0 {
		t.Fatalf("issues = %v, want none (fallback initrd exists)", issues)
	}
	if !strings.Contains(strings.Join(info, "\n"), "initramfs-linux-fallback.img") {
		t.Errorf("info = %v, want the fallback initrd recorded", info)
	}
}

func TestCheckKernelImages_NoInitrd(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"find", shared.BootMountPoint}, "vmlinuz-linux\n").
		OnArgsError([]string{"test", "-f"}, errors.New("missing"))
	defer primitives.SetCommandRunner(rec)()

	var issues, info []string
	checkKernelImages(&shared.InstallContext{}, &issues, &info)
	if len(issues) != 1 || !strings.Contains(issues[0], "no initrd") {
		t.Fatalf("issues = %v, want a no-initrd issue", issues)
	}
}

// --- checkPackages -----------------------------------------------------------

func TestCheckPackages_AllPresent(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	var issues, info []string
	checkPackages(&shared.InstallContext{}, &issues, &info)
	if len(issues) != 0 {
		t.Fatalf("issues = %v, want none", issues)
	}
	if got := len(rec.CallsFor("arch-chroot")); got != len(expectedPackages) {
		t.Errorf("queried %d packages, want %d", got, len(expectedPackages))
	}
	if len(info) != 1 {
		t.Errorf("info = %v, want a single summary line", info)
	}
}

func TestCheckPackages_AllMissing(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("arch-chroot", errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	var issues, info []string
	checkPackages(&shared.InstallContext{}, &issues, &info)
	if len(issues) != len(expectedPackages) {
		t.Errorf("issues = %d, want one per missing package (%d)", len(issues), len(expectedPackages))
	}
	if len(info) != 0 {
		t.Errorf("info = %v, want none when packages are missing", info)
	}
}

// TestCheckPackages_QueriesEveryExpectedPackage pins the query shape so the
// check keeps reading the installed system (arch-chroot pacman -Q) rather than
// the live ISO.
func TestCheckPackages_QueriesEveryExpectedPackage(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	var issues, info []string
	checkPackages(&shared.InstallContext{}, &issues, &info)

	seen := map[string]bool{}
	for _, argv := range rec.CallsFor("arch-chroot") {
		want := []string{"arch-chroot", shared.MountRoot, "pacman", "-Q"}
		if len(argv) < 5 || !reflect.DeepEqual(argv[:4], want) {
			t.Fatalf("unexpected package query argv: %v", argv)
		}
		seen[argv[4]] = true
	}
	for _, pkg := range expectedPackages {
		if !seen[pkg] {
			t.Errorf("package %q was never queried", pkg)
		}
	}
}

// queriedPackages runs checkPackages against a recorder and returns the package
// names it asked the installed system about.
func queriedPackages(t *testing.T, ctx *shared.InstallContext) map[string]bool {
	t.Helper()

	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	var issues, info []string
	checkPackages(ctx, &issues, &info)

	seen := map[string]bool{}
	for _, argv := range rec.CallsFor("arch-chroot") {
		if len(argv) > 4 {
			seen[argv[4]] = true
		}
	}
	return seen
}

// TestCheckPackages_BtrfsOnlyPackagesAreGated keeps the verification list in step
// with the install steps: the snapshot tooling is verified on a btrfs install and
// never on an ext4 one.
func TestCheckPackages_BtrfsOnlyPackagesAreGated(t *testing.T) {
	if len(btrfsOnlyPackages) == 0 {
		t.Fatal("expected a non-empty btrfsOnlyPackages list")
	}

	btrfs := queriedPackages(t, &shared.InstallContext{FileSystem: shared.FileSystemBtrfs})
	ext4 := queriedPackages(t, &shared.InstallContext{FileSystem: shared.FileSystemExt4})

	for _, pkg := range btrfsOnlyPackages {
		if !btrfs[pkg] {
			t.Errorf("a btrfs install did not verify %q", pkg)
		}
		if ext4[pkg] {
			t.Errorf("an ext4 install verified %q, which it never installs", pkg)
		}
	}

	for _, pkg := range expectedPackages {
		if !btrfs[pkg] || !ext4[pkg] {
			t.Errorf("%q must be verified on every filesystem", pkg)
		}
	}
}

// TestCheckPackages_SummaryCountsWhatItChecked pins the log line to the list the
// check actually walked, so a btrfs install reports its snapshot packages too
// instead of the unconditional count.
func TestCheckPackages_SummaryCountsWhatItChecked(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	var issues, info []string
	checkPackages(&shared.InstallContext{FileSystem: shared.FileSystemBtrfs}, &issues, &info)
	if len(issues) != 0 {
		t.Fatalf("issues = %v, want none", issues)
	}

	want := fmt.Sprintf("all %d packages", len(expectedPackages)+len(btrfsOnlyPackages))
	if len(info) != 1 || !strings.Contains(info[0], want) {
		t.Errorf("info = %v, want a summary counting %d packages", info, len(expectedPackages)+len(btrfsOnlyPackages))
	}
}

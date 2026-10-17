package verify

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"aelum-installer/backend/install/installbase"
	"aelum-installer/backend/install/installdesktop"
	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// expectedPackages is the full set of packages installed by the base and
// desktop steps. Verification checks each one against the target so the
// installed system is confirmed to have everything needed to boot and launch
// the desktop. Single-sourced from the install steps (not duplicated here).
var expectedPackages = append(append([]string{}, installbase.BasePackages...), installdesktop.DesktopPackages...)

// Run verifies that the installed system (mounted at /mnt) is complete and
// bootable. The check is read-only and fast: it verifies fstab, kernel/initrd
// images, required packages, the user account, and the hostname. It never
// modifies the installed system.
//
// On failure it returns an error listing every problem found, and logs the
// collected diagnostics to help locate the root cause.
func Run(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Verifying installed system...")
	}

	var issues []string
	var info []string

	checkFstab(ctx, &issues, &info)
	if ctx.Luks {
		checkLuks(ctx, &issues, &info)
	}
	if ctx.Lvm {
		checkLvm(ctx, &issues, &info)
	}
	checkKernelImages(ctx, &issues, &info)
	checkPackages(ctx, &issues, &info)
	checkUserAccount(ctx, &issues, &info)
	checkHostname(ctx, &issues, &info)

	// Emit collected diagnostics before deciding success/failure.
	for _, line := range info {
		if ctx.Logger != nil {
			ctx.Logger.Info("%s", capitalizeFirst(line))
		}
	}

	if len(issues) > 0 {
		if ctx.Logger != nil {
			ctx.Logger.Error("Installed system verification failed (%d issue(s)):", len(issues))
			for _, issue := range issues {
				ctx.Logger.Error("  - %s", issue)
			}
		}
		return fmt.Errorf("installed system verification failed:\n  - %s", strings.Join(issues, "\n  - "))
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Installed system verification passed")
	}
	return nil
}

// capitalizeFirst returns s with its first rune upper-cased. It is a no-op
// for empty strings. Applied to each diagnostic info line so logs start with a
// capital letter (e.g. "kernel image:" -> "Kernel image:").
func capitalizeFirst(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// checkFstab verifies that /etc/fstab exists, is non-empty, and (when the
// partition UUIDs are known) lists the root and EFI partitions.
func checkFstab(ctx *shared.InstallContext, issues, info *[]string) {
	path := shared.MountRoot + "/etc/fstab"
	data, err := primitives.ReadFile(path)
	if err != nil {
		*issues = append(*issues, "fstab not found or unreadable at "+path)
		return
	}
	content := string(data)
	if strings.TrimSpace(content) == "" {
		*issues = append(*issues, "fstab is empty")
		return
	}
	*info = append(*info, "fstab contents:\n"+content)

	if ctx.Partitions != nil {
		if ctx.Partitions.RootUUID != "" && !strings.Contains(content, ctx.Partitions.RootUUID) {
			*issues = append(*issues, "fstab missing root UUID "+ctx.Partitions.RootUUID)
		}
		if ctx.Partitions.EFIUUID != "" && !strings.Contains(content, ctx.Partitions.EFIUUID) {
			*issues = append(*issues, "fstab missing EFI UUID "+ctx.Partitions.EFIUUID)
		}
	}
}

// checkLuks verifies LUKS-related configuration in the installed system when
// encryption is enabled: the boot entry maps the LUKS header to the "aelum"
// mapper via rd.luks.name=, the mapper UUID is in fstab, and the dm-crypt hook
// is present in mkinitcpio.
func checkLuks(ctx *shared.InstallContext, issues, info *[]string) {
	// The boot entry maps the header to the mapper with
	// rd.luks.name=<header-uuid>=<mapper>, consumed by sd-encrypt. The root is
	// then referenced indirectly: root=UUID=<mapper fs uuid> when root is the
	// mapper, or root=/dev/mapper/aelum-root when LVM sits on top. Assert the
	// mapping argument itself rather than a literal /dev/mapper path, which
	// only the LVM root happens to contain as a substring.
	entryPath := shared.BootMountPoint + "/loader/entries/" + shared.BootEntryFileName + ".conf"
	if data, err := primitives.ReadFile(entryPath); err == nil {
		content := string(data)
		switch {
		case !luksEntryMapsMapper(content, ctx.Partitions.LuksUUID):
			if strings.Contains(content, "rd.luks.name=") {
				*issues = append(*issues, "boot entry LUKS mapping does not target "+shared.LuksMapperName)
			} else {
				*issues = append(*issues, "boot entry missing rd.luks.name= for LUKS")
			}
		default:
			*info = append(*info, "boot entry maps LUKS header to "+shared.LuksMapperName+" ✓")
		}
	} else {
		*issues = append(*issues, "failed to read boot entry "+entryPath)
	}

	// Mapper UUID present in fstab (RootUUID is the mapper filesystem UUID).
	// checkFstab already verifies this; surface it for visibility.
	if ctx.Partitions != nil && ctx.Partitions.RootUUID != "" {
		*info = append(*info, "mapper UUID in fstab ✓")
	}

	// dm-crypt hook present in mkinitcpio.conf.
	confPath := shared.MountRoot + "/etc/mkinitcpio.conf"
	if data, err := primitives.ReadFile(confPath); err == nil {
		content := string(data)
		if !strings.Contains(content, "sd-encrypt") {
			*issues = append(*issues, "dm-crypt hook (sd-encrypt) not found in mkinitcpio.conf")
		} else {
			*info = append(*info, "initramfs dm-crypt hook present ✓")
		}
	} else {
		*issues = append(*issues, "failed to read "+confPath)
	}
}

// luksEntryMapsMapper reports whether a boot entry's options map the LUKS header
// to the aelum mapper via rd.luks.name=<header-uuid>=<mapper>, as consumed by
// sd-encrypt. When luksUUID is empty the exact header cannot be asserted, so any
// rd.luks.name= mapping is accepted. The literal /dev/mapper path is deliberately
// not required: a LUKS root selected by UUID (root=UUID=…) never contains it, so
// requiring it produced a false failure on every non-LVM encrypted install.
func luksEntryMapsMapper(options, luksUUID string) bool {
	if !strings.Contains(options, "rd.luks.name=") {
		return false
	}
	if luksUUID == "" {
		return true
	}
	return strings.Contains(options, "rd.luks.name="+luksUUID+"="+shared.LuksMapperName)
}

// checkLvm verifies LVM-related configuration in the installed system when LVM
// is enabled: the boot entry points root at the root logical volume, fstab lists
// the home LV, and the lvm2 hook is present in mkinitcpio.
func checkLvm(ctx *shared.InstallContext, issues, info *[]string) {
	// Boot entry references the root logical volume (not the PV).
	entryPath := shared.BootMountPoint + "/loader/entries/" + shared.BootEntryFileName + ".conf"
	if data, err := primitives.ReadFile(entryPath); err == nil {
		content := string(data)
		if !strings.Contains(content, shared.LvmRootLvPath) {
			*issues = append(*issues, "boot entry does not reference the root LV "+shared.LvmRootLvPath)
		} else {
			*info = append(*info, "boot entry references root LV ✓")
		}
	} else {
		*issues = append(*issues, "failed to read boot entry "+entryPath)
	}

	// Home LV filesystem UUID present in fstab (RootUUID is the root LV's UUID,
	// already covered by checkFstab).
	if ctx.Partitions != nil && ctx.Partitions.HomeUUID != "" {
		path := shared.MountRoot + "/etc/fstab"
		if data, err := primitives.ReadFile(path); err == nil {
			if !strings.Contains(string(data), ctx.Partitions.HomeUUID) {
				*issues = append(*issues, "fstab missing home LV UUID "+ctx.Partitions.HomeUUID)
			} else {
				*info = append(*info, "home LV UUID in fstab ✓")
			}
		}
	}

	// lvm2 hook present in mkinitcpio.conf.
	confPath := shared.MountRoot + "/etc/mkinitcpio.conf"
	if data, err := primitives.ReadFile(confPath); err == nil {
		content := string(data)
		if !strings.Contains(content, shared.LvmInitramfsHook) {
			*issues = append(*issues, "lvm2 hook not found in mkinitcpio.conf")
		} else {
			*info = append(*info, "initramfs lvm2 hook present ✓")
		}
	} else {
		*issues = append(*issues, "failed to read "+confPath)
	}
}

// checkKernelImages verifies that a kernel image and its matching initrd exist
// on the ESP mount, which is required for the system to boot.
func checkKernelImages(ctx *shared.InstallContext, issues, info *[]string) {
	cmd := primitives.Run("find", shared.BootMountPoint, "-maxdepth", "1", "-name", shared.KernelFilePrefix+"*")
	output, err := cmd.Output()
	if err != nil {
		*issues = append(*issues, "failed to list kernel files in "+shared.BootMountPoint)
		return
	}
	files := strings.TrimSpace(string(output))
	if files == "" {
		*issues = append(*issues, "no kernel (vmlinuz-*) image found in "+shared.BootMountPoint)
		return
	}
	kernel := filepath.Base(strings.Split(files, "\n")[0])
	*info = append(*info, "kernel image: "+kernel)

	// Verify the matching initrd exists (try the standard name, then -fallback).
	base := strings.TrimPrefix(kernel, shared.KernelFilePrefix)
	initrd := shared.InitrdFilePrefix + base + shared.InitrdFileExtension
	if err := primitives.Run("test", "-f", shared.BootMountPoint+"/"+initrd).Run(); err != nil {
		fallback := shared.InitrdFilePrefix + base + "-fallback" + shared.InitrdFileExtension
		if err := primitives.Run("test", "-f", shared.BootMountPoint+"/"+fallback).Run(); err != nil {
			*issues = append(*issues, "no initrd found for kernel "+kernel)
			return
		}
		initrd = fallback
	}
	*info = append(*info, "initrd image: "+initrd)
}

// checkPackages verifies that every package installed by the base and desktop
// steps is present in the installed system. Each package is queried individually
// via `pacman -Q <pkg>` (not a full listing) so the check stays fast. Only
// stdout is captured: pacman prints the query result there and the
// "database file for 'core' does not exist" warnings to stderr. Capturing
// stdout alone keeps the logs clean of those warnings (the repos' sync
// databases are never created in an offline install) while a missing package
// still surfaces as a non-zero exit code.
//
// On success it logs a single summary line; on failure it lists every missing
// package as an issue.
func checkPackages(ctx *shared.InstallContext, issues, info *[]string) {
	var missing []string
	for _, pkg := range expectedPackages {
		if _, err := primitives.Run("arch-chroot", shared.MountRoot, "pacman", "-Q", pkg).Output(); err != nil {
			missing = append(missing, pkg)
		}
	}

	if len(missing) > 0 {
		for _, pkg := range missing {
			*issues = append(*issues, "required package "+pkg+" is not installed")
		}
		return
	}

	*info = append(*info, fmt.Sprintf("all %d packages installed and present ✓ (base + desktop)", len(expectedPackages)))
}

// checkUserAccount verifies that the configured user account exists in the
// installed system's /etc/passwd.
func checkUserAccount(ctx *shared.InstallContext, issues, info *[]string) {
	if ctx.Username == "" {
		return
	}
	path := shared.MountRoot + "/etc/passwd"
	data, err := primitives.ReadFile(path)
	if err != nil {
		*issues = append(*issues, "failed to read "+path)
		return
	}
	found := false
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) > 0 && fields[0] == ctx.Username {
			found = true
			break
		}
	}
	if !found {
		*issues = append(*issues, "user account "+ctx.Username+" not found in "+path)
		return
	}
	*info = append(*info, "user account present: "+ctx.Username)
}

// checkHostname verifies that the configured hostname is set in the installed
// system's /etc/hostname.
func checkHostname(ctx *shared.InstallContext, issues, info *[]string) {
	if ctx.Hostname == "" {
		return
	}
	path := shared.MountRoot + "/etc/hostname"
	data, err := primitives.ReadFile(path)
	if err != nil {
		*issues = append(*issues, "failed to read "+path)
		return
	}
	got := strings.TrimSpace(string(data))
	if got != ctx.Hostname {
		*issues = append(*issues, "hostname mismatch: expected "+ctx.Hostname+", got "+got)
		return
	}
	*info = append(*info, "hostname set: "+ctx.Hostname)
}

package verify

import (
	"fmt"
	"strings"
	"unicode"

	"aelum-installer/backend/install/installbase"
	"aelum-installer/backend/install/installdesktop"
	"aelum-installer/backend/install/lvm"
	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// expectedPackages is the full set of packages installed by the base and
// desktop steps. Verification checks each one against the target so the
// installed system is confirmed to have everything needed to boot and launch
// the desktop. Single-sourced from the install steps (not duplicated here).
var expectedPackages = append(append([]string{}, installbase.BasePackages...), installdesktop.DesktopPackages...)

// btrfsOnlyPackages are the packages the base and desktop steps install only for
// a btrfs install (installbase.AppendBtrfsOnly gates them). Expecting them on an
// ext4 install would report a package that was never meant to be there.
var btrfsOnlyPackages = append(append([]string{}, installbase.BtrfsOnlyBasePackages...), installdesktop.BtrfsOnlyDesktopPackages...)

// expectedPackagesFor returns the package set this install must contain.
func expectedPackagesFor(ctx *shared.InstallContext) []string {
	if ctx.FileSystem != shared.FileSystemBtrfs {
		return expectedPackages
	}
	return append(append([]string{}, expectedPackages...), btrfsOnlyPackages...)
}

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
	checkBootloader(ctx, &issues, &info)
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
// partition UUIDs are known) lists the root and EFI partitions plus this
// installation's /boot bind mount on the ESP.
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
		if ctx.Partitions.RootUUID != "" {
			id := shared.InstallID(ctx.Partitions.RootUUID)
			bootSrc := shared.InstalledESPMountPoint + "/" + shared.InstallBootDir(id)
			if !strings.Contains(content, bootSrc) {
				*issues = append(*issues, "fstab missing this installation's boot directory "+bootSrc)
			}
			if !strings.Contains(content, "bind") {
				*issues = append(*issues, "fstab missing the /boot bind mount")
			}
		}
		if ctx.Partitions.HomeUUID != "" && !strings.Contains(content, ctx.Partitions.HomeUUID) {
			*issues = append(*issues, "fstab missing the home LV UUID "+ctx.Partitions.HomeUUID)
		}
	}
}

// espRoot returns the ESP mount point for this installation. The ESP itself is
// mounted at ESPMountPoint; BootMountPoint is only this install's bind-mounted
// kernel directory.
func espRoot(ctx *shared.InstallContext) string {
	if ctx != nil && ctx.ESP != "" {
		return ctx.ESP
	}
	return shared.ESPMountPoint
}

// bootEntryPath returns the host path of the rEFInd menu fragment belonging to
// this installation. Each install has its own fragment, keyed by its root
// filesystem UUID.
func bootEntryPath(ctx *shared.InstallContext) string {
	if ctx == nil || ctx.Partitions == nil {
		return espRoot(ctx) + "/" + shared.RefindConfigPath
	}
	return espRoot(ctx) + "/" + shared.RefindEntryPath(shared.InstallID(ctx.Partitions.RootUUID))
}

// checkLuks verifies LUKS-related configuration in the installed system when
// encryption is enabled: the rEFInd stanza maps the LUKS header to the "aelum"
// mapper via rd.luks.name=, the mapper UUID is in fstab, and the dm-crypt hook
// is present in mkinitcpio.
func checkLuks(ctx *shared.InstallContext, issues, info *[]string) {
	// The rEFInd stanza maps the header to the mapper with
	// rd.luks.name=<header-uuid>=<mapper>, consumed by sd-encrypt. The root is
	// then referenced indirectly (root=UUID=<mapper fs uuid>). Assert the
	// mapping argument itself rather than a literal /dev/mapper path, which the
	// boot entry does not contain.
	confPath := bootEntryPath(ctx)
	if data, err := primitives.ReadFile(confPath); err == nil {
		content := string(data)
		switch {
		case !luksEntryMapsMapper(content, ctx.Partitions.LuksUUID):
			if strings.Contains(content, "rd.luks.name=") {
				*issues = append(*issues, "rEFInd config LUKS mapping does not target "+shared.LuksMapperName)
			} else {
				*issues = append(*issues, "rEFInd config missing rd.luks.name= for LUKS")
			}
		default:
			*info = append(*info, "rEFInd config maps LUKS header to "+shared.LuksMapperName+" ✓")
		}
	} else {
		*issues = append(*issues, "failed to read rEFInd config "+confPath)
	}

	// Mapper UUID present in fstab (RootUUID is the mapper filesystem UUID).
	// checkFstab already verifies this; surface it for visibility.
	if ctx.Partitions != nil && ctx.Partitions.RootUUID != "" {
		*info = append(*info, "mapper UUID in fstab ✓")
	}

	// dm-crypt hook present in mkinitcpio.conf.
	confPath = shared.MountRoot + "/etc/mkinitcpio.conf"
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
// requiring it produced a false failure on every encrypted install.
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
// is enabled: the volume group and its logical volumes exist, the root LV's
// filesystem UUID is the one fstab and the boot entry use, the lvm2 hook is in
// mkinitcpio.conf, and the boot entry never references a device-mapper path.
func checkLvm(ctx *shared.InstallContext, issues, info *[]string) {
	if ctx.Partitions == nil || ctx.Partitions.VGName == "" {
		*issues = append(*issues, "LVM enabled but no volume group was recorded")
		return
	}
	vg := ctx.Partitions.VGName

	if out, err := primitives.Run("vgs", "--noheadings", "-o", "vg_name", vg).Output(); err != nil {
		*issues = append(*issues, "volume group "+vg+" not found")
	} else if strings.TrimSpace(string(out)) == "" {
		*issues = append(*issues, "volume group "+vg+" not reported by vgs")
	} else {
		*info = append(*info, "volume group "+vg+" present ✓")
	}

	expected := []string{shared.LvmRootVolumeName}
	if ctx.Partitions.HomeDevice != "" {
		expected = append(expected, shared.LvmHomeVolumeName)
	}
	if out, err := primitives.Run("lvs", "--noheadings", "-o", "lv_name", vg).Output(); err != nil {
		*issues = append(*issues, "cannot list the logical volumes of "+vg)
	} else {
		for _, lv := range expected {
			if !lvListContains(string(out), lv) {
				*issues = append(*issues, "logical volume "+lv+" missing from "+vg)
			} else {
				*info = append(*info, "logical volume "+lv+" present ✓")
			}
		}
	}

	// The root LV must carry the filesystem UUID fstab and the boot entry use.
	if ctx.Partitions.RootUUID != "" {
		rootLV := lvm.Path(vg, shared.LvmRootVolumeName)
		if out, err := primitives.Run("blkid", "-s", "UUID", "-o", "value", rootLV).Output(); err != nil {
			*issues = append(*issues, "cannot read the root LV filesystem UUID from "+rootLV)
		} else if got := strings.TrimSpace(string(out)); got != ctx.Partitions.RootUUID {
			*issues = append(*issues, "root LV filesystem UUID is "+got+", expected "+ctx.Partitions.RootUUID)
		} else {
			*info = append(*info, "root LV filesystem UUID matches fstab ✓")
		}
	}

	confPath := shared.MountRoot + "/etc/mkinitcpio.conf"
	if data, err := primitives.ReadFile(confPath); err == nil {
		if !strings.Contains(string(data), shared.LvmInitramfsHook) {
			*issues = append(*issues, "lvm2 hook not found in mkinitcpio.conf")
		} else {
			*info = append(*info, "initramfs lvm2 hook present ✓")
		}
	} else {
		*issues = append(*issues, "failed to read "+confPath)
	}

	entryPath := bootEntryPath(ctx)
	if data, err := primitives.ReadFile(entryPath); err == nil {
		content := string(data)
		if ctx.Partitions.RootUUID != "" && !strings.Contains(content, "root=UUID="+ctx.Partitions.RootUUID) {
			*issues = append(*issues, "rEFInd entry does not select the root by UUID (root=UUID="+ctx.Partitions.RootUUID+")")
		} else {
			*info = append(*info, "rEFInd entry selects the root LV by UUID ✓")
		}
		if strings.Contains(content, "/dev/mapper/"+vg) {
			*issues = append(*issues, "rEFInd entry references a device-mapper path instead of a filesystem UUID")
		}
	} else {
		*issues = append(*issues, "failed to read rEFInd config "+entryPath)
	}
}

// lvListContains reports whether lvs output lists the given logical volume.
func lvListContains(output, name string) bool {
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == name {
			return true
		}
	}
	return false
}

// checkBootloader verifies that rEFInd's EFI binary and configuration (with the
// Aelum menu entry) are present on the ESP. The stanza's root/LUKS options are
// checked by checkLuks when encryption is enabled.
func checkBootloader(ctx *shared.InstallContext, issues, info *[]string) {
	esp := espRoot(ctx)
	binaryPath := esp + "/" + shared.RefindBinaryPath
	if err := primitives.Run("test", "-f", binaryPath).Run(); err != nil {
		*issues = append(*issues, "rEFInd binary not found at "+binaryPath)
	} else {
		*info = append(*info, "rEFInd binary present ✓")
	}

	confPath := esp + "/" + shared.RefindConfigPath
	data, err := primitives.ReadFile(confPath)
	if err != nil {
		*issues = append(*issues, "rEFInd config not found or unreadable at "+confPath)
		return
	}
	id := ""
	if ctx.Partitions != nil {
		id = shared.InstallID(ctx.Partitions.RootUUID)
	}
	if id == "" || !strings.Contains(string(data), "include "+shared.RefindEntryInclude(id)) {
		*issues = append(*issues, "rEFInd config is missing the include for this installation's menu entry")
		return
	}
	if !strings.Contains(string(data), "include "+shared.RefindThemeInclude) {
		*issues = append(*issues, "rEFInd config is missing the Aelum theme include")
		return
	}
	*info = append(*info, "rEFInd config present ✓")

	entryPath := bootEntryPath(ctx)
	entryData, err := primitives.ReadFile(entryPath)
	if err != nil {
		*issues = append(*issues, "rEFInd menu entry not found at "+entryPath)
		return
	}
	title := shared.BootEntryTitle
	if ctx.Partitions != nil {
		title = shared.RefindEntryTitle(ctx.Partitions.RootPart)
	}
	if !strings.Contains(string(entryData), fmt.Sprintf("menuentry %q", title)) {
		*issues = append(*issues, "rEFInd menu entry is missing the "+title+" stanza")
		return
	}
	if ctx.Partitions != nil {
		kernelDir := "/" + shared.InstallBootDir(shared.InstallID(ctx.Partitions.RootUUID)) + "/"
		if !strings.Contains(string(entryData), "loader "+kernelDir) {
			*issues = append(*issues, "rEFInd menu entry does not load a kernel from this installation's boot directory ("+kernelDir+")")
			return
		}
	}
	*info = append(*info, "rEFInd menu entry present ✓")

	themeConfPath := esp + "/" + shared.RefindThemeConfigPath
	if _, err := primitives.ReadFile(themeConfPath); err != nil {
		*issues = append(*issues, "Aelum rEFInd theme not found at "+themeConfPath)
		return
	}
	*info = append(*info, "Aelum rEFInd theme present ✓")
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
	// Match the bootloader step's choice: prefer the kernel the installer
	// installed, so a second kernel image cannot verify something the boot
	// entry does not reference.
	kernel := shared.PickKernelFile(strings.Split(files, "\n"))
	if kernel == "" {
		*issues = append(*issues, "no kernel (vmlinuz-*) image found in "+shared.BootMountPoint)
		return
	}
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
	expected := expectedPackagesFor(ctx)
	var missing []string
	for _, pkg := range expected {
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

	*info = append(*info, fmt.Sprintf("all %d packages installed and present ✓ (base + desktop)", len(expected)))
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

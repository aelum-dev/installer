package mount

import (
	"fmt"
	"strings"

	"aelum-installer/backend/install/luks"
	"aelum-installer/backend/install/lvm"
	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// Run mounts the root partition, any btrfs subvolumes, and the EFI partition.
//
// For standard layouts (ext4/default):
//   - /mnt: root partition
//   - /mnt/efi: EFI partition (ESP; rEFInd and per-install boot dirs live here)
//   - /mnt/boot: bind mount of <ESP>/aelum/<id> (this install's kernel/initrd)
//
// For btrfs subvolume layouts:
//   - /mnt: subvolume with MountPoint "/" (e.g. @), with properties from layout
//   - /mnt/<mountPoint>: each additional subvolume, with properties from layout
//   - /mnt/efi: EFI partition (ESP)
//   - /mnt/boot: bind mount of this installation's ESP boot directory
//
// For ext4 layouts:
//   - /mnt: root partition (or the root logical volume when LVM is enabled)
//   - /mnt/home: home logical volume (LVM only)
//   - /mnt/efi: EFI partition (ESP)
//   - /mnt/boot: bind mount of this installation's ESP boot directory
//
// If ctx.Partitions is nil (standalone invocation), it derives partition paths
// from the disk ID. This matches the layout created by format.Run.
func Run(ctx *shared.InstallContext) error {
	// Validate preconditions
	if ctx.DiskID == "" {
		return fmt.Errorf("disk ID not set. Run --format-disk <disk> first")
	}
	if ctx.Partitions == nil {
		return fmt.Errorf("partitions not initialized. Run --format-disk <disk> first")
	}
	if ctx.FileSystem == "" {
		return fmt.Errorf("filesystem not set. Run --format-disk <disk> first")
	}

	// Validate that partitions exist (from FormatDisk) or can be derived
	if ctx.Partitions.EFIPart == "" || ctx.Partitions.RootPart == "" {
		return fmt.Errorf("partition paths not set. FormatDisk may have failed")
	}
	if ctx.Partitions.RootUUID == "" {
		return fmt.Errorf("root UUID not set, cannot name this installation's boot directory. FormatDisk may have failed")
	}

	p := ctx.Partitions
	var cmd *primitives.Command
	var output []byte
	var err error

	// Resolve the device that holds the root filesystem (and its subvolumes):
	//   LUKS : open LUKS, root lives on the decrypted mapper
	//   LVM  : activate the volume group, root lives on its root LV
	//   else : root lives on the raw partition
	rootDevice := p.RootPart
	if ctx.Luks {
		if p.RootMapper == "" {
			return fmt.Errorf("LUKS enabled but root mapper path not set (format step failed)")
		}
		if ctx.Logger != nil {
			ctx.Logger.Info("LUKS root enabled; mounting decrypted mapper %s", p.RootMapper)
		}
		if err := ensureLuksOpen(ctx, p.RootPart); err != nil {
			return err
		}
		rootDevice = p.RootMapper
	}
	if p.VGName != "" {
		// LUKS sits below LVM, so the group is activated after the mapper is
		// open: its physical volume is the decrypted device.
		if err := lvm.Activate(p.VGName); err != nil {
			return err
		}
		rootDevice = lvm.Path(p.VGName, shared.LvmRootVolumeName)
		if ctx.Logger != nil {
			ctx.Logger.Info("LVM root enabled; mounting volume group %s root at %s", p.VGName, rootDevice)
		}
	}

	// ── Mount root partition ──

	// Find the subvolume with MountPoint "/" for btrfs, otherwise mount raw.
	var rootSubvol *shared.BtrfsSubvolume
	if ctx.FileSystem == shared.FileSystemBtrfs {
		rootSubvol = shared.FindSubvolumeByMountPoint(shared.GetBtrfsSubvolumes(), "/")
	}

	if rootSubvol != nil {
		// Mount btrfs subvolume with its properties (e.g. subvol=@,compress=zstd).
		// --mkdir ensures the mount point exists even on a bare live ISO rootfs.
		mountOpts := primitives.BuildMountOpts(rootSubvol)
		cmd = primitives.Run("mount", "--mkdir", "-o", mountOpts, rootDevice, shared.MountRoot)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("mount btrfs %s subvolume failed: %w: %s", rootSubvol.Name, err, string(output))
		}
	} else {
		// Standard mount for ext4 and other filesystems. --mkdir creates the mount
		// point (e.g. /mnt) if the live ISO rootfs doesn't already provide it.
		cmd = primitives.Run("mount", "--mkdir", rootDevice, shared.MountRoot)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("mount root failed: %w: %s", err, string(output))
		}
	}

	// Verify root mount
	if err := verifyMount(shared.MountRoot, rootDevice); err != nil {
		return err
	}

	// ── Mount additional filesystems (btrfs subvolumes) ──
	if err := mountAdditionalMounts(ctx, p, rootDevice); err != nil {
		return err
	}

	// ── Mount EFI partition and this install's boot directory ──
	//
	// The ESP is mounted at /mnt/efi so rEFInd and every installation's kernel
	// directory share one filesystem. This install's directory
	// (aelum/<id>/ on the ESP) is bind-mounted at /mnt/boot, which is where
	// pacstrap and mkinitcpio expect the kernel and initramfs. Two installs on
	// the same ESP therefore never write to the same kernel files.

	espMount := shared.ESPMountPoint
	if err := primitives.Run("mkdir", "-p", espMount).Run(); err != nil {
		return fmt.Errorf("create ESP mount point %s failed: %w", espMount, err)
	}

	// The filesystem type is given explicitly: without it the kernel probes
	// every filesystem it knows, and a FAT partition can be rejected when the
	// vfat module has not been loaded yet (the probe then fails with a
	// confusing "Can't find ext4 filesystem"/squashfs error). Naming the type
	// makes the kernel load the module on demand.
	cmd = primitives.Run("mount", "-t", shared.FileSystemVfat, p.EFIPart, espMount)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount EFI failed: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	if err := verifyMount(espMount, p.EFIPart); err != nil {
		return err
	}

	bootDir := espMount + "/" + shared.InstallBootDir(shared.InstallID(p.RootUUID))
	if err := primitives.Run("mkdir", "-p", bootDir).Run(); err != nil {
		return fmt.Errorf("create per-install boot directory %s failed: %w", bootDir, err)
	}
	if err := primitives.Run("mkdir", "-p", shared.BootMountPoint).Run(); err != nil {
		return fmt.Errorf("create boot mount point %s failed: %w", shared.BootMountPoint, err)
	}
	cmd = primitives.Run("mount", "--bind", bootDir, shared.BootMountPoint)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bind per-install boot directory failed: %w: %s", err, string(output))
	}
	if err := verifyBindMount(bootDir, shared.BootMountPoint); err != nil {
		return err
	}

	return nil
}

// mountAdditionalMounts mounts every filesystem beyond root that the layout
// defines: the btrfs subvolumes, or the ext4 home logical volume.
func mountAdditionalMounts(ctx *shared.InstallContext, p *shared.PartitionInfo, rootDevice string) error {
	if ctx.FileSystem == shared.FileSystemBtrfs {
		return mountBtrfsSubvolumes(ctx, p, rootDevice)
	}
	if p.HomeDevice != "" {
		return mountHomeVolume(ctx, p.HomeDevice)
	}
	return nil
}

// mountHomeVolume mounts the ext4 home logical volume at /mnt/home, so pacstrap
// and the user setup write into it instead of the root filesystem.
func mountHomeVolume(ctx *shared.InstallContext, device string) error {
	fullPath := shared.MountRoot + "/home"
	if err := primitives.Run("mkdir", "-p", fullPath).Run(); err != nil {
		return fmt.Errorf("create home mount point %s failed: %w", fullPath, err)
	}
	out, err := primitives.Run("mount", device, fullPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount home volume %s failed: %w: %s", device, err, strings.TrimSpace(string(out)))
	}
	if ctx.Logger != nil && len(out) > 0 {
		ctx.Logger.Command(string(out))
	}
	return verifyMount(fullPath, device)
}

// mountBtrfsSubvolumes mounts each extra btrfs subvolume recorded in the
// layout's MountPoints map. Each entry maps a mount point (e.g. "/home") to
// the device that holds it.
func mountBtrfsSubvolumes(ctx *shared.InstallContext, p *shared.PartitionInfo, rootDevice string) error {
	for mountPath, device := range p.MountPoints {
		// Skip the root mount point — already mounted by Run.
		if mountPath == "/" {
			continue
		}
		if device == "" {
			device = rootDevice
		}

		// Find the subvolume definition for this mount point to get its options.
		sv := shared.FindSubvolumeByMountPoint(shared.GetBtrfsSubvolumes(), mountPath)
		if sv == nil {
			continue
		}

		// Create the mount point directory.
		fullPath := shared.MountRoot + mountPath
		_ = primitives.Run("mkdir", "-p", fullPath).Run()

		// Mount the subvolume with its properties.
		mountOpts := primitives.BuildMountOpts(sv)
		out, err := primitives.Run("mount", "-o", mountOpts, device, fullPath).CombinedOutput()
		if err != nil {
			return fmt.Errorf("mount %s subvolume (%s) failed: %w: %s", sv.Name, fullPath, err, string(out))
		}
		if ctx.Logger != nil && len(out) > 0 {
			ctx.Logger.Command(string(out))
		}
	}
	return nil
}

// ensureLuksOpen opens the LUKS root mapper if it isn't already open. The
// format step closes the mapper after creating the filesystem, so mount must
// reopen it. Idempotent: if the mapper device already exists, skip.
func ensureLuksOpen(ctx *shared.InstallContext, rootPart string) error {
	if len(ctx.LuksPassphrase) == 0 {
		return fmt.Errorf("LUKS enabled but no passphrase set")
	}
	// The /dev/mapper/<name> node can linger after cryptsetup close removed the
	// underlying dm-crypt device (udev removes the node asynchronously while
	// cryptsetup close removes the device synchronously). A stale node makes
	// os.Stat succeed even though the device is gone, which would make the
	// subsequent mount fail with "fsconfig() failed: No such file or directory".
	// Verify the dm device is really open before skipping the reopen.
	if isLuksMapperOpen(shared.LuksMapperName) {
		if ctx.Logger != nil {
			ctx.Logger.Info("LUKS root mapper already open (%s)", shared.LuksMapperPath)
		}
		return nil
	}
	if err := luks.LuksOpen(rootPart, shared.LuksMapperName, string(ctx.LuksPassphrase)); err != nil {
		return fmt.Errorf("opening LUKS root for mount: %w", err)
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("Opened LUKS root mapper %s", shared.LuksMapperPath)
	}
	return nil
}

// isLuksMapperOpen reports whether the dm-crypt mapper is actually open, i.e.
// backed by a live device-mapper node. cryptsetup status returns 0 only when
// the mapper exists and is open; a stale /dev/mapper node (device already
// removed) makes it return non-zero, so the caller reopens.
func isLuksMapperOpen(mapperName string) bool {
	_, err := primitives.Run("cryptsetup", "status", mapperName).Output()
	return err == nil
}

// BuildMountOpts is an alias for primitives.BuildMountOpts.
var BuildMountOpts = primitives.BuildMountOpts

// verifyBindMount checks that target really is a bind mount of source by
// comparing the device/inode pair of both directories. A plain directory at the
// target has a different inode, so a silently failed bind is caught here.
func verifyBindMount(source, target string) error {
	srcInfo, err := primitives.Run("stat", "-c", "%d:%i", source).Output()
	if err != nil {
		return fmt.Errorf("mount verification failed: cannot stat %s: %w", source, err)
	}
	tgtInfo, err := primitives.Run("stat", "-c", "%d:%i", target).Output()
	if err != nil {
		return fmt.Errorf("mount verification failed: cannot stat %s: %w", target, err)
	}
	if strings.TrimSpace(string(srcInfo)) != strings.TrimSpace(string(tgtInfo)) {
		return fmt.Errorf("mount verification failed: %s is not a bind mount of %s", target, source)
	}
	return nil
}

// verifyMount checks that a mountpoint is actually mounted and references the expected device.
// Uses findmnt which is more reliable than mountpoint (works even if mountpoint package is absent).
// For btrfs subvolume mounts, findmnt returns "device[/subvol]" (e.g. "/dev/vda2[/@]"),
// so we strip the [/subvol] suffix before comparing.
//
// The comparison is by device, not by spelling: findmnt names the node the mount
// actually reached, and for a logical volume that is its device-mapper node —
// where LVM doubles every hyphen of the group name, so the root LV of
// aelum-1a2b3c4d is reported as /dev/mapper/aelum--1a2b3c4d-root while the
// installer mounted it as /dev/aelum-1a2b3c4d/root. Those are the same device,
// so when the two strings differ the paths are resolved to the node underneath
// before the mount is declared wrong (see sameDeviceNode).
func verifyMount(mountpoint, expectedDevice string) error {
	// Check that the mountpoint is mounted
	cmd := primitives.Run("findmnt", "-n", "-o", "SOURCE", mountpoint)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("mount verification failed: %s is not mounted", mountpoint)
	}
	mountedDevice := strings.TrimSpace(string(output))
	if mountedDevice == "" {
		return fmt.Errorf("mount verification failed: %s is not mounted", mountpoint)
	}
	// Strip btrfs subvolume notation: "/dev/vda2[/@]" → "/dev/vda2"
	if idx := strings.Index(mountedDevice, "["); idx != -1 {
		mountedDevice = mountedDevice[:idx]
	}
	// Verify it matches the expected device
	if mountedDevice != expectedDevice && !sameDeviceNode(mountedDevice, expectedDevice) {
		return fmt.Errorf("mount verification failed: %s is mounted on %s, expected %s",
			mountpoint, mountedDevice, expectedDevice)
	}
	return nil
}

// sameDeviceNode reports whether two device paths reach the same device. Every
// device-mapper device has several names for one node: a logical volume as
// /dev/<vg>/<lv> and as /dev/mapper/<vg>-<lv> (hyphens doubled), a LUKS root as
// /dev/mapper/<name> and as its /dev/dm-N node. Resolving each path through
// those symlinks compares the devices themselves, so an alias never looks like a
// wrong mount. Paths that cannot be resolved, or that resolve to nothing, are
// never treated as the same device.
func sameDeviceNode(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	first, err := resolveDevicePath(a)
	if err != nil || first == "" {
		return false
	}
	second, err := resolveDevicePath(b)
	if err != nil || second == "" {
		return false
	}
	return first == second
}

// resolveDevicePath returns the node a device path points at, following the
// symlinks udev creates for device-mapper devices (/dev/<vg>/<lv>,
// /dev/mapper/<name> → /dev/dm-N). readlink -f is used rather than Go's
// EvalSymlinks so the lookup stays on the CommandRunner seam the tests script.
func resolveDevicePath(path string) (string, error) {
	out, err := primitives.Run("readlink", "-f", path).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

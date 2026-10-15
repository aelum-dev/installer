package mount

import (
	"fmt"
	"os/exec"
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
//   - /mnt/boot: EFI partition (ESP, where kernel/initrd live for systemd-boot)
//
// For btrfs subvolume layouts:
//   - /mnt: subvolume with MountPoint "/" (e.g. @), with properties from layout
//   - /mnt/<mountPoint>: each additional subvolume, with properties from layout
//   - /mnt/boot: EFI partition (ESP)
//
// For ext4 LVM layouts:
//   - /mnt: root logical volume
//   - /mnt/home: home logical volume
//   - /mnt/boot: EFI partition (ESP)
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

	p := ctx.Partitions
	var cmd *exec.Cmd
	var output []byte
	var err error

	// Resolve the device that holds the root filesystem (and its subvolumes):
	//   LUKS+LVM : open LUKS, activate the VG, root lives on the root LV
	//   LUKS     : open LUKS, root lives on the decrypted mapper
	//   LVM      : activate the VG, root lives on the root LV
	//   else     : root lives on the raw partition
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
	if ctx.Lvm {
		if p.RootLV == "" {
			return fmt.Errorf("LVM enabled but root LV path not set (format step failed)")
		}
		// The VG must be active for the LVs to appear. For the LUKS+LVM path
		// this runs after the mapper is open, so the PV is available.
		if err := lvm.VGActivate(shared.LvmVolumeGroup); err != nil {
			return err
		}
		if ctx.Logger != nil {
			ctx.Logger.Info("LVM root enabled; mounting root LV %s", p.RootLV)
		}
		rootDevice = p.RootLV
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

	// ── Mount additional filesystems (btrfs subvolumes / non-btrfs LVM home) ──
	if err := mountAdditionalMounts(ctx, p, rootDevice); err != nil {
		return err
	}

	// ── Mount EFI partition at /mnt/boot ──

	bootMount := shared.BootMountPoint

	cmd = primitives.Run("mkdir", "-p", bootMount)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create boot mount point failed: %w: %s", err, string(output))
	}

	cmd = primitives.Run("mount", p.EFIPart, bootMount)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount EFI failed: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	// Verify EFI mount
	if err := verifyMount(bootMount, p.EFIPart); err != nil {
		return err
	}

	return nil
}

// mountAdditionalMounts mounts every filesystem beyond root that the layout
// defines: the btrfs subvolumes and, for a non-btrfs LVM layout, the home
// logical volume at /home.
func mountAdditionalMounts(ctx *shared.InstallContext, p *shared.PartitionInfo, rootDevice string) error {
	if ctx.FileSystem == shared.FileSystemBtrfs {
		return mountBtrfsSubvolumes(ctx, p, rootDevice)
	}
	if ctx.Lvm {
		return mountLvmHome(p)
	}
	return nil
}

// mountBtrfsSubvolumes mounts each extra btrfs subvolume recorded in the
// layout's MountPoints map. Each entry maps a mount point (e.g. "/home") to the
// device that holds it: with LVM, /home lives on the home LV while the rest
// share the root LV.
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

// mountLvmHome mounts the LVM home logical volume at /home for a non-btrfs
// (ext4) LVM layout, where /home is a separate filesystem rather than a btrfs
// subvolume on the home LV.
func mountLvmHome(p *shared.PartitionInfo) error {
	if p.HomeLV == "" {
		return fmt.Errorf("LVM enabled but home LV path not set (format step failed)")
	}
	homePath := shared.MountRoot + "/home"
	_ = primitives.Run("mkdir", "-p", homePath).Run()
	out, err := primitives.Run("mount", p.HomeLV, homePath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount home LV (%s) failed: %w: %s", homePath, err, string(out))
	}
	return verifyMount(homePath, p.HomeLV)
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

// verifyMount checks that a mountpoint is actually mounted and references the expected device.
// Uses findmnt which is more reliable than mountpoint (works even if mountpoint package is absent).
// For btrfs subvolume mounts, findmnt returns "device[/subvol]" (e.g. "/dev/vda2[/@]"),
// so we strip the [/subvol] suffix before comparing.
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
	if mountedDevice != expectedDevice {
		return fmt.Errorf("mount verification failed: %s is mounted on %s, expected %s",
			mountpoint, mountedDevice, expectedDevice)
	}
	return nil
}

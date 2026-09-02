package mount

import (
	"fmt"
	"os/exec"
	"strings"

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
		cmd = primitives.Run("mount", "--mkdir", "-o", mountOpts, p.RootPart, shared.MountRoot)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("mount btrfs %s subvolume failed: %w: %s", rootSubvol.Name, err, string(output))
		}
	} else {
		// Standard mount for ext4 and other filesystems. --mkdir creates the mount
		// point (e.g. /mnt) if the live ISO rootfs doesn't already provide it.
		cmd = primitives.Run("mount", "--mkdir", p.RootPart, shared.MountRoot)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("mount root failed: %w: %s", err, string(output))
		}
	}

	// Verify root mount
	if err := verifyMount(shared.MountRoot, p.RootPart); err != nil {
		return err
	}

	// ── Mount additional subvolumes ──

	if ctx.FileSystem == shared.FileSystemBtrfs {
		// Use MountPoints map (populated by FormatDisk) to mount additional subvolumes.
		// Each entry maps a mount point (e.g., "/home") to the root partition path.
		for mountPath := range p.MountPoints {
			// Skip root mount point — already mounted above
			if mountPath == "/" {
				continue
			}

			// Find the subvolume definition for this mount point to get its options
			var sv *shared.BtrfsSubvolume
			for _, candidate := range shared.GetBtrfsSubvolumes() {
				if candidate.MountPoint == mountPath {
					sv = &candidate
					break
				}
			}
			if sv == nil {
				continue
			}

			// Create mount point directory
			fullPath := shared.MountRoot + mountPath
			_ = primitives.Run("mkdir", "-p", fullPath).Run()

			// Mount subvolume with its properties
			mountOpts := primitives.BuildMountOpts(sv)
			cmd = primitives.Run("mount", "-o", mountOpts, p.RootPart, fullPath)
			output, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("mount %s subvolume (%s) failed: %w: %s", sv.Name, fullPath, err, string(output))
			}
			if ctx.Logger != nil && len(output) > 0 {
				ctx.Logger.Command(string(output))
			}
		}
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

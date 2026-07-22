package steps

import (
	"fmt"
	"os/exec"
	"strings"

	"aesthetic-os-installer/backend/shared"
)

// MountFilesystem mounts the root partition, any btrfs subvolumes, and the EFI partition.
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
// from the disk ID. This matches the layout created by FormatDisk.
func MountFilesystem(ctx *InstallContext) error {
	// Validate preconditions
	if ctx.DiskID == "" {
		return fmt.Errorf("disk ID not set. Run --format-disk <disk> first")
	}
	if ctx.Partitions == nil {
		return fmt.Errorf("partitions not initialized. Run --format-disk <disk> first")
	}
	if ctx.Layout.Name == "" {
		return fmt.Errorf("layout not set. Run --format-disk <disk> first")
	}

	// Validate that partitions exist (from FormatDisk) or can be derived
	if ctx.Partitions.EFIPart == "" || ctx.Partitions.RootPart == "" {
		return fmt.Errorf("partition paths not set. FormatDisk may have failed")
	}

	p := ctx.Partitions

	// ── Mount root partition ──

	// Find the subvolume with MountPoint "/" for btrfs layouts, otherwise mount raw.
	var rootSubvol *shared.BtrfsSubvol
	for i := range ctx.Layout.Subvolumes {
		if ctx.Layout.Subvolumes[i].MountPoint == "/" {
			rootSubvol = &ctx.Layout.Subvolumes[i]
			break
		}
	}

	if rootSubvol != nil {
		// Mount btrfs subvolume with its properties (e.g. subvol=@,compress=zstd)
		mountOpts := buildMountOpts(rootSubvol)
		if ctx.Logger != nil {
			ctx.Logger.Info("Mounting btrfs %s subvolume → /mnt (%s)", rootSubvol.Name, mountOpts)
		}
		cmd := exec.Command("sudo", ctx.Helper, "mount", "-o", mountOpts, p.RootPart, "/mnt")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("mount btrfs %s subvolume failed: %w: %s", rootSubvol.Name, err, string(output))
		}
		if ctx.Logger != nil && len(output) > 0 {
			ctx.Logger.Command(string(output))
		}
	} else {
		// Standard mount for ext4 and other filesystems
		if ctx.Logger != nil {
			ctx.Logger.Info("Mounting root partition (%s) → /mnt", p.RootPart)
		}
		cmd := exec.Command("sudo", ctx.Helper, "mount", p.RootPart, "/mnt")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("mount root failed: %w: %s", err, string(output))
		}
		if ctx.Logger != nil && len(output) > 0 {
			ctx.Logger.Command(string(output))
		}
	}

	// Verify root mount
	if err := verifyMount("/mnt", p.RootPart); err != nil {
		return err
	}

	// ── Mount additional subvolumes ──

	for _, sv := range ctx.Layout.Subvolumes {
		if sv.MountPoint == "/" {
			continue // already mounted as root
		}

		// Create mount point directory
		mountPath := "/mnt" + sv.MountPoint
		cmd := exec.Command("sudo", ctx.Helper, "run", "mkdir", "-p", mountPath)
		cmd.Run()

		// Mount subvolume with its properties
		mountOpts := buildMountOpts(&sv)
		if ctx.Logger != nil {
			ctx.Logger.Info("Mounting btrfs %s subvolume → %s (%s)", sv.Name, mountPath, mountOpts)
		}
		cmd = exec.Command("sudo", ctx.Helper, "mount", "-o", mountOpts, p.RootPart, mountPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("mount %s subvolume (%s) failed: %w: %s", sv.Name, mountPath, err, string(output))
		}
		if ctx.Logger != nil && len(output) > 0 {
			ctx.Logger.Command(string(output))
		}
	}

	// ── Mount EFI partition at /mnt/boot ──

	bootMount := "/mnt/boot"
	if ctx.Logger != nil {
		ctx.Logger.Info("Mounting EFI partition (%s) → %s", p.EFIPart, bootMount)
	}

	cmd := exec.Command("sudo", ctx.Helper, "run", "mkdir", "-p", bootMount)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create boot mount point failed: %w: %s", err, string(output))
	}

	cmd = exec.Command("sudo", ctx.Helper, "mount", p.EFIPart, bootMount)
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

	if ctx.Logger != nil {
		ctx.Logger.Info("All mounts verified successfully")
	}
	return nil
}

// verifyMount checks that a mountpoint is actually mounted and references the expected device.
// Uses findmnt which is more reliable than mountpoint (works even if mountpoint package is absent).
// For btrfs subvolume mounts, findmnt returns "device[/subvol]" (e.g. "/dev/vda2[/@]"),
// so we strip the [/subvol] suffix before comparing.
func verifyMount(mountpoint, expectedDevice string) error {
	// Check that the mountpoint is mounted
	cmd := exec.Command("findmnt", "-n", "-o", "SOURCE", mountpoint)
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

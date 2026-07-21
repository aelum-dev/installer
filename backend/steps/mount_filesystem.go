package steps

import (
	"fmt"
	"os/exec"
	"strings"

	"aesthetic-os-installer/backend/shared"
)

// MountFilesystem mounts the root and EFI partitions using sudo.
// For btrfs subvolume layouts, also mounts subvolumes (@ → /, @home → /home).
// Mount points (standard Arch procedure):
//   - /mnt: root partition (or @ subvolume for btrfs)
//   - /mnt/boot: EFI partition (where kernel/initrd live, and $BOOT for systemd-boot)
//
// If ctx.Partitions is nil (standalone invocation), it derives partition paths
// from the disk ID: /dev/sda → /dev/sda1 (EFI) + /dev/sda2 (root).
// This matches the layout created by FormatDisk.
func MountFilesystem(ctx *InstallContext) error {
	// Validate that partitions exist (from FormatDisk) or can be derived
	if ctx.Partitions == nil {
		if ctx.DiskID == "" {
			return fmt.Errorf("disk ID not set. Provide a disk path (e.g., --mount-filesystem /dev/vda)")
		}
		// Discover partition paths via lsblk (handles NVMe naming correctly)
		partitions, err := shared.GetPartitions(ctx.DiskID)
		if err != nil {
			return fmt.Errorf("failed to discover partitions: %w", err)
		}

		// Find EFI and root partitions — try fstype first, fall back to partition index.
		efiPart, rootPart := findPartitionsByFstype(partitions)
		if efiPart == nil || rootPart == nil {
			if ctx.Logger != nil {
				ctx.Logger.Info("FSTYPE not available, falling back to partition index (1=EFI, 2=root)")
			}
			efiPart, rootPart = findPartitionsByIndex(partitions)
		}

		if efiPart == nil {
			return fmt.Errorf("could not find EFI partition on %s (got %d partitions)", ctx.DiskID, len(partitions))
		}
		if rootPart == nil {
			return fmt.Errorf("could not find root partition on %s (got %d partitions)", ctx.DiskID, len(partitions))
		}
		ctx.Partitions = &shared.PartitionInfo{
			Disk:        ctx.DiskID,
			EFIPart:     efiPart.Path,
			EFIUUID:     efiPart.UUID,
			RootPart:    rootPart.Path,
			RootUUID:    rootPart.UUID,
			MountPoints: make(map[string]string),
		}
	}
	p := ctx.Partitions

	// Mount root partition
	if ctx.Logger != nil {
		ctx.Logger.Info("Mounting root partition (%s) → /mnt", p.RootPart)
	}

	// For btrfs subvolume layout, mount without default subvolume
	var mountOpts string
	if p.LayoutName == "btrfs-subvolumes" {
		mountOpts = "-o subvolid=5"
		if ctx.Logger != nil {
			ctx.Logger.Info("Mounting btrfs root without default subvolume (subvolid=5)")
		}
	}

	cmd := exec.Command("sudo", ctx.Helper, "mount")
	if mountOpts != "" {
		cmd.Args = append(cmd.Args, mountOpts)
	}
	cmd.Args = append(cmd.Args, p.RootPart, "/mnt")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount root failed: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	// Verify root mount
	if err := verifyMount("/mnt", p.RootPart); err != nil {
		return err
	}

	// For btrfs subvolumes, create mount points and mount subvolumes
	if p.LayoutName == "btrfs-subvolumes" {
		if ctx.Logger != nil {
			ctx.Logger.Info("Mounting btrfs subvolumes (@ and @home)")
		}

		// Create /home mount point
		cmd = exec.Command("sudo", ctx.Helper, "run", "mkdir", "-p", "/mnt/home")
		cmd.Run()

		// Mount @home subvolume at /mnt/home
		if homePart, ok := p.MountPoints["/home"]; ok {
			cmd = exec.Command("sudo", ctx.Helper, "mount", "-o", "subvol=@home", homePart, "/mnt/home")
			output, err = cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("mount @home subvolume failed: %w: %s", err, string(output))
			}
			if ctx.Logger != nil && len(output) > 0 {
				ctx.Logger.Command(string(output))
			}
		}
	}

	// Mount EFI partition at /mnt/boot (standard Arch: kernel files live here, on the ESP)
	// This makes /mnt/boot the $BOOT partition for systemd-boot.
	bootMount := "/mnt/boot"
	if ctx.Logger != nil {
		ctx.Logger.Info("Mounting EFI partition (%s) → %s", p.EFIPart, bootMount)
	}

	cmd = exec.Command("sudo", ctx.Helper, "run", "mkdir", "-p", bootMount)
	output, err = cmd.CombinedOutput()
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
	// Verify it matches the expected device
	if mountedDevice != expectedDevice {
		return fmt.Errorf("mount verification failed: %s is mounted on %s, expected %s",
			mountpoint, mountedDevice, expectedDevice)
	}
	return nil
}

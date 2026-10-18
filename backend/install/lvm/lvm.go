// Package lvm implements the LVM operations the installer needs: create the
// `aelum` volume group with a root and a home logical volume, format them with
// the selected filesystem (btrfs with subvolumes, or ext4), activate the group
// for mounting, and tear it down again.
//
// The root partition becomes the physical volume (PV). When LUKS is enabled the
// PV is the decrypted mapper device (/dev/mapper/aelum) instead of the raw
// partition — the caller decides which one to pass, so a single routine covers
// all four install paths (auto/manual × LUKS on/off).
package lvm

import (
	"fmt"
	"strconv"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// LV describes a logical volume created by Apply.
type LV struct {
	Name       string // "root" | "home"
	Path       string // /dev/mapper/aelum-root
	FileSystem string // "btrfs" | "ext4"
	MountPoint string // "/" | "/home"
}

// Params configures the volume group Apply creates. Both LVs use the same
// filesystem: btrfs (the root LV carries every subvolume except @home, which
// the home LV carries) or ext4.
type Params struct {
	// FileSystem is the filesystem for both LVs (defaults to btrfs).
	FileSystem string
}

// Formatter formats a freshly created logical volume: it writes the filesystem
// (mkfs) and creates the given btrfs subvolumes on it. It is injected by the
// caller because the format package imports this one to orchestrate LVM, so
// this package must not import format back (that would be an import cycle).
type Formatter func(device, fsType string, subvolumes []shared.BtrfsSubvolume) error

// Apply tears down any existing `aelum` volume group, creates the PV/VG, creates
// the root and home logical volumes, and formats each one via format (btrfs with
// the LVM subvolume schemes, or ext4 with none). It returns the created logical
// volumes in creation order (root, then home).
//
// device is the raw root partition (LUKS off) or the decrypted LUKS mapper
// (/dev/mapper/aelum, LUKS on).
//
// Sizing: root = max(30% of the VG, 8 GiB); home takes 100% of the rest. The
// device must be at least shared.LvmVGMinBytes (16 GiB) so both LVs meet the
// 8 GiB floor.
func Apply(device string, params Params, format Formatter) ([]LV, error) {
	if device == "" {
		return nil, fmt.Errorf("lvm: device is required")
	}
	if format == nil {
		return nil, fmt.Errorf("lvm: formatter is required")
	}
	fsType := params.FileSystem
	if fsType == "" {
		fsType = shared.FileSystemBtrfs
	}

	sizeBytes, err := DeviceSizeBytes(device)
	if err != nil {
		return nil, err
	}
	// Check the floor before Teardown: a too-small device must not lose an
	// existing `aelum` volume group on its way to the same size error.
	if sizeBytes < shared.LvmVGMinBytes {
		return nil, fmt.Errorf("LVM requires at least %d GiB of space (device %s has %d GiB)",
			shared.LvmVGMinBytes>>30, device, sizeBytes>>30)
	}

	// Idempotent retries: remove any group left behind by a prior attempt.
	Teardown(shared.LvmVolumeGroup, []string{device})

	if out, err := primitives.Run(pvCreateArgs(device)...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("pvcreate %s failed: %w: %s", device, err, strings.TrimSpace(string(out)))
	}
	if out, err := primitives.Run(vgCreateArgs(shared.LvmVolumeGroup, device)...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("vgcreate %s failed: %w: %s", shared.LvmVolumeGroup, err, strings.TrimSpace(string(out)))
	}

	rootMiB, err := rootLVSizeMiB(shared.LvmVolumeGroup)
	if err != nil {
		return nil, err
	}
	if out, err := primitives.Run(lvCreateSizeArgs(rootMiB, shared.LvmRootVolume, shared.LvmVolumeGroup)...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("lvcreate %s failed: %w: %s", shared.LvmRootVolume, err, strings.TrimSpace(string(out)))
	}
	if out, err := primitives.Run(lvCreateRestArgs(shared.LvmHomeVolume, shared.LvmVolumeGroup)...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("lvcreate %s failed: %w: %s", shared.LvmHomeVolume, err, strings.TrimSpace(string(out)))
	}

	root := LV{Name: shared.LvmRootVolume, Path: Path(shared.LvmVolumeGroup, shared.LvmRootVolume), FileSystem: fsType, MountPoint: "/"}
	home := LV{Name: shared.LvmHomeVolume, Path: Path(shared.LvmVolumeGroup, shared.LvmHomeVolume), FileSystem: fsType, MountPoint: "/home"}

	if err := formatLV(format, root, shared.GetLvmBtrfsSubvolumes()); err != nil {
		return nil, fmt.Errorf("format root LV: %w", err)
	}
	if err := formatLV(format, home, shared.GetLvmHomeSubvolumes()); err != nil {
		return nil, fmt.Errorf("format home LV: %w", err)
	}

	return []LV{root, home}, nil
}

// formatLV formats one logical volume, passing the btrfs subvolumes only when
// the filesystem is btrfs.
func formatLV(format Formatter, lv LV, subvolumes []shared.BtrfsSubvolume) error {
	if lv.FileSystem != shared.FileSystemBtrfs {
		return format(lv.Path, lv.FileSystem, nil)
	}
	return format(lv.Path, lv.FileSystem, subvolumes)
}

// rootLVSizeMiB computes the root LV size in MiB: max(30% of the VG, 8 GiB),
// clamped so the home LV still gets at least the 8 GiB floor.
func rootLVSizeMiB(vg string) (int64, error) {
	vgMiB, err := VGSizeMiB(vg)
	if err != nil {
		return 0, err
	}
	return computeRootMiB(vgMiB)
}

// computeRootMiB is the pure sizing rule behind rootLVSizeMiB: root is 30% of
// the volume group, raised to the 8 GiB floor, but never so large that the home
// LV drops below the same floor. It errors when the VG cannot hold both.
func computeRootMiB(vgMiB int64) (int64, error) {
	minMiB := int64(shared.LvmRootMinBytes >> 20)
	rootMiB := vgMiB * int64(shared.LvmRootPercent) / 100
	if rootMiB < minMiB {
		rootMiB = minMiB
	}
	if maxRoot := vgMiB - minMiB; rootMiB > maxRoot {
		rootMiB = maxRoot
	}
	if rootMiB <= 0 {
		return 0, fmt.Errorf("volume group is too small to hold a root and home LV (%d MiB)", vgMiB)
	}
	return rootMiB, nil
}

// pvCreateArgs builds the pvcreate invocation for a physical volume.
func pvCreateArgs(device string) []string {
	return []string{"pvcreate", "-ff", "-y", device}
}

// vgCreateArgs builds the vgcreate invocation.
func vgCreateArgs(vg, device string) []string {
	return []string{"vgcreate", vg, device}
}

// lvCreateSizeArgs builds a fixed-size lvcreate invocation.
//
// -y and -W y are required: when the new LV's first extents still hold a
// filesystem signature from a previous install (LVM reuses the same LV name and
// extents), lvcreate would otherwise prompt "Wipe it? [y/n]" and, with no TTY,
// default to "n" and abort with exit status 5. The signature is always wiped
// because the caller formats the LV immediately afterwards.
func lvCreateSizeArgs(sizeMiB int64, lv, vg string) []string {
	return []string{"lvcreate", "-y", "-W", "y", "-L", fmt.Sprintf("%dM", sizeMiB), "-n", lv, vg}
}

// lvCreateRestArgs builds an lvcreate invocation that consumes all remaining
// free space in the volume group. -y / -W y have the same rationale as
// lvCreateSizeArgs.
func lvCreateRestArgs(lv, vg string) []string {
	return []string{"lvcreate", "-y", "-W", "y", "-l", "100%FREE", "-n", lv, vg}
}

// vgChangeArgs builds a vgchange activation/deactivation invocation.
func vgChangeArgs(activate bool, vg string) []string {
	mode := "-an"
	if activate {
		mode = "-ay"
	}
	return []string{"vgchange", mode, vg}
}

// vgRemoveArgs builds the force-remove invocation for a volume group.
func vgRemoveArgs(vg string) []string {
	return []string{"vgremove", "-ff", vg}
}

// pvRemoveArgs builds the force-remove invocation for a physical volume.
func pvRemoveArgs(device string) []string {
	return []string{"pvremove", "-ff", "-y", device}
}

// Path returns the device-mapper path of a logical volume (e.g.
// /dev/mapper/aelum-root).
func Path(vg, lv string) string {
	return "/dev/mapper/" + vg + "-" + lv
}

// VGActivate activates the volume group so its logical volumes appear (used by
// the mount step, and after LUKS is opened for the LUKS+LVM path).
func VGActivate(vg string) error {
	out, err := primitives.Run(vgChangeArgs(true, vg)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("vgchange -ay %s failed: %w: %s", vg, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// VGDeactivate deactivates the volume group so it can be closed/removed. It must
// run before closing the LUKS mapper the group sits on.
func VGDeactivate(vg string) error {
	out, err := primitives.Run(vgChangeArgs(false, vg)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("vgchange -an %s failed: %w: %s", vg, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// VGSizeMiB returns the volume group's size in MiB (via `vgs`).
func VGSizeMiB(vg string) (int64, error) {
	out, err := primitives.Run("vgs", "--noheadings", "--nosuffix", "--units", "m", "-o", "vg_size", vg).Output()
	if err != nil {
		return 0, fmt.Errorf("vgs %s: %w", vg, err)
	}
	trimmed := strings.TrimSpace(string(out))
	size, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, fmt.Errorf("parse vg size %q: %w", trimmed, err)
	}
	return int64(size), nil
}

// DeviceSizeBytes returns the size of a block device in bytes (via blockdev).
func DeviceSizeBytes(device string) (int64, error) {
	out, err := primitives.Run("blockdev", "--getsize64", device).Output()
	if err != nil {
		return 0, fmt.Errorf("blockdev %s: %w", device, err)
	}
	size, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse device size %q: %w", strings.TrimSpace(string(out)), err)
	}
	return size, nil
}

// Teardown removes the volume group and any PV signatures on the given devices,
// ignoring every error. It is the idempotent cleanup used before creating the
// group and by rollback.
func Teardown(vg string, devices []string) {
	_ = primitives.Run(vgChangeArgs(false, vg)...).Run()
	_ = primitives.Run(vgRemoveArgs(vg)...).Run()
	// vgremove can leave the LV device-mapper nodes behind when vgchange -an
	// could not close them, or when they are already orphaned. They keep the
	// underlying device busy for exclusive opens (pvcreate, mkfs) — "Device or
	// resource busy" / "Can't open exclusively" — so remove them explicitly.
	for _, lv := range []string{shared.LvmRootVolume, shared.LvmHomeVolume} {
		_ = primitives.Run("dmsetup", "remove", Path(vg, lv)).Run()
	}
	for _, device := range devices {
		if device != "" {
			_ = primitives.Run(pvRemoveArgs(device)...).Run()
		}
	}
}

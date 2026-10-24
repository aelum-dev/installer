// Package lvm implements the LVM operations the installer needs: create a
// per-install volume group on a single physical volume, create the logical
// volumes, activate the group for mounting, and tear it down again.
//
// Design rules (see AGENTS.md):
//   - One PV per install: the System partition, or its LUKS mapper. LUKS always
//     sits below LVM, so there is exactly one encrypted device and one
//     passphrase.
//   - The volume group name is derived from the PV's PARTUUID
//     ("aelum-<8 hex>"), never a global constant, so two installs can never
//     collide and teardown can never target another system's volumes.
//   - Teardown is scoped by looking the group up from the PV (`pvs`), and a
//     group that spans more than that one PV is refused rather than removed.
//   - Logical volumes are addressed as /dev/<vg>/<lv>, never /dev/mapper/
//     paths: LVM escapes hyphens in device-mapper names, so a hand-built
//     /dev/mapper/<vg>-<lv> string is wrong for a hyphenated group name.
//
// This package deliberately does not format anything: the format package
// orchestrates LVM and calls back into mkfs/btrfs, so importing format here
// would create an import cycle.
package lvm

import (
	"fmt"
	"strconv"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// VolumeGroupName returns the volume group name for a partition's PARTUUID.
//
// PARTUUID belongs to the partition table, so it exists before any mkfs and
// survives luksFormat: the same name is derived whether LVM sits on the raw
// partition or on its LUKS mapper, and a retried install reuses it instead of
// leaking a second group.
func VolumeGroupName(partUUID string) string {
	return shared.LvmVolumeGroupPrefix + shared.InstallID(partUUID)
}

// PARTUUID returns a block device's PARTUUID (via blkid).
func PARTUUID(device string) (string, error) {
	out, err := primitives.Run("blkid", "-s", "PARTUUID", "-o", "value", device).Output()
	if err != nil {
		return "", fmt.Errorf("blkid PARTUUID %s: %w", device, err)
	}
	uuid := strings.TrimSpace(string(out))
	if uuid == "" {
		return "", fmt.Errorf("blkid %s returned no PARTUUID; LVM needs a partitioned device", device)
	}
	return uuid, nil
}

// VGNameForDevice derives the volume group name from a device's PARTUUID.
func VGNameForDevice(device string) (string, error) {
	uuid, err := PARTUUID(device)
	if err != nil {
		return "", err
	}
	return VolumeGroupName(uuid), nil
}

// Path returns the device path of a logical volume (e.g. /dev/aelum-1a2b3c4d/root).
//
// The /dev/<vg>/<lv> symlink LVM creates is used on purpose: it is correct for
// hyphenated group names, whereas the device-mapper node escapes each hyphen as
// two (aelum--1a2b3c4d-root) and a hand-built /dev/mapper path would not exist.
func Path(vg, lv string) string {
	return "/dev/" + vg + "/" + lv
}

// DeviceSizeBytes returns a block device's size in bytes (via blockdev).
func DeviceSizeBytes(device string) (int64, error) {
	out, err := primitives.Run("blockdev", "--getsize64", device).Output()
	if err != nil {
		return 0, fmt.Errorf("blockdev %s: %w", device, err)
	}
	trimmed := strings.TrimSpace(string(out))
	size, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse device size %q: %w", trimmed, err)
	}
	return size, nil
}

// VGSizeMiB returns a volume group's size in MiB (via vgs).
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

// Create makes device the single physical volume of volume group vg.
//
// Any stale LVM state on that device is released first, scoped to the device
// itself — the installer never removes a group by name. A group that spans
// several physical volumes is refused: only this install's partition may be
// touched, and a multi-PV group belongs to something else.
func Create(device, vg string) error {
	if device == "" {
		return fmt.Errorf("lvm: device is required")
	}
	if vg == "" {
		return fmt.Errorf("lvm: volume group name is required")
	}

	if err := ReleaseDevice(device); err != nil {
		return err
	}

	sizeBytes, err := DeviceSizeBytes(device)
	if err != nil {
		return err
	}
	if sizeBytes < shared.MinLvmVolumeGroupBytes {
		return fmt.Errorf("LVM requires at least %s of space (device %s has %s)",
			shared.FormatSize(shared.MinLvmVolumeGroupBytes), device, shared.FormatSize(sizeBytes))
	}

	if out, err := primitives.Run(pvCreateArgs(device)...).CombinedOutput(); err != nil {
		return fmt.Errorf("pvcreate %s failed: %w: %s", device, err, strings.TrimSpace(string(out)))
	}
	if out, err := primitives.Run(vgCreateArgs(vg, device)...).CombinedOutput(); err != nil {
		return fmt.Errorf("vgcreate %s failed: %w: %s", vg, err, strings.TrimSpace(string(out)))
	}

	// lvm2 rounds the usable size down to whole extents; confirm the group is
	// still big enough to hold the root and home floors before any LV exists.
	vgMiB, err := VGSizeMiB(vg)
	if err != nil {
		return err
	}
	if minMiB := int64(shared.MinLvmVolumeGroupBytes >> 20); vgMiB < minMiB {
		return fmt.Errorf("volume group %s is too small: %d MiB (minimum %d MiB)", vg, vgMiB, minMiB)
	}
	return nil
}

// CreateRootLV creates the root logical volume and returns its device path.
//
// btrfs uses a single volume holding every subvolume, so it takes the whole
// group. ext4 gets a separate home volume and therefore a sized root that
// leaves room for it (see computeRootMiB).
func CreateRootLV(vg, fsType string) (string, error) {
	var args []string
	if fsType == shared.FileSystemBtrfs {
		args = lvCreateRestArgs(shared.LvmRootVolumeName, vg)
	} else {
		vgMiB, err := VGSizeMiB(vg)
		if err != nil {
			return "", err
		}
		rootMiB, err := computeRootMiB(vgMiB)
		if err != nil {
			return "", err
		}
		args = lvCreateSizeArgs(rootMiB, shared.LvmRootVolumeName, vg)
	}

	if out, err := primitives.Run(args...).CombinedOutput(); err != nil {
		return "", fmt.Errorf("lvcreate %s failed: %w: %s", shared.LvmRootVolumeName, err, strings.TrimSpace(string(out)))
	}
	return Path(vg, shared.LvmRootVolumeName), nil
}

// CreateHomeLV creates the home logical volume, consuming the rest of the
// group, and returns its device path. Only ext4 layouts use it: btrfs keeps
// /home on the @home subvolume of the root volume.
func CreateHomeLV(vg string) (string, error) {
	out, err := primitives.Run(lvCreateRestArgs(shared.LvmHomeVolumeName, vg)...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("lvcreate %s failed: %w: %s", shared.LvmHomeVolumeName, err, strings.TrimSpace(string(out)))
	}
	return Path(vg, shared.LvmHomeVolumeName), nil
}

// computeRootMiB is the sizing rule for the ext4 root volume: LvmRootPercent of
// the group, raised to the root floor, but never so large that the home volume
// drops below the same floor. It errors when the group cannot hold both.
func computeRootMiB(vgMiB int64) (int64, error) {
	minMiB := int64(shared.MinLvmRootVolumeBytes >> 20)
	rootMiB := vgMiB * int64(shared.LvmRootPercent) / 100
	if rootMiB < minMiB {
		rootMiB = minMiB
	}
	if maxRoot := vgMiB - minMiB; rootMiB > maxRoot {
		rootMiB = maxRoot
	}
	if rootMiB <= 0 {
		return 0, fmt.Errorf("volume group is too small to hold a root and home volume (%d MiB)", vgMiB)
	}
	return rootMiB, nil
}

// Activate makes a group's logical volumes available. It is idempotent: an
// already-active group is a no-op, so mount can call it on every run.
func Activate(vg string) error {
	out, err := primitives.Run(vgChangeArgs(true, vg)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("vgchange -ay %s failed: %w: %s", vg, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Deactivate closes a group's logical volumes. It must run before the LUKS
// mapper underneath is closed: cryptsetup refuses to tear down a device that an
// active logical volume still holds.
func Deactivate(vg string) error {
	out, err := primitives.Run(vgChangeArgs(false, vg)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("vgchange -an %s failed: %w: %s", vg, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// VGForDevice returns the volume group device is a physical volume of, and how
// many physical volumes that group has. Both are zero/empty when device is not
// a PV (the common case for a freshly partitioned disk).
func VGForDevice(device string) (vg string, pvCount int, err error) {
	out, err := primitives.Run("pvs", "--noheadings", "-o", "vg_name", device).Output()
	if err != nil {
		// pvs exits non-zero for a device without a PV label. That is not an
		// error here: the caller only wants to know whether stale LVM exists.
		return "", 0, nil
	}
	vg = strings.TrimSpace(string(out))
	if vg == "" {
		return "", 0, nil
	}

	countOut, err := primitives.Run("vgs", "--noheadings", "-o", "pv_count", vg).Output()
	if err != nil {
		return vg, 0, fmt.Errorf("vgs %s: %w", vg, err)
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(countOut)))
	if err != nil {
		return vg, 0, fmt.Errorf("parse pv_count %q for %s: %w", strings.TrimSpace(string(countOut)), vg, err)
	}
	return vg, count, nil
}

// ReleaseDevice removes the LVM state on one device: deactivate and remove the
// group the device belongs to (only when the device is its sole physical
// volume), then drop the PV label.
//
// The lookup is what makes this safe: the group is resolved from the device, so
// a hard-coded name is never trusted and another disk's group is never touched.
// A multi-PV group is refused outright — it is not this install's to remove.
func ReleaseDevice(device string) error {
	if device == "" {
		return nil
	}

	vg, pvCount, err := VGForDevice(device)
	if err != nil {
		return err
	}
	if vg != "" {
		if pvCount > 1 {
			return fmt.Errorf("device %s is a physical volume of volume group %q, which spans %d physical volumes; refusing to remove a group that this install does not own",
				device, vg, pvCount)
		}
		// Best-effort deactivation: vgremove needs the volumes closed, but a
		// failed deactivate is reported by vgremove itself.
		_ = Deactivate(vg)
		if out, err := primitives.Run(vgRemoveArgs(vg)...).CombinedOutput(); err != nil {
			return fmt.Errorf("vgremove %s failed: %w: %s", vg, err, strings.TrimSpace(string(out)))
		}
		removeLVNodes(vg)
	}

	// A device without a PV label is the normal case; pvremove's failure is
	// only informative there, so it is best-effort.
	_ = primitives.Run(pvRemoveArgs(device)...).Run()
	return nil
}

// ReleaseForDisk tears down LVM on every device derived from diskID: its
// partitions, any dm-crypt mapper on them, and any logical volume. It is the
// cleanup/rollback entry point and never fails the caller — a device it cannot
// release is logged and skipped.
//
// The PV may be a partition (LVM on bare metal) or a LUKS mapper (LUKS+LVM), so
// the device list is built from lsblk's child types rather than from
// partitions alone.
func ReleaseForDisk(diskID string, logger shared.Logger) {
	if diskID == "" {
		return
	}
	for _, device := range derivedDevices(diskID) {
		if err := ReleaseDevice(device); err != nil && logger != nil {
			logger.Error("Cleanup: releasing LVM on %s: %v", device, err)
		}
	}
}

// derivedDevices lists diskID's partitions, dm-crypt mappers, and logical
// volumes, parents first so a PV is released before its own children.
func derivedDevices(diskID string) []string {
	out, err := primitives.Run("lsblk", "-lnbo", "KNAME,TYPE", diskID).Output()
	if err != nil {
		return nil
	}

	var devices []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[1] {
		case "part", "crypt", "lvm":
			name := fields[0]
			if !strings.HasPrefix(name, "/dev/") {
				name = "/dev/" + name
			}
			devices = append(devices, name)
		}
	}
	return devices
}

// removeLVNodes drops any leftover device-mapper nodes for this group's logical
// volumes. vgremove normally removes them, but a node orphaned by an
// interrupted teardown keeps the underlying device busy for the exclusive open
// pvcreate/mkfs need, so removal is attempted and ignored.
func removeLVNodes(vg string) {
	for _, lv := range []string{shared.LvmRootVolumeName, shared.LvmHomeVolumeName} {
		_ = primitives.Run("dmsetup", "remove", mapperName(vg, lv)).Run()
	}
}

// mapperName returns the device-mapper name for a logical volume: every hyphen
// in the group and volume names is doubled by LVM (aelum-1a2b3c4d + root →
// aelum--1a2b3c4d-root). dmsetup takes the dm name, not a /dev path.
func mapperName(vg, lv string) string {
	escape := func(s string) string { return strings.ReplaceAll(s, "-", "--") }
	return escape(vg) + "-" + escape(lv)
}

// pvCreateArgs builds the pvcreate invocation. -ff overwrites any stale PV
// label, -y skips the confirmation prompt.
func pvCreateArgs(device string) []string {
	return []string{"pvcreate", "-ff", "-y", device}
}

// vgCreateArgs builds the vgcreate invocation for a single-PV group.
func vgCreateArgs(vg, device string) []string {
	return []string{"vgcreate", vg, device}
}

// lvCreateSizeArgs builds a fixed-size lvcreate invocation.
//
// -y and -W y are required: when the new volume's first extents still hold a
// filesystem signature from a previous install, lvcreate would otherwise prompt
// "Wipe it? [y/n]" and, with no TTY, default to "n" and abort. The caller
// formats the volume immediately afterwards anyway.
func lvCreateSizeArgs(sizeMiB int64, lv, vg string) []string {
	return []string{"lvcreate", "-y", "-W", "y", "-L", fmt.Sprintf("%dM", sizeMiB), "-n", lv, vg}
}

// lvCreateRestArgs builds an lvcreate invocation that consumes all remaining
// free space in the group. -y / -W y have the same rationale as above.
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

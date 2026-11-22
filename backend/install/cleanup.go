package install

import (
	"fmt"
	"strings"

	"aelum-installer/backend/install/luks"
	"aelum-installer/backend/install/lvm"
	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// Rollback
// ============================================================================

// Rollback cleans up a partial or failed installation.
//
// Every command here destroys something, so every command that fails becomes part
// of the returned error: a wipe refused with EBUSY leaves the old partition table
// and filesystems exactly where they were, and calling that disk "ready for a
// fresh install" sends the next attempt at a disk nobody believes is empty. The
// failures are collected instead of returned on the first one — a wipe that did
// work is still progress, and the person running this needs to see all of what is
// left behind at once, not one command per retry.
func Rollback(logger shared.Logger, diskID string) error {
	if diskID == "" {
		return fmt.Errorf("disk ID is required")
	}
	if logger != nil {
		logger.Info("Rolling back: cleaning up %s", diskID)
	}

	if err := primitives.CleanupMounts(logger); err != nil {
		if logger != nil {
			logger.Error("Rollback: cleanup failed: %v", err)
		}
		return fmt.Errorf("rollback cleanup failed: %w", err)
	}

	// Every command below is destructive, so each is attempted even after another
	// one failed — the disk must end up as empty as it can — but none of them may
	// fail silently: what did not work is what the caller has to be told.
	var failed []string

	// Deactivate and remove any volume group on the target before wiping
	// signatures: an active logical volume keeps the physical volume busy, and a
	// group must never outlive the partitions it sits on. The lookup is scoped to
	// this disk's devices, so a group on another disk is never touched.
	lvm.ReleaseForDisk(diskID, logger)

	// Close the root LUKS mapper once the volumes on it are gone and before any
	// signature is wiped: an open mapper holds its partition, and both wipefs and
	// sgdisk -Z are refused with EBUSY while it is open. A run killed after
	// cryptsetup open leaves the mapper behind, so this is what makes the wipe
	// below possible at all.
	if err := releaseLuksMapper(diskID, logger); err != nil {
		failed = append(failed, err.Error())
	}

	partitions := discoverPartitions(diskID)

	if len(partitions) == 0 {
		if logger != nil {
			logger.Info("Rollback: no partitions found on %s, skipping wipe", diskID)
		}
	} else {
		for _, part := range partitions {
			if logger != nil {
				logger.Info("Wiping signatures on %s", part)
			}
			out, err := primitives.Run("wipefs", "--all", "-f", part).CombinedOutput()
			if err != nil {
				if logger != nil {
					logger.Error("Failed to wipe %s: %v (%s)", part, err, string(out))
				}
				failed = append(failed, describeFailure("wipefs "+part, err, out))
			}
		}
	}

	if logger != nil {
		logger.Info("Wiping signatures on %s", diskID)
	}
	out, err := primitives.Run("wipefs", "--all", "-f", diskID).CombinedOutput()
	if err != nil {
		if logger != nil {
			logger.Error("Failed to wipe disk %s: %v (%s)", diskID, err, string(out))
		}
		failed = append(failed, describeFailure("wipefs "+diskID, err, out))
	}

	if logger != nil {
		logger.Info("Clearing GPT partition table on %s", diskID)
	}
	out, err = primitives.Run("sgdisk", "-Z", diskID).CombinedOutput()
	if err != nil {
		if logger != nil {
			logger.Error("Failed to clear GPT table on %s: %v (%s)", diskID, err, string(out))
		}
		failed = append(failed, describeFailure("sgdisk -Z "+diskID, err, out))
	}

	// The mount root stays as it is. shared.MountRoot is the installer's own target
	// root, not part of this disk: rolling one disk back must not delete a directory
	// another install or the live session is using, and the next install creates what
	// it needs again (mount --mkdir, then mkdir -p for every other mount point).

	if len(failed) > 0 {
		return fmt.Errorf("rollback of %s did not finish: %s", diskID, strings.Join(failed, "; "))
	}

	if logger != nil {
		logger.Info("Rollback complete: %s is ready for a fresh install", diskID)
	}
	return nil
}

// releaseLuksMapper closes the root LUKS mapper when an interrupted run left one
// open on this disk, retrying while the device is transiently busy (see
// closeLuksMapperWithRetry).
//
// Both probes matter. The mapper's name is the same on every install, so the device
// has to be traced to the disk actually being rolled back — a mapper another disk
// holds is not this rollback's to tear down, and closing one that is mounted would
// pull the device out from under it. And a disk that was never encrypted has no
// mapper, which is the usual case here and nothing to report. A mapper on this disk
// that will not close is a real failure: it holds its partition, so every wipe after
// it fails as well.
func releaseLuksMapper(diskID string, logger shared.Logger) error {
	if !luksMapperOnDisk(diskID, shared.LuksMapperName) || !luks.IsOpen(shared.LuksMapperName) {
		return nil
	}
	if logger != nil {
		logger.Info("Closing LUKS mapper %s", shared.LuksMapperName)
	}
	if err := closeLuksMapperWithRetry(); err != nil {
		if logger != nil {
			logger.Error("Failed to close LUKS mapper %s: %v", shared.LuksMapperName, err)
		}
		return fmt.Errorf("close LUKS mapper %s: %w", shared.LuksMapperName, err)
	}
	return nil
}

// luksMapperOnDisk reports whether this disk's own device tree carries the named
// dm-crypt mapper. lsblk lists a device-mapper device by its mapper name (KNAME), so
// the row for the root mapper can only belong to the disk holding the partition it
// was opened from.
func luksMapperOnDisk(diskID, mapperName string) bool {
	out, err := primitives.Run("lsblk", "-lnbo", "KNAME,TYPE", diskID).Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == "crypt" && fields[0] == mapperName {
			return true
		}
	}
	return false
}

// describeFailure renders one failed command for the caller: what was attempted,
// why it stopped, and what the command itself said. An ExitError only carries its
// status, so the output is the half that names the reason (EBUSY, a busy disk).
func describeFailure(command string, err error, out []byte) string {
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		return fmt.Sprintf("%s: %v", command, err)
	}
	return fmt.Sprintf("%s: %v: %s", command, err, msg)
}

func discoverPartitions(diskID string) []string {
	cmd := primitives.Run("lsblk", "-lnbo", "KNAME,TYPE", diskID)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return nil
	}
	var partitions []string
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[1] == "part" {
			name := fields[0]
			if !strings.HasPrefix(name, "/dev/") {
				name = "/dev/" + name
			}
			partitions = append(partitions, name)
		}
	}
	return partitions
}

package partition

import (
	"fmt"

	"aelum-installer/backend/shared"
)

// Validate checks a ManualLayout for the invariants required to install:
//   - exactly one ESP and exactly one System partition
//   - the sum of partition sizes must fit the disk
//   - at most one partition may request "use all remaining space" (SizeMB == 0)
//   - every type must be a known value
func Validate(layout shared.ManualLayout) error {
	// Debug: record the layout and every number behind the size checks so a
	// rejection that only happens on real disk geometry can be traced from the
	// installation log (see navigation.ts Next gate). Read over SSH in the live
	// ISO at /usr/local/bin/aelum-installer.log.
	var partsDebug string
	for _, p := range layout.Partitions {
		partsDebug += fmt.Sprintf("[path=%q sizeMB=%d type=%q] ", p.Path, p.SizeMB, p.Type)
	}
	shared.GetLogger().Info("ValidateManualLayout: diskMB=%d diskBytes=%d freeMB=%d freeSectors=%d parts=%s",
		layout.DiskSizeMB, layout.DiskSizeBytes, layout.FreeMB, layout.FreeSectors, partsDebug)

	ss := layoutSectorSize(&layout)
	spm := sectorsPerMiB(ss)

	var esp, sys, zeroSize int
	var newSectors int64
	// seenRemaining records that a brand-new "use all remaining space" partition
	// has appeared, so no further brand-new partition may follow it: there would
	// be nothing left to carve, and buildActions would place it past the disk.
	seenRemaining := false
	for _, p := range layout.Partitions {
		switch p.Type {
		case shared.TypeESP:
			esp++
		case shared.TypeSystem:
			sys++
		case "":
			// empty type = untouched (dualboot-safe)
		default:
			return fmt.Errorf("invalid partition type %q (want esp or system)", p.Type)
		}
		if p.SizeMB < 0 {
			return fmt.Errorf("partition size cannot be negative")
		}

		// Designated Boot/System partitions must be large enough to be usable. A
		// SizeMB of 0 means "use all remaining space", whose resulting size is
		// whatever is left on the disk, so it is exempt from the floor. Existing
		// rows carry their exact byte size; brand-new rows are whole MiB.
		if p.SizeMB != 0 {
			sizeBytes := p.SizeBytes
			if sizeBytes == 0 {
				sizeBytes = p.SizeMB * 1024 * 1024
			}
			switch p.Type {
			case shared.TypeESP:
				if sizeBytes < shared.MinESPSizeBytes {
					return fmt.Errorf("ESP partition is too small: %d MB (minimum %s required)", p.SizeMB, shared.FormatSize(shared.MinESPSizeBytes))
				}
			case shared.TypeSystem:
				if sizeBytes < shared.MinSystemPartitionBytes {
					return fmt.Errorf("system partition is too small: %d MB (minimum %s required)", p.SizeMB, shared.FormatSize(shared.MinSystemPartitionBytes))
				}
			}
		}

		// A SizeMB of 0 requests "use all remaining space"; at most one is allowed.
		// A real existing partition always carries its exact byte size, so only a
		// brand-new row (no Path, no SizeBytes) is a genuine request — an existing
		// partition smaller than 1 MiB is not.
		if p.SizeMB == 0 && p.SizeBytes == 0 {
			zeroSize++
		}

		isNew := p.Path == ""
		if isNew && seenRemaining {
			return fmt.Errorf("a partition cannot be added after one that uses all remaining space")
		}
		if isNew {
			if p.SizeMB != 0 {
				newSectors += p.SizeMB * spm
			} else {
				seenRemaining = true
			}
		}
	}

	if esp != 1 {
		return fmt.Errorf("exactly one ESP partition is required (found %d)", esp)
	}
	if sys != 1 {
		return fmt.Errorf("exactly one System partition is required (found %d)", sys)
	}
	if zeroSize > 1 {
		return fmt.Errorf("only one partition may use \"use all remaining space\"")
	}

	// Coarse guard, in MiB: the sum of every declared size must not exceed the
	// disk. Existing partitions are rounded down here, so this is lenient by
	// design — the strict, truncation-free check below is what prevents real
	// over-allocation of brand-new partitions.
	var sumMB int64
	for _, p := range layout.Partitions {
		sumMB += p.SizeMB
	}
	if sumMB > layout.DiskSizeMB {
		return fmt.Errorf("partition sizes (%d MB) exceed the disk size (%d MB)", sumMB, layout.DiskSizeMB)
	}

	// Strict guard, in sectors: brand-new explicit partitions must fit the free
	// space, with the leading GPT prefix and the trailing backup region excluded.
	//
	// Free space is recomputed from the live layout rather than taken from
	// layout.FreeSectors: that value is stamped by ComputeLayout at disk-load time
	// and is never updated once the user edits the plan. Deleting an existing row
	// and adding a brand-new one in its place must free real room for the new
	// partition, so the check has to measure free space against the partitions that
	// remain in the plan (existing ones kept verbatim). Existing partitions are
	// read by exact byte size when known so rounding them down to MiB can't
	// overstate how much room is left; the MiB fallback covers layouts without
	// byte geometry. Brand-new rows are skipped here because newSectors already
	// accounts for exactly the space they consume.
	var reservedSectors int64
	for _, p := range layout.Partitions {
		if p.Path == "" {
			continue
		}
		if p.SizeBytes > 0 {
			reservedSectors += p.SizeBytes / ss
		} else {
			reservedSectors += p.SizeMB * spm
		}
	}
	diskSectors := layout.DiskSizeBytes / ss
	if diskSectors == 0 {
		diskSectors = layout.DiskSizeMB * spm
	}
	// Usable space for new partitions is the disk minus the existing partitions,
	// minus the leading GPT prefix (partition placement starts at that same 1 MiB
	// boundary) and the trailing backup region. Subtracting the prefix here keeps
	// this totals-only check in step with the actual placement in buildActions, so
	// it can never admit a plan buildActions will reject.
	usableSectors := diskSectors - reservedSectors - prefixSectors(ss) - backupSectors(ss)
	if usableSectors < 0 {
		usableSectors = 0
	}
	// Only brand-new partitions can over-allocate free space, so this strict,
	// truncation-free check is skipped entirely when there are none. On a
	// fully-partitioned disk (all rows existing) the remaining free space is
	// often smaller than the GPT prefix/backup regions, and comparing
	// `0 > usableSectors` would then reject a valid untouched layout.
	if newSectors > 0 && newSectors > usableSectors {
		shared.GetLogger().Info("ValidateManualLayout: STRICT FAIL newSectors=%d usableSectors=%d gptPrefix=%d gptBackup=%d",
			newSectors, usableSectors, prefixSectors(ss), backupSectors(ss))
		return fmt.Errorf("new partition sizes (%d MB) exceed the free space (%d MB)",
			newSectors/spm, usableSectors/spm)
	}

	// A brand-new "use all remaining space" row (SizeMB 0) is exempt from the
	// per-row floor above because its final size is only known after allocation.
	// Enforce the floor on the resolved space now.
	if err := validateUseRemainingFloors(layout, usableSectors, newSectors, spm); err != nil {
		return err
	}

	shared.GetLogger().Info("ValidateManualLayout: PASS sumMB=%d diskMB=%d newSectors=%d usableSectors=%d",
		sumMB, layout.DiskSizeMB, newSectors, usableSectors)

	return nil
}

// validateUseRemainingFloors enforces the role floor on a brand-new "use all
// remaining space" row (SizeMB 0). Its final size is only known after explicit
// new partitions are subtracted from the usable space, so the per-row pass in
// Validate cannot check it.
func validateUseRemainingFloors(layout shared.ManualLayout, usableSectors, newSectors, spm int64) error {
	remainingSectors := usableSectors - newSectors
	if remainingSectors < 0 {
		remainingSectors = 0
	}
	remainingMB := remainingSectors / spm

	for _, p := range layout.Partitions {
		if p.Path != "" || p.SizeMB != 0 || (p.Type != shared.TypeESP && p.Type != shared.TypeSystem) {
			continue
		}
		if min := minPartitionSizeMB(p.Type); remainingMB < min {
			return fmt.Errorf("%s partition would have only %d MB of free space, below the %d MB minimum", p.Type, remainingMB, min)
		}
	}
	return nil
}

// ValidateFeasible runs Validate and, when the layout has brand-new partitions
// and the disk geometry can be read, additionally simulates the exact placement
// Apply performs. This catches fragmentation — free space that is large in
// total but split into gaps too small for a requested partition — which
// Validate's totals-only check cannot see. It performs no writes.
//
// A geometry read failure (e.g. an unprivileged caller that cannot open the
// block device) is not fatal: the coarse Validate result is returned, and Apply
// re-checks with the real geometry as root.
func ValidateFeasible(layout shared.ManualLayout) error {
	if err := Validate(layout); err != nil {
		return err
	}
	if layout.Disk == "" || !hasNewPartitions(layout) {
		return nil
	}
	// A layout from the CLI may omit the sector size; read it so the dry-run
	// placement uses the same unit sgdisk reports in.
	ensureSectorSize(&layout, layout.Disk)
	sectors, err := partitionSectors(layout.Disk)
	if err != nil {
		// Cannot read geometry here; Apply will re-check as root.
		return nil
	}
	return feasible(layout, sectors)
}

// feasible reports whether every brand-new partition in the layout fits the
// disk's current free regions. deleted partitions are treated as already
// removed (Apply deletes them before reading geometry), so freeing a partition
// and re-creating another in its place validates correctly.
func feasible(layout shared.ManualLayout, sectors []partitionSector) error {
	sectors = dropDeleted(sectors, layout.DeletedPartitions)
	_, err := buildActions(layout.Disk, &layout, shared.DefaultFileSystem, sectors)
	return err
}

// hasNewPartitions reports whether the layout contains a brand-new Boot/System
// partition that must be carved into free space.
func hasNewPartitions(layout shared.ManualLayout) bool {
	for _, p := range layout.Partitions {
		if p.Path == "" && (p.Type == shared.TypeESP || p.Type == shared.TypeSystem) {
			return true
		}
	}
	return false
}

// dropDeleted removes the partitions the user deleted from a sector list, so a
// dry-run placement sees the same freed space Apply produces after it runs
// `sgdisk -d` on them.
func dropDeleted(sectors []partitionSector, deleted []shared.ManualPartition) []partitionSector {
	if len(deleted) == 0 {
		return sectors
	}
	gone := make(map[int]bool, len(deleted))
	for _, p := range deleted {
		if n := parsePartitionNum(p.Path); n > 0 {
			gone[n] = true
		}
	}
	out := make([]partitionSector, 0, len(sectors))
	for _, s := range sectors {
		if !gone[s.num] {
			out = append(out, s)
		}
	}
	return out
}

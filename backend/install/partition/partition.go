// Package partition implements dualboot-safe manual partitioning.
//
// Unlike the automatic format step (which wipes the whole disk and creates an
// EFI + root layout), the manual partition step only touches the two
// partitions the user designates (one Boot, one Root). Every other partition
// on the disk is left exactly as-is, which is what makes a dualboot install
// safe.
package partition

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"aelum-installer/backend/install/format"
	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// gptBackupBytes is the trailing space a GPT reserves for its backup table and
// header. The real default is 33 logical sectors (a 32-entry table plus one
// header), but a generous 128 KiB is reserved so sgdisk never rejects a
// partition whose end sits just before the last usable sector. Expressed in
// bytes so it scales with the disk's logical sector size (256 sectors at
// 512 bytes/sector, 32 at 4096).
const gptBackupBytes = int64(128 * 1024)

// gptPrefixBytes is the leading space a GPT reserves for the protective MBR and
// the main partition table + header: the first usable 1 MiB, which no partition
// may occupy. Expressed in bytes so it scales with the logical sector size.
const gptPrefixBytes = int64(1024 * 1024)

// defaultSectorSize is the logical sector size assumed when a disk's real value
// is unknown. sgdisk numbers sectors in the disk's logical sector size, so
// every sector calculation must use the same unit (4Kn disks report 4096).
const defaultSectorSize = int64(512)

// sectorsPerMiB returns the number of logical sectors in one MiB for the given
// sector size (2048 at 512 bytes/sector, 256 at 4096).
func sectorsPerMiB(sectorSize int64) int64 {
	if sectorSize <= 0 {
		sectorSize = defaultSectorSize
	}
	return (1024 * 1024) / sectorSize
}

// prefixSectors returns the leading GPT reservation (bytes) in logical sectors.
func prefixSectors(sectorSize int64) int64 {
	if sectorSize <= 0 {
		sectorSize = defaultSectorSize
	}
	return gptPrefixBytes / sectorSize
}

// backupSectors returns the trailing GPT reservation (bytes) in logical sectors.
func backupSectors(sectorSize int64) int64 {
	if sectorSize <= 0 {
		sectorSize = defaultSectorSize
	}
	return gptBackupBytes / sectorSize
}

// layoutSectorSize returns the layout's recorded logical sector size, falling
// back to 512 when it is unset (layouts built by older callers or in tests).
func layoutSectorSize(layout *shared.ManualLayout) int64 {
	if layout.SectorSizeBytes > 0 {
		return layout.SectorSizeBytes
	}
	return defaultSectorSize
}

// ensureSectorSize records the disk's logical sector size on the layout when a
// caller (e.g. the CLI, or a layout built without geometry) left it unset.
func ensureSectorSize(layout *shared.ManualLayout, diskID string) {
	if layout.SectorSizeBytes <= 0 {
		layout.SectorSizeBytes = shared.LogicalSectorSize(diskID)
	}
}

// ComputeLayout returns a ManualLayout describing a disk's current partitions
// (all preset to Preserve) plus the derived free space. It reuses the same
// lsblk path as shared.GetDiskPartitions / shared.GetDisks.
func ComputeLayout(diskID string) (shared.ManualLayout, error) {
	if diskID == "" {
		return shared.ManualLayout{}, fmt.Errorf("disk ID not set")
	}

	// Disk size.
	disks, err := shared.GetDisks()
	if err != nil {
		return shared.ManualLayout{}, err
	}
	var diskSizeBytes int64
	found := false
	for _, d := range disks {
		if d.Path == diskID {
			diskSizeBytes = d.Size
			found = true
			break
		}
	}
	if !found {
		return shared.ManualLayout{}, fmt.Errorf("disk %s not found", diskID)
	}

	// Existing partitions.
	partitions, err := shared.GetDiskPartitions(diskID)
	if err != nil {
		return shared.ManualLayout{}, err
	}

	var layout shared.ManualLayout
	layout.Disk = diskID
	layout.DiskSizeMB = bytesToMB(diskSizeBytes)
	// Carry the exact byte/sector geometry so downstream free-space math can stay
	// truncation-free instead of relying on the rounded-down MiB values below.
	layout.DiskSizeBytes = diskSizeBytes
	// Record the disk's logical sector size so all downstream sector math uses
	// the same unit sgdisk reports in (512e vs 4Kn).
	layout.SectorSizeBytes = shared.LogicalSectorSize(diskID)
	var usedBytes int64
	for i, p := range partitions {
		sizeBytes := p.Size
		usedBytes += sizeBytes
		layout.Partitions = append(layout.Partitions, shared.ManualPartition{
			Index:          i,
			Path:           p.Path,
			SizeMB:         bytesToMB(sizeBytes),
			SizeBytes:      sizeBytes,
			ExistingFstype: p.Fstype,
			// Role is empty → untouched (dualboot-safe).
		})
	}

	freeBytes := diskSizeBytes - usedBytes
	if freeBytes < 0 {
		freeBytes = 0
	}
	layout.FreeMB = bytesToMB(freeBytes)
	layout.FreeSectors = freeBytes / layout.SectorSizeBytes
	return layout, nil
}

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
					return fmt.Errorf("ESP partition is too small: %d MB (minimum %d MB required)", p.SizeMB, shared.MinESPSizeBytes/(1024*1024))
				}
			case shared.TypeSystem:
				if sizeBytes < shared.MinSystemPartitionBytes {
					return fmt.Errorf("system partition is too small: %d MB (minimum %d MB required)", p.SizeMB, shared.MinSystemPartitionBytes/(1024*1024*1024))
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

	shared.GetLogger().Info("ValidateManualLayout: PASS sumMB=%d diskMB=%d newSectors=%d usableSectors=%d",
		sumMB, layout.DiskSizeMB, newSectors, usableSectors)

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

// Apply formats and prepares the Boot and Root partitions described by layout,
// then populates ctx.Partitions so the subsequent mount/install/configure steps
// work unchanged. Preserve partitions are never touched.
//
// This replaces format.Run on the manual install path.
func Apply(ctx *shared.InstallContext, layout *shared.ManualLayout) (retErr error) {
	diskID := layout.Disk
	if diskID == "" {
		diskID = ctx.DiskID
	}
	if diskID == "" {
		return fmt.Errorf("manual layout has no disk")
	}
	// A layout from the CLI may omit the sector size; read it so the placement
	// math uses the same sector unit sgdisk reports in (512e vs 4Kn).
	ensureSectorSize(layout, diskID)
	if err := Validate(*layout); err != nil {
		return err
	}

	defer func() {
		if retErr != nil {
			if ctx.Logger != nil {
				ctx.Logger.Error("manual partition apply failed, cleaning up: %v", retErr)
			}
			_ = primitives.CleanupMounts(ctx.Logger) //nolint:errcheck // cleanup best-effort on error path
		}
	}()

	// Locate the System row (needed only to confirm the layout has a System
	// partition before the btrfs subvolume step). The ESP path is resolved from
	// the format actions below.
	var sys shared.ManualPartition
	for _, p := range layout.Partitions {
		if p.Type == shared.TypeSystem {
			sys = p
		}
	}

	// Unmount anything left behind by a prior attempt.
	_ = primitives.CleanupMounts(ctx.Logger)

	// Remove any partitions the user deleted from the layout before reading the
	// disk geometry or creating new partitions. Deleting first frees their space
	// for new partitions and keeps the numbers of the surviving partitions valid.
	if err := deletePartitions(diskID, layout.DeletedPartitions); err != nil {
		return err
	}

	// Build the ordered list of format actions for Boot + Root only.
	sectors, err := partitionSectors(diskID)
	if err != nil {
		return err
	}
	actions, err := buildActions(diskID, layout, ctx.FileSystem, sectors)
	if err != nil {
		return err
	}

	// Resolve the actual device paths for the ESP and System partitions. For
	// brand-new partitions the layout rows carry an empty Path; buildActions
	// computed the real device path (e.g. /dev/vda1) in each formatAction, and
	// that is what format/blkid/mount must use downstream.
	var espPath, sysPath string
	for _, a := range actions {
		switch a.pType {
		case shared.TypeESP:
			espPath = a.path
		case shared.TypeSystem:
			sysPath = a.path
		}
	}

	// Create any brand-new partitions first.
	for _, a := range actions {
		if !a.isNew {
			continue
		}
		if err := createPartition(diskID, a); err != nil {
			return err
		}
	}

	// Ensure existing ESP/System partitions carry the correct GPT type GUID.
	for _, a := range actions {
		if a.isNew {
			continue
		}
		out, err := primitives.Run("sgdisk", "-t", fmt.Sprintf("%d:%s", a.num, a.guid), diskID).CombinedOutput()
		if err != nil {
			return fmt.Errorf("sgdisk set type for existing partition %d: %w: %s", a.num, err, string(out))
		}
	}

	// Refresh the kernel partition table.
	probeOut, probeErr := primitives.Run("partprobe", diskID).CombinedOutput()
	if probeErr != nil {
		return fmt.Errorf("partprobe: %w: %s", probeErr, string(probeOut))
	}

	// Format each Boot/Root partition. When LUKS is enabled the System
	// partition is encrypted (handled below via EncryptRoot), so only the ESP
	// is formatted here.
	for _, a := range actions {
		if a.pType == shared.TypeSystem && ctx.Luks {
			continue
		}
		label := "aelum"
		if a.fstype == shared.FileSystemVfat {
			label = "EFI"
		}
		if err := format.FormatPartition(a.path, a.fstype, label); err != nil {
			return err
		}
	}

	var mapperPath, mapperUUID, luksUUID string
	if ctx.Luks {
		if len(ctx.LuksPassphrase) == 0 {
			return fmt.Errorf("LUKS enabled but no passphrase set")
		}
		var err error
		mapperPath, mapperUUID, luksUUID, err = format.EncryptRoot(ctx, sysPath, string(ctx.LuksPassphrase), ctx.FileSystem)
		if err != nil {
			return err
		}
	} else if sys.Type == shared.TypeSystem && ctx.FileSystem == shared.FileSystemBtrfs {
		// btrfs root → automatic subvolumes.
		if err := format.SetupBtrfsSubvolumes(ctx, sysPath, shared.GetBtrfsSubvolumes()); err != nil {
			return err
		}
	}

	// Build PartitionInfo from the resolved EFI and System paths.
	partitions, err := format.BuildPartitionInfo(diskID, espPath, sysPath, ctx.FileSystem)
	if err != nil {
		return err
	}
	if ctx.Luks {
		// The root filesystem lives on the decrypted mapper, so fstab/bootloader
		// must reference the mapper path, its filesystem UUID, and the LUKS
		// header UUID.
		partitions.RootMapper = mapperPath
		partitions.LuksUUID = luksUUID
		partitions.RootUUID = mapperUUID
		for mp := range partitions.MountPoints {
			partitions.MountPoints[mp] = mapperPath
		}
	}
	ctx.Partitions = partitions

	return nil
}

// formatAction describes a single ESP or System partition to be formatted.
type formatAction struct {
	num         int    // GPT partition number
	path        string // device path
	startSector int64  // start sector (new partitions only)
	sizeMB      int64  // size in MB (new partitions only)
	guid        string // GPT type GUID
	fstype      string // vfat (ESP) or ctx.FileSystem (System)
	pType       shared.PartitionType
	isNew       bool
}

// buildActions turns the layout into the ordered Boot/Root format actions.
// Existing partitions keep their number; brand-new partitions are carved into
// the disk's free gaps (see allocateFree) and numbered after the last existing
// partition.
func buildActions(diskID string, layout *shared.ManualLayout, fileSystem string, sectors []partitionSector) ([]formatAction, error) {
	ss := layoutSectorSize(layout)
	spm := sectorsPerMiB(ss)

	// Brand-new partitions are numbered immediately after the highest existing one.
	maxNum := 0
	for _, s := range sectors {
		if s.num > maxNum {
			maxNum = s.num
		}
	}

	// lastSector caps placement before the trailing GPT backup.
	diskSectors := layout.DiskSizeBytes / ss
	if diskSectors == 0 {
		diskSectors = layout.DiskSizeMB * spm
	}
	lastSector := diskSectors - backupSectors(ss)

	free := freeRegions(sectors, prefixSectors(ss), lastSector)

	var actions []formatAction
	for _, p := range layout.Partitions {
		if p.Type != shared.TypeESP && p.Type != shared.TypeSystem {
			continue // empty type = untouched, never formatted or mounted
		}

		fstype := fileSystem
		guid := shared.GPTTypeGUIDs.LinuxRootX86_64
		if p.Type == shared.TypeESP {
			fstype = shared.FileSystemVfat
			guid = shared.GPTTypeGUIDs.ESP
		}

		if p.Path != "" {
			// Reuse an existing partition in place.
			num := parsePartitionNum(p.Path)
			if num > maxNum {
				maxNum = num
			}
			actions = append(actions, formatAction{
				num:    num,
				path:   p.Path,
				sizeMB: p.SizeMB,
				guid:   guid,
				fstype: fstype,
				pType:  p.Type,
			})
			continue
		}

		// Brand-new partition: next available number, placed in a free gap.
		num := maxNum + 1
		start, sizeMB, rest, ok := allocateFree(free, p.SizeMB, spm)
		if !ok {
			if p.SizeMB != 0 {
				return nil, fmt.Errorf("no single free region is large enough for the %s partition (%d MB); the disk's free space is fragmented", p.Type, p.SizeMB)
			}
			return nil, fmt.Errorf("no free space to create a %s partition", p.Type)
		}
		// allocateFree returns an exact fit for an explicit-size row (Validate
		// already enforced its floor), so this is a belt-and-suspenders guard; a
		// "use all remaining space" row is exempt because its floor is the disk.
		if p.SizeMB != 0 && sizeMB < minPartitionSizeMB(p.Type) {
			return nil, fmt.Errorf("%s partition has only %d MB of contiguous free space, below the %d MB minimum", p.Type, sizeMB, minPartitionSizeMB(p.Type))
		}
		free = rest
		actions = append(actions, formatAction{
			num:         num,
			path:        shared.DerivePartitionPath(diskID, num),
			startSector: start,
			sizeMB:      sizeMB,
			guid:        guid,
			fstype:      fstype,
			pType:       p.Type,
			isNew:       true,
		})
		maxNum = num
	}

	return actions, nil
}

// freeRegion is an inclusive sector range [start, end] of unallocated space.
type freeRegion struct {
	start int64
	end   int64
}

// freeRegions returns the disk's gaps as inclusive sector ranges (before the
// first partition, between neighbours, after the last). Allocation cannot begin
// before the leading GPT prefix (prefixSector, the first 1 MiB) nor extend past
// lastSector (the trailing backup region). Both are expressed in the disk's
// logical sectors.
func freeRegions(sectors []partitionSector, prefixSector, lastSector int64) []freeRegion {
	sorted := make([]partitionSector, len(sectors))
	copy(sorted, sectors)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].start < sorted[j].start })

	var regions []freeRegion
	cursor := prefixSector
	for _, s := range sorted {
		if s.start > cursor {
			regions = append(regions, freeRegion{cursor, min64(s.start-1, lastSector)})
		}
		if s.end+1 > cursor {
			cursor = s.end + 1
		}
	}
	if cursor <= lastSector {
		regions = append(regions, freeRegion{cursor, lastSector})
	}
	return regions
}

// removeSpan cuts the inclusive sector span [start, end] out of the regions,
// keeping any surviving prefix/suffix of a consumed region.
func removeSpan(regions []freeRegion, start, end int64) []freeRegion {
	out := make([]freeRegion, 0, len(regions))
	for _, r := range regions {
		if end < r.start || start > r.end {
			out = append(out, r) // no overlap
			continue
		}
		if r.start < start {
			out = append(out, freeRegion{r.start, start - 1})
		}
		if r.end > end {
			out = append(out, freeRegion{end + 1, r.end})
		}
	}
	return out
}

// allocateFree carves a brand-new partition of sizeMB MiB from the free regions,
// returning the 1-MiB-aligned start sector, the resolved size in MiB and the
// leftover regions. spm is the number of logical sectors per MiB (2048 at a
// 512-byte sector size, 256 at 4096), so alignment and sizing track the disk's
// real sector size.
//
//   - sizeMB == 0 ("use all remaining space") fills the largest free region. If
//     the free space is fragmented the result is the largest single gap, which
//     may be smaller than the total free space; the leftover gaps stay unused
//     because a partition must be contiguous.
//   - sizeMB > 0 requires a single region that fits the exact request. If none
//     does, ok is false: the caller must fail rather than silently create a
//     smaller partition than the user asked for.
func allocateFree(regions []freeRegion, sizeMB, spm int64) (start, size int64, rest []freeRegion, ok bool) {
	align := spm

	// Per-region candidate: aligned start and whole-MiB capacity.
	type cand struct {
		start int64
		avail int64
	}
	cands := make([]cand, 0, len(regions))
	for _, r := range regions {
		if r.start > r.end {
			continue
		}
		s := alignUp(r.start, align)
		avail := r.end - s + 1
		if avail < 0 {
			avail = 0
		}
		cands = append(cands, cand{start: s, avail: avail})
	}

	if sizeMB == 0 {
		// "Use all remaining space": fill the largest region.
		best := -1
		for i, c := range cands {
			if best < 0 || c.avail > cands[best].avail {
				best = i
			}
		}
		if best < 0 {
			return 0, 0, regions, false
		}
		c := cands[best]
		sz := c.avail / align
		if sz == 0 {
			return 0, 0, regions, false
		}
		return c.start, sz, removeSpan(regions, c.start, c.start+sz*align-1), true
	}

	// Explicit size: take the first region (by address) that fits the request
	// exactly. No clamping — a fragmented disk must fail loudly instead of
	// installing a smaller partition than requested.
	for _, c := range cands {
		if c.avail >= sizeMB*align {
			return c.start, sizeMB, removeSpan(regions, c.start, c.start+sizeMB*align-1), true
		}
	}
	return 0, 0, regions, false
}

// min64 returns the smaller of a and b.
func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// createPartition creates a new GPT partition with the correct size and type
// GUID using sgdisk. The disk path is passed as sgdisk's final argument — it is
// required: without it sgdisk succeeds silently and creates no partition.
func createPartition(diskID string, a formatAction) error {
	args := append([]string{"sgdisk"}, []string{
		"-n", fmt.Sprintf("%d:%d:+%dM", a.num, a.startSector, a.sizeMB),
		"-t", fmt.Sprintf("%d:%s", a.num, a.guid),
		"-c", fmt.Sprintf("%d:%s", a.num, roleLabel(a.fstype)),
		diskID,
	}...)
	out, err := primitives.Run(args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("sgdisk create partition %d: %w: %s", a.num, err, string(out))
	}
	return nil
}

// deletePartitions removes the given partitions from the disk with `sgdisk -d`,
// highest partition number first so the numbers of the surviving partitions are
// left untouched, then refreshes the kernel's view of the partition table.
func deletePartitions(diskID string, deleted []shared.ManualPartition) error {
	nums := deleteOrder(deleted)
	for _, num := range nums {
		out, err := primitives.Run("sgdisk", "-d", strconv.Itoa(num), diskID).CombinedOutput()
		if err != nil {
			return fmt.Errorf("sgdisk delete partition %d: %w: %s", num, err, string(out))
		}
	}

	probeOut, probeErr := primitives.Run("partprobe", diskID).CombinedOutput()
	if probeErr != nil {
		return fmt.Errorf("partprobe: %w: %s", probeErr, string(probeOut))
	}
	return nil
}

// deleteOrder extracts the partition numbers to delete and returns them
// highest-first. Deleting in that order leaves the numbers of the surviving
// partitions unchanged (sgdisk does not renumber after a deletion, but going
// high-to-low is still the safe, deterministic order).
func deleteOrder(deleted []shared.ManualPartition) []int {
	nums := make([]int, 0, len(deleted))
	for _, p := range deleted {
		if n := parsePartitionNum(p.Path); n > 0 {
			nums = append(nums, n)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(nums)))
	return nums
}

// partitionSector is a parsed start/end sector pair for one existing partition.
type partitionSector struct {
	num   int
	start int64
	end   int64
}

// partitionSectors parses `sgdisk -p` to find the start/end sector of each
// existing partition on the disk.
//
// sgdisk prints a whitespace-aligned table whose first three columns are the
// partition number, start sector, and end sector:
//
//	Number  Start (sector)    End (sector)  Size       Code  Name
//	   1            2048         2099199   1024.0 MiB  EF00  EFI
//
// A row is recognised only when its first three fields are integers, which
// naturally skips the header line (starts with "Number") and any free-space
// summary (no leading partition number).
func partitionSectors(diskID string) ([]partitionSector, error) {
	out, err := primitives.Run("sgdisk", "-p", diskID).Output()
	if err != nil {
		return nil, fmt.Errorf("sgdisk -p: %w", err)
	}

	return parsePartitionSectors(string(out)), nil
}

// parsePartitionSectors extracts the start/end sector of every partition from
// the textual output of `sgdisk -p`.
func parsePartitionSectors(out string) []partitionSector {
	var result []partitionSector
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		num, err1 := strconv.Atoi(fields[0])
		start, err2 := strconv.ParseInt(fields[1], 10, 64)
		end, err3 := strconv.ParseInt(fields[2], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		result = append(result, partitionSector{num: num, start: start, end: end})
	}
	return result
}

// parsePartitionNum extracts the partition number from a device path, handling
// both /dev/sdaN and every /dev/nvmeNdNpN convention (nvme0n1p12 -> 12).
func parsePartitionNum(path string) int {
	// NVMe: /dev/nvme<n>n<m>p<part> — the partition is the trailing number after "p".
	if m := nvmePartitionRe.FindStringSubmatch(path); m != nil {
		num, _ := strconv.Atoi(m[1])
		return num
	}
	// Everything else (sd, vd, md, ...): the trailing digit run is the number.
	digits := trailingDigitRe.FindString(path)
	if digits == "" {
		return 0
	}
	num, _ := strconv.Atoi(digits)
	return num
}

var (
	// nvmePartitionRe matches the trailing partition number of an NVMe path,
	// e.g. "/dev/nvme3n1p12" -> "12" (any controller/disk index).
	nvmePartitionRe = regexp.MustCompile(`nvme\d+n\d+p(\d+)$`)
	// trailingDigitRe matches the trailing digit run of conventional names,
	// e.g. "/dev/sda21" -> "21".
	trailingDigitRe = regexp.MustCompile(`\d+$`)
)

// alignUp rounds sector up to the nearest multiple of align. align is one MiB
// worth of the disk's logical sectors (typically 2048).
func alignUp(sector, align int64) int64 {
	return ((sector + align - 1) / align) * align
}

func roleLabel(fstype string) string {
	if fstype == shared.FileSystemVfat {
		return "EFI"
	}
	return "aelum"
}

// minPartitionSizeMB returns the enforced minimum size in MiB for a role: the
// ESP floor (128 MiB) and the system/root floor (8 GiB). Other roles are not
// designated partitions and have no floor (0). Mirrors the size checks in
// Validate; here it backstops the *resolved* size of a brand-new partition so a
// too-small partition can never reach sgdisk.
func minPartitionSizeMB(pt shared.PartitionType) int64 {
	switch pt {
	case shared.TypeESP:
		return shared.MinESPSizeBytes / (1024 * 1024)
	case shared.TypeSystem:
		return shared.MinSystemPartitionBytes / (1024 * 1024)
	default:
		return 0
	}
}

func bytesToMB(bytes int64) int64 {
	if bytes <= 0 {
		return 0
	}
	return bytes / (1024 * 1024)
}

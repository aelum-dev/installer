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

// gptBackupSectors is the number of trailing sectors a GPT reserves for its
// backup partition table at the very end of the disk (256 sectors by default).
// The "use all remaining space" computation reserves this room so sgdisk does
// not reject a partition whose end would extend past the last usable sector.
const gptBackupSectors = int64(256)

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
			diskSizeBytes, _ = parseBytes(d.Size)
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
	var usedBytes int64
	for i, p := range partitions {
		sizeBytes, _ := parseBytes(p.Size)
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
	layout.FreeSectors = freeBytes / 512
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
		if p.SizeMB == 0 {
			zeroSize++
		}

		isNew := p.Path == ""
		if isNew && seenRemaining {
			return fmt.Errorf("a partition cannot be added after one that uses all remaining space")
		}
		if isNew {
			if p.SizeMB != 0 {
				newSectors += p.SizeMB * 2048
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
	// Coarse guard, in MiB: the sum of every declared size must not exceed the
	// disk. Existing partitions are rounded down here, so this is lenient by
	// design — the strict, truncation-free check below is what prevents real
	// over-allocation of brand-new partitions.
	if sumMB > layout.DiskSizeMB {
		return fmt.Errorf("partition sizes (%d MB) exceed the disk size (%d MB)", sumMB, layout.DiskSizeMB)
	}

	// Strict guard, in sectors: brand-new explicit partitions must fit the free
	// space, with the trailing gptBackupSectors kept for the backup GPT header.
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
			reservedSectors += p.SizeBytes / 512
		} else {
			reservedSectors += p.SizeMB * 2048
		}
	}
	diskSectors := layout.DiskSizeBytes / 512
	if diskSectors == 0 {
		diskSectors = layout.DiskSizeMB * 2048
	}
	freeSectors := diskSectors - reservedSectors
	if freeSectors < 0 {
		freeSectors = 0
	}
	// Only brand-new partitions can over-allocate free space, so this strict,
	// truncation-free check is skipped entirely when there are none. On a
	// fully-partitioned disk (all rows existing) the remaining free space is
	// often smaller than the trailing GPT backup region, and comparing
	// `0 > freeSectors-gptBackupSectors` would then be true and reject a valid
	// untouched layout.
	if newSectors > 0 && newSectors > freeSectors-gptBackupSectors {
		shared.GetLogger().Info("ValidateManualLayout: STRICT FAIL newSectors=%d freeSectors=%d gptBackup=%d allowedMB=%d availableAddMB-style",
			newSectors, freeSectors, gptBackupSectors, (freeSectors-gptBackupSectors)/2048)
		return fmt.Errorf("new partition sizes (%d MB) exceed the free space (%d MB)",
			newSectors/2048, (freeSectors-gptBackupSectors)/2048)
	}

	shared.GetLogger().Info("ValidateManualLayout: PASS sumMB=%d diskMB=%d newSectors=%d freeSectors=%d",
		sumMB, layout.DiskSizeMB, newSectors, freeSectors)

	return nil
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

	// Format each Boot/Root partition.
	for _, a := range actions {
		label := "aelum"
		if a.fstype == shared.FileSystemVfat {
			label = "EFI"
		}
		if err := format.FormatPartition(a.path, a.fstype, label); err != nil {
			return err
		}
	}

	// btrfs root → automatic subvolumes.
	if sys.Type == shared.TypeSystem && ctx.FileSystem == shared.FileSystemBtrfs {
		if err := format.SetupBtrfsSubvolumes(ctx, sysPath, shared.GetBtrfsSubvolumes()); err != nil {
			return err
		}
	}

	// Discover UUIDs and populate ctx.Partitions, keyed by type.
	espUUID, err := readUUID(espPath)
	if err != nil {
		return fmt.Errorf("blkid %s: %w", espPath, err)
	}
	rootUUID, err := readUUID(sysPath)
	if err != nil {
		return fmt.Errorf("blkid %s: %w", sysPath, err)
	}

	partitions := &shared.PartitionInfo{
		Disk:        diskID,
		EFIPart:     espPath,
		EFIUUID:     espUUID,
		RootPart:    sysPath,
		RootUUID:    rootUUID,
		MountPoints: make(map[string]string),
	}
	if ctx.FileSystem == shared.FileSystemBtrfs {
		for _, sv := range shared.GetBtrfsSubvolumes() {
			if sv.MountPoint != "/" {
				partitions.MountPoints[sv.MountPoint] = sys.Path
			}
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
// Existing partitions keep their current number; new partitions are assigned
// the next available numbers and placed after the last existing partition.
func buildActions(diskID string, layout *shared.ManualLayout, fileSystem string, sectors []partitionSector) ([]formatAction, error) {
	maxNum := 0
	maxEnd := int64(shared.GPTPartitionStartSector)
	for _, s := range sectors {
		if s.num > maxNum {
			maxNum = s.num
		}
		// s.end is the inclusive end sector; the next free sector is s.end + 1.
		if next := s.end + 1; next > maxEnd {
			maxEnd = next
		}
	}

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
			if next := endSectorFor(sectors, num) + 1; next > maxEnd {
				maxEnd = next
			}
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

		// Carve a brand-new partition from free space.
		// maxEnd is already the next free sector, so align it directly. Adding 1
		// here would leave a spurious 1 MiB gap whenever the previous partition
		// ends exactly on a 1 MiB boundary (the common case), so new partitions
		// sit edge-to-edge instead.
		num := maxNum + 1
		start := alignUp(maxEnd, 2048)

		// lastSector is the last sector a partition may occupy: the disk's true
		// sector count minus the trailing sectors reserved for the backup GPT, so
		// sgdisk won't reject a partition past the last usable sector. DiskSizeMB
		// is truncated to MiB, so fall back to it when the layout carries no byte
		// geometry (e.g. hand-built test layouts).
		diskSectors := layout.DiskSizeBytes / 512
		if diskSectors == 0 {
			diskSectors = layout.DiskSizeMB * 2048
		}
		lastSector := diskSectors - gptBackupSectors

		sizeMB := p.SizeMB
		if sizeMB < 0 {
			sizeMB = 0
		}
		if sizeMB == 0 {
			// Consume all remaining free space, capped so the partition ends within
			// the disk's usable region. Reserve the trailing gptBackupSectors for
			// the backup GPT; without this sgdisk rejects a partition whose end
			// extends past the last usable sector.
			available := lastSector - start
			if available < 0 {
				available = 0
			}
			sizeMB = available / 2048
		} else {
			// Defensive clamp: an explicit size must never run past the usable end
			// of the disk. Free space is measured in sectors (not the MiB shown in
			// the UI), so rounding existing partitions down to MiB can't let an
			// explicit partition overflow. Validate already rejects clearly
			// oversized layouts; this is the backstop at creation time.
			available := lastSector - start
			if available < 0 {
				available = 0
			}
			if sectors := sizeMB * 2048; sectors > available {
				sizeMB = available / 2048
			}
		}
		end := start + (sizeMB*1024*1024)/512
		if end > maxEnd {
			maxEnd = end
		}
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

func endSectorFor(sectors []partitionSector, num int) int64 {
	for _, s := range sectors {
		if s.num == num {
			return s.end
		}
	}
	return 0
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

// alignUp rounds sector up to the nearest multiple of align (1 MiB = 2048).
func alignUp(sector, align int64) int64 {
	return ((sector + align - 1) / align) * align
}

func roleLabel(fstype string) string {
	if fstype == shared.FileSystemVfat {
		return "EFI"
	}
	return "aelum"
}

func readUUID(partition string) (string, error) {
	cmd := primitives.Run("blkid", "-s", "UUID", "-o", "value", partition)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func parseBytes(s string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
}

func bytesToMB(bytes int64) int64 {
	if bytes <= 0 {
		return 0
	}
	return bytes / (1024 * 1024)
}

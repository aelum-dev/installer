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
	var usedBytes int64
	for i, p := range partitions {
		sizeBytes, _ := parseBytes(p.Size)
		usedBytes += sizeBytes
		layout.Partitions = append(layout.Partitions, shared.ManualPartition{
			Index:          i,
			Path:           p.Path,
			SizeMB:         bytesToMB(sizeBytes),
			ExistingFstype: p.Fstype,
			// Role is empty → untouched (dualboot-safe).
		})
	}

	freeBytes := diskSizeBytes - usedBytes
	if freeBytes < 0 {
		freeBytes = 0
	}
	layout.FreeMB = bytesToMB(freeBytes)
	return layout, nil
}

// Validate checks a ManualLayout for the invariants required to install:
//   - exactly one ESP and exactly one System partition
//   - the sum of partition sizes must fit the disk
//   - at most one partition may request "use all remaining space" (SizeMB == 0)
//   - every type must be a known value
func Validate(layout shared.ManualLayout) error {
	var esp, sys, zeroSize int
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
		if p.SizeMB == 0 {
			zeroSize++
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

	var sumMB int64
	for _, p := range layout.Partitions {
		if p.SizeMB < 0 {
			return fmt.Errorf("partition size cannot be negative")
		}
		sumMB += p.SizeMB
	}

	// Σsizes ≤ diskSize, all in MiB.
	if sumMB > layout.DiskSizeMB {
		return fmt.Errorf("partition sizes (%d MB) exceed the disk size (%d MB)", sumMB, layout.DiskSizeMB)
	}

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
		if s.end > maxEnd {
			maxEnd = s.end
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
			if end := endSectorFor(sectors, num); end > maxEnd {
				maxEnd = end
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
		num := maxNum + 1
		start := alignUp(maxEnd+1, 2048)
		sizeMB := p.SizeMB
		if sizeMB == 0 {
			// Consume all remaining free space, capped so the partition ends
			// within the disk's usable region. DiskSizeMB is in MiB, so the disk
			// holds DiskSizeMB*2048 sectors; reserve the trailing gptBackupSectors
			// for the backup GPT. Without this, sgdisk rejects a partition whose
			// end extends past the last usable sector (the backup GPT needs room
			// at the very end of the disk).
			available := layout.DiskSizeMB*2048 - gptBackupSectors - start
			if available < 2048 {
				available = 2048 // at least 1 MiB
			}
			sizeMB = available / 2048
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

var partitionSectorRe = regexp.MustCompile(`Partition (\d+) is start sector (\d+) and end sector (\d+)`)

// partitionSectors parses `sgdisk -p` to find the start/end sector of each
// existing partition on the disk.
func partitionSectors(diskID string) ([]partitionSector, error) {
	out, err := primitives.Run("sgdisk", "-p", diskID).Output()
	if err != nil {
		return nil, fmt.Errorf("sgdisk -p: %w", err)
	}

	var result []partitionSector
	for _, line := range strings.Split(string(out), "\n") {
		m := partitionSectorRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		num, _ := strconv.Atoi(m[1])
		start, _ := strconv.ParseInt(m[2], 10, 64)
		end, _ := strconv.ParseInt(m[3], 10, 64)
		result = append(result, partitionSector{num: num, start: start, end: end})
	}
	return result, nil
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

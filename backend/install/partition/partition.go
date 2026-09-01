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
	"strconv"
	"strings"

	"aelum-installer/backend/install/format"
	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// GPTOverheadBytes is the ~1 MiB of disk reserved for GPT metadata. Exposed for
// tests that check free-space math.
const GPTOverheadBytes = int64(shared.GPTOverheadMB * 1024 * 1024)

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

	freeBytes := diskSizeBytes - usedBytes - GPTOverheadBytes
	if freeBytes < 0 {
		freeBytes = 0
	}
	layout.FreeMB = bytesToMB(freeBytes)
	return layout, nil
}

// Validate checks a ManualLayout for the invariants required to install:
//   - exactly one Boot and exactly one Root partition
//   - the sum of partition sizes plus GPT overhead must fit the disk
//   - at most one partition may request "use all remaining space" (SizeMB == 0)
//   - every role must be a known value
func Validate(layout shared.ManualLayout) error {
	var boot, root, zeroSize int
	for _, p := range layout.Partitions {
		switch p.Role {
		case shared.RoleBoot:
			boot++
		case shared.RoleRoot:
			root++
		case "":
			// empty role = untouched (dualboot-safe)
		default:
			return fmt.Errorf("invalid partition role %q (want boot or root)", p.Role)
		}
		if p.SizeMB == 0 {
			zeroSize++
		}
	}

	if boot != 1 {
		return fmt.Errorf("exactly one Boot partition is required (found %d)", boot)
	}
	if root != 1 {
		return fmt.Errorf("exactly one Root partition is required (found %d)", root)
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

	// Σsizes + 1 MiB GPT overhead ≤ diskSize, all in MiB.
	if sumMB+shared.GPTOverheadMB > layout.DiskSizeMB {
		return fmt.Errorf("partition sizes (%d MB) exceed the disk size (%d MB) minus GPT overhead", sumMB, layout.DiskSizeMB)
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

	// Locate the Boot and Root rows.
	var boot, root shared.ManualPartition
	for _, p := range layout.Partitions {
		switch p.Role {
		case shared.RoleBoot:
			boot = p
		case shared.RoleRoot:
			root = p
		}
	}

	// Unmount anything left behind by a prior attempt.
	_ = primitives.CleanupMounts(ctx.Logger)

	// Build the ordered list of format actions for Boot + Root only.
	sectors, err := partitionSectors(diskID)
	if err != nil {
		return err
	}
	actions, err := buildActions(diskID, layout, ctx.FileSystem, sectors)
	if err != nil {
		return err
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

	// Ensure existing Boot/Root partitions carry the correct GPT type GUID.
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
	if root.Role == shared.RoleRoot && ctx.FileSystem == shared.FileSystemBtrfs {
		if err := format.SetupBtrfsSubvolumes(ctx, root.Path, shared.GetBtrfsSubvolumes()); err != nil {
			return err
		}
	}

	// Discover UUIDs and populate ctx.Partitions, keyed by role.
	bootUUID, err := readUUID(boot.Path)
	if err != nil {
		return fmt.Errorf("blkid %s: %w", boot.Path, err)
	}
	rootUUID, err := readUUID(root.Path)
	if err != nil {
		return fmt.Errorf("blkid %s: %w", root.Path, err)
	}

	partitions := &shared.PartitionInfo{
		Disk:        diskID,
		EFIPart:     boot.Path,
		EFIUUID:     bootUUID,
		RootPart:    root.Path,
		RootUUID:    rootUUID,
		MountPoints: make(map[string]string),
	}
	if ctx.FileSystem == shared.FileSystemBtrfs {
		for _, sv := range shared.GetBtrfsSubvolumes() {
			if sv.MountPoint != "/" {
				partitions.MountPoints[sv.MountPoint] = root.Path
			}
		}
	}
	ctx.Partitions = partitions

	return nil
}

// formatAction describes a single Boot or Root partition to be formatted.
type formatAction struct {
	num         int    // GPT partition number
	path        string // device path
	startSector int64  // start sector (new partitions only)
	sizeMB      int64  // size in MB (new partitions only)
	guid        string // GPT type GUID
	fstype      string // vfat (Boot) or ctx.FileSystem (Root)
	role        shared.PartitionRole
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
		if p.Role != shared.RoleBoot && p.Role != shared.RoleRoot {
			continue // empty role = untouched, never formatted or mounted
		}

		fstype := fileSystem
		guid := shared.GPTTypeGUIDs.LinuxRootX86_64
		if p.Role == shared.RoleBoot {
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
				role:   p.Role,
			})
			continue
		}

		// Carve a brand-new partition from free space.
		num := maxNum + 1
		start := alignUp(maxEnd+1, 2048)
		sizeMB := p.SizeMB
		if sizeMB == 0 {
			// Consume all remaining free space.
			var sumOther int64
			for _, o := range layout.Partitions {
				if o.Index != p.Index {
					sumOther += o.SizeMB
				}
			}
			sizeMB = layout.DiskSizeMB - sumOther - shared.GPTOverheadMB
			if sizeMB < 1 {
				sizeMB = 1
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
			role:        p.Role,
			isNew:       true,
		})
		maxNum = num
	}

	return actions, nil
}

// createPartition creates a new GPT partition with the correct size and type
// GUID using sgdisk.
func createPartition(diskID string, a formatAction) error {
	args := []string{
		"-n", fmt.Sprintf("%d:%d:+%dM", a.num, a.startSector, a.sizeMB),
		"-t", fmt.Sprintf("%d:%s", a.num, a.guid),
		"-c", fmt.Sprintf("%d:%s", a.num, roleLabel(a.fstype)),
	}
	args = append([]string{"sgdisk"}, args...)
	out, err := primitives.Run(args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("sgdisk create partition %d: %w: %s", a.num, err, string(out))
	}
	return nil
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
// both /dev/sdaN and /dev/nvme0n1pN conventions.
func parsePartitionNum(path string) int {
	short := strings.TrimPrefix(path, "/dev/")
	short = strings.TrimPrefix(short, "nvme0n1p")
	short = strings.TrimPrefix(short, "nvme1n1p")
	// Fallback: strip everything but trailing digits.
	digits := regexp.MustCompile(`\d+`).FindAllString(short, -1)
	if len(digits) == 0 {
		return 0
	}
	num, _ := strconv.Atoi(digits[len(digits)-1])
	return num
}

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

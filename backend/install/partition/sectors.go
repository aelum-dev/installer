package partition

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// gptBackupBytes is the trailing space a GPT reserves for its backup table and
// header. Sourced from shared so the frontend's free-space math (constants.ts)
// and this package can never disagree. See shared.GPTBackupBytes for the
// rationale behind the 128 KiB value.
const gptBackupBytes int64 = shared.GPTBackupBytes

// gptPrefixBytes is the leading space a GPT reserves for the protective MBR and
// the main partition table + header: the first usable 1 MiB, which no partition
// may occupy. Sourced from shared so it stays in sync with the frontend.
const gptPrefixBytes int64 = shared.GPTPrefixBytes

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

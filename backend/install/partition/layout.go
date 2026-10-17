package partition

import (
	"fmt"
	"sort"

	"aelum-installer/backend/shared"
)

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
		} else if layout.Lvm {
			// The System partition becomes an LVM physical volume.
			guid = shared.GPTTypeGUIDs.LinuxLVM
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
		// Enforce the role floor on the *resolved* size. This matters for a
		// "use all remaining space" row (SizeMB 0): Validate cannot know its
		// final size, so without this check a tiny ESP or sub-8-GiB root could
		// be created from whatever little free space is left.
		if sizeMB < minPartitionSizeMB(p.Type) {
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

func roleLabel(fstype string) string {
	if fstype == shared.FileSystemVfat {
		return "EFI"
	}
	return "aelum"
}

// minPartitionSizeMB returns the enforced minimum size in MiB for a role: the
// ESP floor (512 MiB) and the system/root floor (8 GiB). Other roles are not
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

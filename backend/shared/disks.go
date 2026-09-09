package shared

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// PartitionEntry represents a single partition on a disk.
// Used both internally (format discovery) and by the frontend (display).
type PartitionEntry struct {
	Path        string   `json:"path"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Size        int64    `json:"size"` // raw byte count
	Fstype      string   `json:"fstype"`
	UUID        string   `json:"uuid"`
	Mountpoints []string `json:"mountpoints"` // expanded for btrfs, single-element slice otherwise
}

// GetPartitions returns the partitions of a given disk by querying lsblk.
func GetPartitions(diskPath string) ([]PartitionEntry, error) {
	if !strings.HasPrefix(diskPath, "/dev/") {
		diskPath = "/dev/" + diskPath
	}

	cmd := exec.Command("lsblk", "-b", "-J", "-o", "NAME,TYPE,SIZE,FSTYPE,UUID,TRAN", diskPath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run lsblk for %s: %w", diskPath, err)
	}

	var result struct {
		Blockdevices []lsblkEntry `json:"blockdevices"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse lsblk output: %w", err)
	}

	if len(result.Blockdevices) == 0 {
		return nil, fmt.Errorf("no block devices found for %s", diskPath)
	}

	var partitions []PartitionEntry
	for _, child := range result.Blockdevices[0].Children {
		if child.Type != "part" {
			continue
		}
		partitions = append(partitions, PartitionEntry{
			Path:   "/dev/" + child.Name,
			Name:   child.Name,
			Type:   child.Type,
			Size:   child.Size.Int64(),
			Fstype: child.Fstype,
			UUID:   child.UUID,
		})
	}

	return partitions, nil
}

type jsonSize struct{ Value string }

func (j *jsonSize) UnmarshalJSON(data []byte) error {
	if len(data) >= 2 && data[0] == '"' && data[len(data)-1] == '"' {
		j.Value = strings.Trim(string(data), `"`)
		return nil
	}
	j.Value = string(data)
	return nil
}

// Int64 parses the size value as an int64. Returns 0 if the value is empty or
// cannot be parsed — callers should check Size > 0 before using the result.
func (j *jsonSize) Int64() int64 {
	var n int64
	fmt.Sscanf(j.Value, "%d", &n) //nolint:errcheck // lsblk always produces valid numbers
	return n
}

type jsonBool bool

func (j *jsonBool) UnmarshalJSON(data []byte) error {
	s := string(data)
	switch s {
	case "true", `"1"`:
		*j = true
	case "false", `"0"`, "null":
		*j = false
	default:
		*j = false
	}
	return nil
}

type lsblkEntry struct {
	Name       string       `json:"name"`
	Type       string       `json:"type"`
	Size       jsonSize     `json:"size"`
	Model      string       `json:"model"`
	Vendor     string       `json:"vendor"`
	Rota       jsonBool     `json:"rota"`
	Ro         jsonBool     `json:"ro"`
	Fstype     string       `json:"fstype"`
	Mountpoint string       `json:"mountpoint"`
	Serial     string       `json:"serial"`
	Trans      string       `json:"tran"`
	UUID       string       `json:"uuid"`
	Children   []lsblkEntry `json:"children"`
}

// fakeDisksEnabled can be set to true to force fake disk data (e.g. when
// running in dry-run mode without sudo access). Also checked: FAKE_DISKS=1
// env var for manual testing.
var fakeDisksEnabled bool

// SetFakeDisks enables or disables fake disk data.
func SetFakeDisks(enabled bool) {
	fakeDisksEnabled = enabled
}

// GetDisks returns a list of available block devices (disks) for installation.
func GetDisks() ([]DiskInfo, error) {
	if fakeDisksEnabled || os.Getenv("FAKE_DISKS") == "1" {
		disks := fakeDisks()
		// Derive fstype from fake partitions for disks that don't have it set.
		for i := range disks {
			if disks[i].Fstype == "" {
				disks[i].Fstype = deriveFstypeFromFakePartitions(disks[i].Path)
			}
		}
		return disks, nil
	}
	cmd := exec.Command("lsblk", "-b", "-d", "-o", "NAME,TYPE,SIZE,MODEL,VENDOR,ROTA,RO,FSTYPE,MOUNTPOINT,SERIAL,TRAN", "-J")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run lsblk: %w", err)
	}

	var result struct {
		Blockdevices []lsblkEntry `json:"blockdevices"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse lsblk output: %w", err)
	}

	disks := []DiskInfo{}
	for _, bd := range result.Blockdevices {
		if bd.Type != "disk" {
			continue
		}
		if isVirtualDevice(bd.Name) {
			continue
		}

		disk := DiskInfo{
			Name:          bd.Name,
			Path:          "/dev/" + bd.Name,
			Size:          bd.Size.Int64(),
			SizeFormatted: FormatSize(bd.Size.Int64()),
			Type:          bd.Type,
			Model:         strings.TrimSpace(bd.Model),
			Vendor:        strings.TrimSpace(bd.Vendor),
			Rotational:    bool(bd.Rota),
			Readonly:      bool(bd.Ro),
			Fstype:        bd.Fstype,
			Mountpoint:    bd.Mountpoint,
			Serial:        strings.TrimSpace(bd.Serial),
			Trans:         bd.Trans,
			IsRemovable:   isRemovable(bd.Trans),
		}

		if disk.Model == "" && disk.Vendor != "" {
			disk.Model = disk.Vendor
		}
		if disk.Model == "" {
			disk.Model = "Unknown"
		}

		// Inherit fstype from child partitions if not set at disk level.
		// This ensures the "Existing ext4/btrfs" tag appears on page load
		// without requiring the user to click the disk card first.
		if disk.Fstype == "" {
			disk.Fstype = deriveFstypeFromChildren(bd.Children)
		}

		disks = append(disks, disk)
	}

	return disks, nil
}

func isVirtualDevice(name string) bool {
	short := strings.TrimPrefix(name, "/dev/")
	for _, prefix := range VirtualDevicePrefixes {
		if strings.HasPrefix(short, prefix) {
			return true
		}
	}
	return false
}

func isRemovable(trans string) bool {
	for _, t := range RemovableTransportTypes {
		if t == trans {
			return true
		}
	}
	return false
}

// deriveFstypeFromFakePartitions derives fstype from fake partition data.
// Uses the same priority: root mount > btrfs/ext4 > first with fstype.
func deriveFstypeFromFakePartitions(diskPath string) string {
	parts := fakePartitions(diskPath)
	// 1. Partition mounted at /
	for _, p := range parts {
		if p.Mountpoints != nil && contains(p.Mountpoints, "/") && p.Fstype != "" {
			return p.Fstype
		}
	}
	// 2. First btrfs or ext4 partition
	for _, p := range parts {
		if p.Fstype == "btrfs" || p.Fstype == "ext4" {
			return p.Fstype
		}
	}
	// 3. First partition with any fstype
	for _, p := range parts {
		if p.Fstype != "" {
			return p.Fstype
		}
	}
	return ""
}

// contains checks if a string slice contains a value.
func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

// deriveFstypeFromChildren inherits fstype from a disk's child partitions
// using the same priority as the frontend: root mount > btrfs/ext4 > first with fstype.
func deriveFstypeFromChildren(children []lsblkEntry) string {
	// 1. Partition mounted at /
	for _, c := range children {
		if c.Type == "part" && c.Mountpoint == "/" && c.Fstype != "" {
			return c.Fstype
		}
	}
	// 2. First btrfs or ext4 partition
	for _, c := range children {
		if c.Type == "part" && (c.Fstype == "btrfs" || c.Fstype == "ext4") {
			return c.Fstype
		}
	}
	// 3. First partition with any fstype
	for _, c := range children {
		if c.Type == "part" && c.Fstype != "" {
			return c.Fstype
		}
	}
	return ""
}

// GetDiskPartitions returns partition information for a disk using lsblk.
// For btrfs partitions, Mountpoints is expanded via findmnt; for others it
// contains a single element (the direct mountpoint).
func GetDiskPartitions(diskPath string) ([]PartitionEntry, error) {
	if fakeDisksEnabled || os.Getenv("FAKE_DISKS") == "1" {
		return fakePartitions(diskPath), nil
	}
	if !strings.HasPrefix(diskPath, "/dev/") {
		diskPath = "/dev/" + diskPath
	}

	cmd := exec.Command("lsblk", "-b", "-J", "-o", "NAME,TYPE,SIZE,FSTYPE,UUID,MOUNTPOINT", diskPath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("lsblk failed for %s: %w", diskPath, err)
	}

	var result struct {
		Blockdevices []struct {
			Children []struct {
				Name       string      `json:"name"`
				Type       string      `json:"type"`
				Size       json.Number `json:"size"`
				Fstype     string      `json:"fstype"`
				UUID       string      `json:"uuid"`
				Mountpoint string      `json:"mountpoint"`
			} `json:"children"`
		} `json:"blockdevices"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse lsblk output: %w", err)
	}

	var partitions []PartitionEntry
	for _, child := range result.Blockdevices[0].Children {
		if child.Type != "part" {
			continue
		}
		partPath := "/dev/" + child.Name

		// For btrfs, get all subvolume mount points
		var mountpoints []string
		if child.Fstype == "btrfs" {
			mountpoints = getBtrfsMountpoints(partPath)
		}
		if mountpoints == nil {
			mountpoints = []string{child.Mountpoint}
		}

		partSize, _ := child.Size.Int64() //nolint:errcheck // lsblk always produces valid numbers
		partitions = append(partitions, PartitionEntry{
			Path:        partPath,
			Name:        child.Name,
			Type:        child.Type,
			Size:        partSize,
			Fstype:      child.Fstype,
			UUID:        child.UUID,
			Mountpoints: mountpoints,
		})
	}

	return partitions, nil
}

// getBtrfsMountpoints returns all mount points for a btrfs partition using findmnt.
func getBtrfsMountpoints(partPath string) []string {
	cmd := exec.Command("findmnt", "-J", "-o", "TARGET", partPath)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var result struct {
		Filesystems []struct {
			Target string `json:"target"`
		} `json:"filesystems"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil
	}

	var mounts []string
	for _, fs := range result.Filesystems {
		if fs.Target != "" {
			mounts = append(mounts, fs.Target)
		}
	}
	return mounts
}

// FormatSize converts a byte count to human-readable format.
func FormatSize(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}

	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// DerivePartitionPath returns the device path for partition num on diskID.
// NVMe devices use the spN convention (e.g. /dev/nvme0n1p1), everything else
// uses dN (e.g. /dev/sda1). The last character of the disk name determines the
// separator: if it's a digit (e.g. nvme0n1), we insert "p" before the partition
// number; otherwise we append directly (e.g. sda → sda1).
func DerivePartitionPath(diskID string, num int) string {
	if len(diskID) > 0 {
		last := diskID[len(diskID)-1]
		if last >= '0' && last <= '9' {
			return fmt.Sprintf("%sp%d", diskID, num)
		}
	}
	return fmt.Sprintf("%s%d", diskID, num)
}

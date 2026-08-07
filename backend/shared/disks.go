package shared

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// PartitionEntry represents a single partition on a disk, as returned by lsblk.
type PartitionEntry struct {
	Path   string
	Type   string
	Size   string
	Fstype string
	UUID   string
	Trans  string
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
			Type:   child.Type,
			Size:   child.Size.Value,
			Fstype: child.Fstype,
			UUID:   child.UUID,
			Trans:  child.Trans,
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

// GetDisks returns a list of available block devices (disks) for installation.
func GetDisks() ([]DiskInfo, error) {
	if os.Getenv("FAKE_DISKS") == "1" {
		return fakeDisks(), nil
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
			Name:        bd.Name,
			Path:        "/dev/" + bd.Name,
			Size:        bd.Size.Value,
			Type:        bd.Type,
			Model:       strings.TrimSpace(bd.Model),
			Vendor:      strings.TrimSpace(bd.Vendor),
			Rotational:  bool(bd.Rota),
			Readonly:    bool(bd.Ro),
			Fstype:      bd.Fstype,
			Mountpoint:  bd.Mountpoint,
			Serial:      strings.TrimSpace(bd.Serial),
			Trans:       bd.Trans,
			IsRemovable: isRemovable(bd.Trans),
		}

		if disk.Model == "" && disk.Vendor != "" {
			disk.Model = disk.Vendor
		}
		if disk.Model == "" {
			disk.Model = "Unknown"
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

// PartitionDetail represents a partition with full information for display.
type PartitionDetail struct {
	Path        string   `json:"path"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Size        string   `json:"size"`
	Fstype      string   `json:"fstype"`
	UUID        string   `json:"uuid"`
	Mountpoints []string `json:"mountpoints"`
}

// GetDiskPartitions returns detailed partition information for a disk using lsblk.
func GetDiskPartitions(diskPath string) ([]PartitionDetail, error) {
	if os.Getenv("FAKE_DISKS") == "1" {
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

	var partitions []PartitionDetail
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

		partitions = append(partitions, PartitionDetail{
			Path:        partPath,
			Name:        child.Name,
			Type:        child.Type,
			Size:        child.Size.String(),
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

// FormatSize converts byte size to human-readable format.
func FormatSize(byteStr string) string {
	var bytes uint64
	fmt.Sscanf(byteStr, "%d", &bytes) //nolint:errcheck // format is validated before calling
	if bytes == 0 {
		return byteStr
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
// uses dN (e.g. /dev/sda1).
func DerivePartitionPath(diskID string, num int) string {
	if strings.HasPrefix(diskID, "/dev/nvme") {
		return fmt.Sprintf("%sp%d", diskID, num)
	}
	return fmt.Sprintf("%s%d", diskID, num)
}

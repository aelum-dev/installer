package shared

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// PartitionEntry represents a single partition on a disk, as returned by lsblk.
type PartitionEntry struct {
	Path    string // e.g. "/dev/sda1" or "/dev/nvme0n1p1"
	Type    string // always "part"
	Size    string // byte size (from lsblk -b)
	Fstype  string // e.g. "vfat", "ext4"
	UUID    string // filesystem UUID
	Trans   string // transport: usb, sata, nvme, etc.
}

// GetPartitions returns the partitions of a given disk by querying lsblk.
// Handles all naming conventions: /dev/sda, /dev/nvme0n1, /dev/vda, etc.
// Returns only entries of type "part" (excludes the disk itself and any sub-devices).
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

// jsonSize handles lsblk's SIZE field which is a number with -b flag,
// or a human-readable string without it.
type jsonSize struct {
	Value string
}

func (j *jsonSize) UnmarshalJSON(data []byte) error {
	if len(data) >= 2 && data[0] == '"' && data[len(data)-1] == '"' {
		j.Value = strings.Trim(string(data), `"`)
		return nil
	}
	j.Value = string(data)
	return nil
}

// jsonBool handles lsblk's boolean fields which can be bool or string "0"/"1".
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

// lsblkEntry is the JSON structure returned by lsblk.
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
// Package-level: shared by GUI and console installer.
func GetDisks() ([]DiskInfo, error) {
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

		// Filter out virtual/non-physical block devices
		if isVirtualDevice(bd.Name) {
			continue
		}

		rotational := bool(bd.Rota)

		disk := DiskInfo{
			Name:         bd.Name,
			Path:         "/dev/" + bd.Name,
			Size:         bd.Size.Value,
			Type:         bd.Type,
			Model:        strings.TrimSpace(bd.Model),
			Vendor:       strings.TrimSpace(bd.Vendor),
			Rotational:   rotational,
			Readonly:     bool(bd.Ro),
			Fstype:       bd.Fstype,
			Mountpoint:   bd.Mountpoint,
			Serial:       strings.TrimSpace(bd.Serial),
			Trans:        bd.Trans,
			IsRemovable:  isRemovable(bd.Trans),
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

// isVirtualDevice returns true if the device name corresponds to a virtual
// block device that should not be shown to the user (loop, ram, dm, md).
func isVirtualDevice(name string) bool {
	// Strip /dev/ prefix if present
	short := strings.TrimPrefix(name, "/dev/")

	switch {
	case strings.HasPrefix(short, "loop"): // loop0, loop1, ...
		return true
	case strings.HasPrefix(short, "ram"): // ram0, ram1, ...
		return true
	case strings.HasPrefix(short, "dm-"): // dm-0, dm-1 (LVM, etc.)
		return true
	case strings.HasPrefix(short, "md"): // md0, md1 (software RAID)
		return true
	}
	return false
}

// isRemovable returns true if the transport indicates removable media.
func isRemovable(trans string) bool {
	switch trans {
	case "usb", "sd", "mmc":
		return true
	}
	return false
}

// PartitionDetail represents a partition with full information for display.
type PartitionDetail struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Size       string `json:"size"`
	Fstype     string `json:"fstype"`
	UUID       string `json:"uuid"`
	Mountpoint string `json:"mountpoint"`
}

// GetDiskPartitions returns detailed partition information for a disk.
// Uses lsblk directly (no root required) — safe to call from the GUI.
func GetDiskPartitions(diskPath string) ([]PartitionDetail, error) {
	if !strings.HasPrefix(diskPath, "/dev/") {
		diskPath = "/dev/" + diskPath
	}

	cmd := exec.Command("lsblk", "-b", "-J", "-o", "NAME,TYPE,SIZE,FSTYPE,UUID,MOUNTPOINT", diskPath)
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
		return nil, nil // disk not found, not an error
	}

	var partitions []PartitionDetail
	for _, child := range result.Blockdevices[0].Children {
		if child.Type != "part" {
			continue
		}
		partitions = append(partitions, PartitionDetail{
			Path:       "/dev/" + child.Name,
			Name:       child.Name,
			Type:       child.Type,
			Size:       child.Size.Value,
			Fstype:     child.Fstype,
			UUID:       child.UUID,
			Mountpoint: child.Mountpoint,
		})
	}

	return partitions, nil
}

// FormatPartitionSize converts a byte size string to human-readable format.
// Convenience wrapper around FormatSize for partition data.
func FormatPartitionSize(byteStr string) string {
	return FormatSize(byteStr)
}

// FormatSize converts byte size to human-readable format.
// Shared by GUI and console installer.
func FormatSize(byteStr string) string {
	var bytes uint64
	fmt.Sscanf(byteStr, "%d", &bytes)
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

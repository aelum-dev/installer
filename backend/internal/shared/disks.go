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
// When FAKE_DISKS=1, returns synthetic data for UI testing.
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
// block device that should not be shown to the user.
func isVirtualDevice(name string) bool {
	// Strip /dev/ prefix if present
	short := strings.TrimPrefix(name, "/dev/")

	for _, prefix := range VirtualDevicePrefixes {
		if strings.HasPrefix(short, prefix) {
			return true
		}
	}
	return false
}

// isRemovable returns true if the transport indicates removable media.
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
// When FAKE_DISKS=1, returns synthetic partition data.
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

// ── Fake data generator for UI testing (FAKE_DISKS=1) ──

func fakeDisks() []DiskInfo {
	return []DiskInfo{
		{
			Name: "nvme0n1", Path: "/dev/nvme0n1", Size: "512110984960", Type: "disk",
			Model: "Samsung SSD 980 PRO 500GB", Vendor: "Samsung",
			Rotational: false, Readonly: false, Trans: "nvme", IsRemovable: false,
		},
		{
			Name: "sda", Path: "/dev/sda", Size: "1000204886016", Type: "disk",
			Model: "WD Blue SA516 1TB", Vendor: "Western Digital",
			Rotational: true, Readonly: false, Trans: "sata", Fstype: "ext4", Mountpoint: "/", Serial: "WDBJBK001001",
		},
		{
			Name: "sdb", Path: "/dev/sdb", Size: "2000398934016", Type: "disk",
			Model: "Seagate Barracuda 2TB", Vendor: "Seagate",
			Rotational: true, Readonly: false, Trans: "sata", Fstype: "ntfs", Mountpoint: "", Serial: "ZTK8F9A1",
		},
		{
			Name: "nvme1n1", Path: "/dev/nvme1n1", Size: "2000398934016", Type: "disk",
			Model: "Samsung SSD 870 EVO 2TB", Vendor: "Samsung",
			Rotational: false, Readonly: false, Trans: "nvme", IsRemovable: false,
		},
		{
			Name: "sdc", Path: "/dev/sdc", Size: "12803567616", Type: "disk",
			Model: "Verbatim STORE N GO", Vendor: "Verbatim",
			Rotational: true, Readonly: false, Trans: "usb", IsRemovable: true,
		},
		{
			Name: "sdd", Path: "/dev/sdd", Size: "500107862016", Type: "disk",
			Model: "Crucial MX500 500GB", Vendor: "Crucial",
			Rotational: false, Readonly: false, Trans: "sata", Fstype: "btrfs", Mountpoint: "/home",
		},
		{
			Name: "sde", Path: "/dev/sde", Size: "4000787031616", Type: "disk",
			Model: "WD Red Plus 4TB", Vendor: "Western Digital",
			Rotational: true, Readonly: false, Trans: "sata", IsRemovable: false,
		},
		{
			Name: "nvme2n1", Path: "/dev/nvme2n1", Size: "1000215250688", Type: "disk",
			Model: "SK hynix Platinum P41 1TB", Vendor: "SK hynix",
			Rotational: false, Readonly: false, Trans: "nvme", IsRemovable: false,
		},
		{
			Name: "sdg", Path: "/dev/sdg", Size: "32000000000", Type: "disk",
			Model: "SanDisk Ultra Fit", Vendor: "SanDisk",
			Rotational: true, Readonly: false, Trans: "usb", IsRemovable: true,
		},
		{
			Name: "sdh", Path: "/dev/sdh", Size: "8001567940096", Type: "disk",
			Model: "Toshiba X300 8TB", Vendor: "Toshiba",
			Rotational: true, Readonly: false, Trans: "sata", Fstype: "ext4", Mountpoint: "/mnt/data",
		},
	}
}

func fakePartitions(diskPath string) []PartitionDetail {
	switch diskPath {
	case "/dev/nvme0n1":
		return []PartitionDetail{
			{Path: "/dev/nvme0n1p1", Name: "nvme0n1p1", Type: "part", Size: "536870912", Fstype: "fat32", UUID: "A1B2-C3D4", Mountpoint: "/boot/efi"},
			{Path: "/dev/nvme0n1p2", Name: "nvme0n1p2", Type: "part", Size: "511574094848", Fstype: "ext4", UUID: "e5f6-a7b8-c9d0-e1f2", Mountpoint: ""},
		}
	case "/dev/sda":
		return []PartitionDetail{
			{Path: "/dev/sda1", Name: "sda1", Type: "part", Size: "536870912", Fstype: "fat32", UUID: "B2C3-D4E5", Mountpoint: "/boot/efi"},
			{Path: "/dev/sda2", Name: "sda2", Type: "part", Size: "42949672960", Fstype: "ext4", UUID: "c3d4-e5f6-a7b8-c9d0", Mountpoint: "/"},
			{Path: "/dev/sda3", Name: "sda3", Type: "part", Size: "926655841792", Fstype: "ext4", UUID: "d4e5-f6a7-b8c9-d0e1", Mountpoint: "/home"},
			{Path: "/dev/sda4", Name: "sda4", Type: "part", Size: "60344669184", Fstype: "swap", UUID: "e5f6-a7b8-c9d0-e1f2", Mountpoint: "[SWAP]"},
		}
	case "/dev/sdb":
		return []PartitionDetail{
			{Path: "/dev/sdb1", Name: "sdb1", Type: "part", Size: "2000398934016", Fstype: "ntfs", UUID: "F0A1-B2C3-D4E5-F6A7", Mountpoint: ""},
		}
	case "/dev/nvme1n1":
		return []PartitionDetail{
			{Path: "/dev/nvme1n1p1", Name: "nvme1n1p1", Type: "part", Size: "536870912", Fstype: "fat32", UUID: "C3D4-E5F6", Mountpoint: "/boot/efi"},
			{Path: "/dev/nvme1n1p2", Name: "nvme1n1p2", Type: "part", Size: "1999861872640", Fstype: "btrfs", UUID: "a7b8-c9d0-e1f2-a3b4", Mountpoint: ""},
		}
	case "/dev/sdc":
		return []PartitionDetail{
			{Path: "/dev/sdc1", Name: "sdc1", Type: "part", Size: "12803567616", Fstype: "vfat", UUID: "D4E5-F6A7", Mountpoint: ""},
		}
	case "/dev/sdd":
		return []PartitionDetail{
			{Path: "/dev/sdd1", Name: "sdd1", Type: "part", Size: "500107862016", Fstype: "btrfs", UUID: "b8c9-d0e1-f2a3-b4c5", Mountpoint: "/home"},
		}
	case "/dev/sde":
		return []PartitionDetail{
			{Path: "/dev/sde1", Name: "sde1", Type: "part", Size: "536870912", Fstype: "fat32", UUID: "E5F6-A7B8", Mountpoint: "/boot/efi"},
			{Path: "/dev/sde2", Name: "sde2", Type: "part", Size: "3999649161728", Fstype: "ext4", UUID: "c9d0-e1f2-a3b4-c5d6", Mountpoint: ""},
		}
	case "/dev/nvme2n1":
		return []PartitionDetail{
			{Path: "/dev/nvme2n1p1", Name: "nvme2n1p1", Type: "part", Size: "536870912", Fstype: "fat32", UUID: "F6A7-B8C9", Mountpoint: "/boot/efi"},
			{Path: "/dev/nvme2n1p2", Name: "nvme2n1p2", Type: "part", Size: "999678379776", Fstype: "ext4", UUID: "d0e1-f2a3-b4c5-d6e7", Mountpoint: ""},
		}
	case "/dev/sdg":
		return []PartitionDetail{
			{Path: "/dev/sdg1", Name: "sdg1", Type: "part", Size: "32000000000", Fstype: "vfat", UUID: "A7B8-C9D0", Mountpoint: ""},
		}
	case "/dev/sdh":
		return []PartitionDetail{
			{Path: "/dev/sdh1", Name: "sdh1", Type: "part", Size: "536870912", Fstype: "fat32", UUID: "B8C9-D0E1", Mountpoint: "/boot/efi"},
			{Path: "/dev/sdh2", Name: "sdh2", Type: "part", Size: "7999649161728", Fstype: "ext4", UUID: "e1f2-a3b4-c5d6-e7f8", Mountpoint: "/mnt/data"},
		}
	default:
		return []PartitionDetail{}
	}
}

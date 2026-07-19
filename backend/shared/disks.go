package shared

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

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

		rotational := bool(bd.Rota)

		disk := DiskInfo{
			Name:       bd.Name,
			Path:       "/dev/" + bd.Name,
			Size:       bd.Size.Value,
			Type:       bd.Type,
			Model:      strings.TrimSpace(bd.Model),
			Vendor:     strings.TrimSpace(bd.Vendor),
			Rotational: rotational,
			Readonly:   bool(bd.Ro),
			Fstype:     bd.Fstype,
			Mountpoint: bd.Mountpoint,
			Serial:     strings.TrimSpace(bd.Serial),
			Trans:      bd.Trans,
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

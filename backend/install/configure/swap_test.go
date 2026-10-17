package configure

import "testing"

// Swap-file support was replaced with zram (see services.go): ConfigureZram
// sizes a compressed swap device from the host's RAM. These tests cover the pure
// sizing rules instead of the (mount-dependent) step itself.

// TestCalculateZramSizeMiB covers the sizing rules in configure_files_test.go
// against a scripted /proc/meminfo, so the floor/cap boundaries are pinned
// deterministically instead of depending on the host's RAM.

func TestZramSizeMaxMiB(t *testing.T) {
	if zramSizeMaxMiB != 8192 {
		t.Errorf("zramSizeMaxMiB = %d, want 8192", zramSizeMaxMiB)
	}
}

// TestMeminfoKBRegex pins the /proc/meminfo parser used to size zram.
func TestMeminfoKBRegex(t *testing.T) {
	matches := meminfoKBRegex.FindStringSubmatch("MemTotal:       16299560 kB")
	if matches == nil || matches[1] != "16299560" {
		t.Errorf("failed to parse a MemTotal line: %v", matches)
	}
	if meminfoKBRegex.MatchString("MemFree:        123456 kB") {
		t.Error("a MemFree line must not match the MemTotal regex")
	}
}

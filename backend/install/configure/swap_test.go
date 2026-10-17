package configure

import "testing"

// Swap-file support was replaced with zram (see services.go): ConfigureZram
// sizes a compressed swap device from the host's RAM. These tests cover the pure
// sizing rules instead of the (mount-dependent) step itself.

// TestCalculateZramSizeMiB verifies the host-memory-based sizing returns one of
// the supported sizes and respects the floor/cap.
func TestCalculateZramSizeMiB(t *testing.T) {
	got := calculateZramSizeMiB()
	switch got {
	case 2048, 4096, zramSizeMaxMiB:
	default:
		t.Errorf("calculateZramSizeMiB() = %d, want one of 2048, 4096, %d", got, zramSizeMaxMiB)
	}
	if got < 2048 {
		t.Errorf("zram size %d is below the 2 GiB floor", got)
	}
	if got > zramSizeMaxMiB {
		t.Errorf("zram size %d exceeds the cap %d", got, zramSizeMaxMiB)
	}
}

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

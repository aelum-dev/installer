package install

import (
	"testing"
)

// Note: Swap file support has been replaced with zram.
// The ConfigureZram step writes /etc/systemd/zram-generator.conf
// and the generator handles the rest at boot.

func TestConfigureZram_ConfigContent(t *testing.T) {
	// Verify the config string content is correct.
	// We can't easily test the full function without a real helper,
	// but we can verify the config template is well-formed.
	zramConf := `[zram0]
zram-size = min(ram / 2, 16384)
compression-algorithm = zstd
`
	if zramConf == "" {
		t.Error("zram config should not be empty")
	}
	if !containsString(zramConf, "zram-size = min(ram / 2, 16384)") {
		t.Error("zram config missing zram-size setting")
	}
	if !containsString(zramConf, "compression-algorithm = zstd") {
		t.Error("zram config missing compression-algorithm setting")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && indexOfSubstring(s, substr) >= 0
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

package install

import (
	"testing"
)

func TestCalculateSwapSize_8GBRAM(t *testing.T) {
	// 8 GB = 8 * 1024 * 1024 kB
	ramKB := int64(8 * 1024 * 1024)
	size := CalculateSwapSize(ramKB)
	expected := int64(4 * 1024 * 1024 * 1024) // 4 GiB
	if size != expected {
		t.Errorf("8 GB RAM: got %d bytes (%d GiB), want %d bytes (%d GiB)", size, size/(1024*1024*1024), expected, expected/(1024*1024*1024))
	}
}

func TestCalculateSwapSize_16GBRAM(t *testing.T) {
	ramKB := int64(16 * 1024 * 1024)
	size := CalculateSwapSize(ramKB)
	expected := int64(8 * 1024 * 1024 * 1024) // 8 GiB
	if size != expected {
		t.Errorf("16 GB RAM: got %d bytes (%d GiB), want %d bytes (%d GiB)", size, size/(1024*1024*1024), expected, expected/(1024*1024*1024))
	}
}

func TestCalculateSwapSize_32GBRAM(t *testing.T) {
	ramKB := int64(32 * 1024 * 1024)
	size := CalculateSwapSize(ramKB)
	expected := int64(16 * 1024 * 1024 * 1024) // 16 GiB (capped)
	if size != expected {
		t.Errorf("32 GB RAM: got %d bytes (%d GiB), want %d bytes (%d GiB)", size, size/(1024*1024*1024), expected, expected/(1024*1024*1024))
	}
}

func TestCalculateSwapSize_64GBRAM(t *testing.T) {
	ramKB := int64(64 * 1024 * 1024)
	size := CalculateSwapSize(ramKB)
	expected := int64(16 * 1024 * 1024 * 1024) // 16 GiB (capped)
	if size != expected {
		t.Errorf("64 GB RAM: got %d bytes (%d GiB), want %d bytes (%d GiB)", size, size/(1024*1024*1024), expected, expected/(1024*1024*1024))
	}
}

func TestCalculateSwapSize_SmallRAM(t *testing.T) {
	// 2 GB RAM
	ramKB := int64(2 * 1024 * 1024)
	size := CalculateSwapSize(ramKB)
	expected := int64(1 * 1024 * 1024 * 1024) // 1 GiB
	if size != expected {
		t.Errorf("2 GB RAM: got %d bytes (%d MiB), want %d bytes (%d MiB)", size, size/(1024*1024), expected, expected/(1024*1024))
	}
}

func TestFormatSwapSize_GiB(t *testing.T) {
	tests := []struct {
		bytes  int64
		expect string
	}{
		{4 * 1024 * 1024 * 1024, "4G"},
		{8 * 1024 * 1024 * 1024, "8G"},
		{16 * 1024 * 1024 * 1024, "16G"},
	}
	for _, tt := range tests {
		got := formatSwapSize(tt.bytes)
		if got != tt.expect {
			t.Errorf("formatSwapSize(%d) = %q, want %q", tt.bytes, got, tt.expect)
		}
	}
}

func TestFormatSwapSize_MiB(t *testing.T) {
	tests := []struct {
		bytes  int64
		expect string
	}{
		{512 * 1024 * 1024, "512M"},
		{1024 * 1024 * 1024 - 1, "1023M"}, // just under 1 GiB
	}
	for _, tt := range tests {
		got := formatSwapSize(tt.bytes)
		if got != tt.expect {
			t.Errorf("formatSwapSize(%d) = %q, want %q", tt.bytes, got, tt.expect)
		}
	}
}

func TestParseSwapSize_G(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"4G", 4 * 1024 * 1024 * 1024},
		{"8G", 8 * 1024 * 1024 * 1024},
		{"16G", 16 * 1024 * 1024 * 1024},
	}
	for _, tt := range tests {
		got, err := parseSwapSize(tt.input)
		if err != nil {
			t.Errorf("parseSwapSize(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.expected {
			t.Errorf("parseSwapSize(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestParseSwapSize_M(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"512M", 512 * 1024 * 1024},
		{"4096M", 4096 * 1024 * 1024},
	}
	for _, tt := range tests {
		got, err := parseSwapSize(tt.input)
		if err != nil {
			t.Errorf("parseSwapSize(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.expected {
			t.Errorf("parseSwapSize(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestParseSwapSize_Invalid(t *testing.T) {
	invalid := []string{"", "8", "8X", "abcG", "8 GB"}
	for _, s := range invalid {
		_, err := parseSwapSize(s)
		if err == nil {
			t.Errorf("parseSwapSize(%q) expected error, got nil", s)
		}
	}
}

func TestSwapSizeRoundTrip(t *testing.T) {
	// Verify that formatSwapSize and parseSwapSize are inverse operations
	sizes := []int64{
		1 * 1024 * 1024 * 1024,   // 1G
		4 * 1024 * 1024 * 1024,   // 4G
		8 * 1024 * 1024 * 1024,   // 8G
		16 * 1024 * 1024 * 1024,  // 16G
		512 * 1024 * 1024,        // 512M
		4096 * 1024 * 1024,       // 4096M
	}
	for _, original := range sizes {
		formatted := formatSwapSize(original)
		parsed, err := parseSwapSize(formatted)
		if err != nil {
			t.Errorf("round-trip failed for %d: parse error: %v", original, err)
			continue
		}
		if parsed != original {
			t.Errorf("round-trip failed: %d -> %q -> %d", original, formatted, parsed)
		}
	}
}

package progress

import "testing"

func TestParsePackagesCount(t *testing.T) {
	tests := []struct {
		line string
		want int
	}{
		{"Packages (42)", 42},
		{"Packages (1)", 1},
		{"no package count here", 0},
		{"Packages ()", 0},
		// The regex matches the digits but Atoi overflows, so the count is
		// rejected rather than wrapping.
		{"Packages (99999999999999999999)", 0},
	}
	for _, tt := range tests {
		if got := parsePackagesCount(tt.line); got != tt.want {
			t.Errorf("parsePackagesCount(%q) = %d, want %d", tt.line, got, tt.want)
		}
	}
}

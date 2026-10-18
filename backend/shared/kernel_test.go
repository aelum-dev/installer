package shared

import "testing"

func TestPickKernelFile(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  string
	}{
		{
			name:  "prefers the Aelum kernel",
			files: []string{"/mnt/boot/vmlinuz-linux", "/mnt/boot/vmlinuz-linux-lts"},
			want:  "vmlinuz-linux-lts",
		},
		{
			name:  "prefers the Aelum kernel regardless of order",
			files: []string{"/mnt/boot/vmlinuz-linux-lts", "/mnt/boot/vmlinuz-linux"},
			want:  "vmlinuz-linux-lts",
		},
		{
			name:  "falls back to the first entry",
			files: []string{"/mnt/boot/vmlinuz-linux\n", "/mnt/boot/vmlinuz-custom"},
			want:  "vmlinuz-linux",
		},
		{
			name:  "a prefix is not a match",
			files: []string{"vmlinuz-linux", "vmlinuz-lts"},
			want:  "vmlinuz-linux",
		},
		{
			name:  "empty input",
			files: nil,
			want:  "",
		},
		{
			name:  "blank entries are skipped",
			files: []string{"", "   ", "vmlinuz-custom"},
			want:  "vmlinuz-custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PickKernelFile(tt.files); got != tt.want {
				t.Errorf("PickKernelFile(%v) = %q, want %q", tt.files, got, tt.want)
			}
		})
	}
}

package shared

import "testing"

// TestValidateDiskPath covers the shell-safety guard applied to every disk path
// before it reaches a privileged command.
func TestValidateDiskPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid sata", "/dev/sda", false},
		{"valid nvme", "/dev/nvme0n1", false},
		{"valid with hyphen and underscore", "/dev/disk-my_disk1", false},
		{"empty", "", true},
		{"relative path", "dev/sda", true},
		{"no device name", "/dev/", true},
		{"shell metacharacter", "/dev/sda; rm -rf /", true},
		{"path traversal", "/dev/sda/../sdb", true},
		{"space", "/dev/sd a", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDiskPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDiskPath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

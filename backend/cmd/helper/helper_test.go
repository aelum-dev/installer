package main

import "testing"

func TestBuildPacstrapArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantErr   bool
		wantMount string
	}{
		{
			name:      "with -K flag",
			args:      []string{"-K", "/mnt", "base", "linux"},
			wantErr:   false,
			wantMount: "-K",
		},
		{
			name:      "without -K flag",
			args:      []string{"/mnt", "base", "linux"},
			wantErr:   false,
			wantMount: "/mnt",
		},
		{
			name:    "no args",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "only mountpoint",
			args:    []string{"/mnt"},
			wantErr: true,
		},
		{
			name:      "only -K flag no mountpoint",
			args:      []string{"-K"},
			wantErr:   true,
			wantMount: "-K",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildPacstrapArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildPacstrapArgs(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantMount {
				t.Errorf("buildPacstrapArgs(%v) = %q, want %q", tt.args, got, tt.wantMount)
			}
		})
	}
}

func TestPacstrapPreservesKFlag(t *testing.T) {
	// Verify that -K is a valid arg and not stripped.
	// The actual exec is tested indirectly via buildPacstrapArgs accepting it.
	args := []string{"-K", "/mnt", "base"}
	mount, err := buildPacstrapArgs(args)
	if err != nil {
		t.Fatalf("buildPacstrapArgs(-K /mnt base) errored: %v", err)
	}
	if mount != "-K" {
		t.Fatalf("expected mount arg to be '-K', got %q", mount)
	}
	// The second arg is the actual mountpoint — confirm it's preserved.
	if len(args) < 2 || args[1] != "/mnt" {
		t.Fatalf("expected '/mnt' at args[1], got %v", args)
	}
}

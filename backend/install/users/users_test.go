package users

import (
	"testing"
)

// The copy jobs that populate the target before a home is seeded from it live in
// install/isocopy, and so do the tests that hold them (including the btrfs-only
// snapshot-restore group). What is asserted here is the account.

func TestTargetNameConflict(t *testing.T) {
	passwd := []byte("root:x:0:0:root:/root:/bin/bash\nalice:x:1000:1000::/home/alice:/bin/bash\n")
	group := []byte("root:x:0:\nwheel:x:10:alice\nalice:x:1000:\n")

	tests := []struct {
		name         string
		wantKind     string
		wantConflict bool
	}{
		{name: "root", wantKind: "user", wantConflict: true},
		{name: "alice", wantKind: "user", wantConflict: true},
		{name: "wheel", wantKind: "group", wantConflict: true},
		{name: "nosuchuser", wantConflict: false},
		// Prefix matches must not count: "rootless" is not "root".
		{name: "rootless", wantConflict: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, conflict := targetNameConflict(passwd, group, tt.name)
			if conflict != tt.wantConflict || kind != tt.wantKind {
				t.Errorf("targetNameConflict(%q) = (%q, %v), want (%q, %v)", tt.name, kind, conflict, tt.wantKind, tt.wantConflict)
			}
		})
	}
}

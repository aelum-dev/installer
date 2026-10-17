package users

import (
	"strings"
	"testing"

	"aelum-installer/backend/shared"
)

func TestISOCopyJobs_NotEmpty(t *testing.T) {
	if len(ISOCopyJobs) == 0 {
		t.Error("expected non-empty ISOCopyJobs")
	}
}

// TestISOCopyJobs_Invariants pins the safety properties of every copy job: the
// source is absolute and the destination lands inside the installed system at
// /mnt. A job that fails either would copy to the live ISO or crash the step.
func TestISOCopyJobs_Invariants(t *testing.T) {
	for _, job := range ISOCopyJobs {
		if !strings.HasPrefix(job.Source, "/") {
			t.Errorf("copy job source %q is not absolute", job.Source)
		}
		if !strings.HasPrefix(job.Dest, shared.MountRoot+"/") {
			t.Errorf("copy job destination %q is not under %s", job.Dest, shared.MountRoot)
		}
	}
}

// TestRun_NotMounted checks the step fails cleanly when the installed system is
// not mounted (the normal state during unit tests).
func TestRun_NotMounted(t *testing.T) {
	if err := Run(&shared.InstallContext{Username: "aelum"}); err == nil {
		t.Error("expected an error when /mnt is not mounted")
	}
}

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

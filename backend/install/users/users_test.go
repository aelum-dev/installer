package users

import (
	"testing"
)

func TestISOCopyJobs_NotEmpty(t *testing.T) {
	if len(ISOCopyJobs) == 0 {
		t.Error("expected non-empty ISOCopyJobs")
	}
}

func TestCopyJob_Struct(t *testing.T) {
	job := CopyJob{
		Source: "/etc/skel/.",
		Dest:   "/mnt/etc/skel/",
		Flags:  []string{"-a"},
	}

	if job.Source == "" {
		t.Error("expected non-empty Source")
	}
	if job.Dest == "" {
		t.Error("expected non-empty Dest")
	}
}

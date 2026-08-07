package install

import (
	"fmt"
	"path/filepath"
	"strings"
)

// CopyJob defines a source→destination path pair for ISO-to-target file copying.
type CopyJob struct {
	// Source is the path on the live ISO.
	Source string
	// Dest is the path inside the installed system (/mnt/...).
	Dest string
	// Flags are extra cp flags (e.g. []string{"-a"} or []string{"-f"}).
	Flags []string
}

// ISOCopyJobs lists all files to copy from the live ISO to the installed system.
// Each job is executed in order; mkdir -p is run on the destination parent
// directory before each copy (idempotent, safe to run unconditionally).
var ISOCopyJobs = []CopyJob{
	{Source: "/etc/skel/.", Dest: "/mnt/etc/skel/", Flags: []string{"-a"}},
	{Source: "/etc/dconf/profile/user", Dest: "/mnt/etc/dconf/profile/user", Flags: []string{"-a"}},
	{Source: "/etc/dconf/db/local.d/01-icons", Dest: "/mnt/etc/dconf/db/local.d/01-icons", Flags: []string{"-a"}},
	{Source: "/usr/share/DankMaterialShell/wallpaper.jpg", Dest: "/mnt/usr/share/DankMaterialShell/wallpaper.jpg", Flags: []string{}},
	{Source: "/etc/environment", Dest: "/mnt/etc/environment", Flags: []string{}},
	{Source: "/etc/greetd/hypr.lua", Dest: "/mnt/etc/greetd/hypr.lua", Flags: []string{}},
}

// ExecuteCopyJobs copies all static files from the live ISO to the installed system.
// It iterates over ISOCopyJobs, runs mkdir -p on each destination parent,
// then cp. Individual copy failures are collected and reported together so that
// a single failed file does not prevent the rest from being copied.
func ExecuteCopyJobs() error {
	var errs []error
	for _, job := range ISOCopyJobs {

		// Ensure parent directory exists (idempotent — safe to run unconditionally).
		parentDir := filepath.Dir(job.Dest)
		if _, err := Run("mkdir", "-p", parentDir).CombinedOutput(); err != nil {
			errs = append(errs, fmt.Errorf("failed to create dir %s: %w", parentDir, err))
			continue
		}

		// Build cp command: cp [flags] source dest
		cpArgs := append([]string{"cp"}, job.Flags...)
		cpArgs = append(cpArgs, job.Source, job.Dest)
		cmd := Run(cpArgs...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to copy %s → %s: %w: %s", job.Source, job.Dest, err, strings.TrimSpace(string(output))))
			continue
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d copy job(s) failed", len(errs))
	}
	return nil
}

// copyISOFiles is kept for backwards compatibility — delegates to ExecuteCopyJobs.
func copyISOFiles() error {
	return ExecuteCopyJobs()
}

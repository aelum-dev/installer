package install

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// copyEntry defines a source→destination path pair for ISO-to-target file copying.
type copyEntry struct {
	// Source is the path on the live ISO.
	Source string
	// Dest is the path inside the installed system (/mnt/...).
	Dest string
	// Flags are extra cp flags (e.g. []string{"-a"} or []string{"-f"}).
	Flags []string
}

// isoCopies lists all files to copy from the live ISO to the installed system.
// Each entry is copied in order; mkdir -p is run on the destination parent
// directory before each copy (idempotent, safe to run unconditionally).
var isoCopies = []copyEntry{
	{Source: "/etc/skel/.", Dest: "/mnt/etc/skel/", Flags: []string{"-a"}},
	{Source: "/etc/dconf/profile/user", Dest: "/mnt/etc/dconf/profile/user", Flags: []string{"-a"}},
	{Source: "/etc/dconf/db/local.d/01-icons", Dest: "/mnt/etc/dconf/db/local.d/01-icons", Flags: []string{"-a"}},
	{Source: "/usr/share/DankMaterialShell/wallpaper.jpg", Dest: "/mnt/usr/share/DankMaterialShell/wallpaper.jpg", Flags: []string{}},
	{Source: "/etc/environment", Dest: "/mnt/etc/environment", Flags: []string{}},
	{Source: "/etc/greetd/hypr.lua", Dest: "/mnt/etc/greetd/hypr.lua", Flags: []string{}},
}

// copyISOFiles copies all static files from the live ISO to the installed system.
// It iterates over isoCopies, runs mkdir -p on each destination parent, then cp.
func copyISOFiles(ctx *InstallContext) error {
	for _, entry := range isoCopies {
		if ctx.Logger != nil {
		}

		// Ensure parent directory exists (idempotent — safe to run unconditionally).
		parentDir := filepath.Dir(entry.Dest)
		mkdirCmd := ctx.run(HelperCmdRun, "mkdir", "-p", parentDir)
		if _, err := mkdirCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create dir %s: %w", parentDir, err)
		}

		// Build cp command: cp [flags] source dest
		cpArgs := append([]string{ctx.Helper, HelperCmdRun, "cp"}, entry.Flags...)
		cpArgs = append(cpArgs, entry.Source, entry.Dest)
		cmd := exec.Command("sudo", cpArgs...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to copy %s → %s: %w: %s", entry.Source, entry.Dest, err, strings.TrimSpace(string(output)))
		}
	}
	return nil
}

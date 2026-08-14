package users

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"aestheticos-installer/backend/install/primitives"
	"aestheticos-installer/backend/shared"
)

// ============================================================================
// ISO file copying
// ============================================================================

// CopyJob defines a source→destination path pair for ISO-to-target file copying.
type CopyJob struct {
	Source string
	Dest   string
	Flags  []string
}

// ISOCopyJobs lists all files to copy from the live ISO to the installed system.
var ISOCopyJobs = []CopyJob{
	{Source: "/etc/skel/.", Dest: "/mnt/etc/skel/", Flags: []string{"-a"}},
	{Source: "/etc/dconf/profile/user", Dest: "/mnt/etc/dconf/profile/user", Flags: []string{"-a"}},
	{Source: "/etc/dconf/db/local.d/01-icons", Dest: "/mnt/etc/dconf/db/local.d/01-icons", Flags: []string{"-a"}},
	{Source: "/etc/environment", Dest: "/mnt/etc/environment", Flags: []string{}},
	{Source: "/etc/greetd/hypr.lua", Dest: "/mnt/etc/greetd/hypr.lua", Flags: []string{}},
	{Source: "/usr/lib/firefox/distribution/policies.json", Dest: "/mnt/usr/lib/firefox/distribution/policies.json", Flags: []string{}},
	{Source: shared.WallpaperSourceDark, Dest: "/mnt" + shared.WallpaperSourceDark, Flags: []string{}},
	{Source: shared.WallpaperSourceLight, Dest: "/mnt" + shared.WallpaperSourceLight, Flags: []string{}},
}

// ExecuteCopyJobs copies all static files from the live ISO to the installed system.
func ExecuteCopyJobs() error {
	var errs []error
	for _, job := range ISOCopyJobs {
		parentDir := filepath.Dir(job.Dest)
		if _, err := primitives.Run("mkdir", "-p", parentDir).CombinedOutput(); err != nil {
			errs = append(errs, fmt.Errorf("failed to create dir %s: %w", parentDir, err))
			continue
		}

		cpArgs := append([]string{"cp"}, job.Flags...)
		cpArgs = append(cpArgs, job.Source, job.Dest)
		cmd := primitives.Run(cpArgs...)
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

func copyISOFiles() error {
	return ExecuteCopyJobs()
}

// ============================================================================
// User setup
// ============================================================================

// Run creates user accounts using sudo.
func Run(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}

	if ctx.Username == "" {
		return nil
	}

	var cmd *exec.Cmd
	var output []byte
	var err error

	if err := copyISOFiles(); err != nil {
		return err
	}

	output, err = primitives.RunChroot("dconf", "update")
	if err != nil {
		return fmt.Errorf("failed to compile dconf: %w: %s", err, string(output))
	}

	output, err = primitives.RunChroot("useradd", "-m", "-G", "wheel", ctx.Username)
	if err != nil {
		if !strings.Contains(string(output), "already exists") {
			return fmt.Errorf("failed to setup user: %w: %s", err, string(output))
		}
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	if len(ctx.Password) > 0 {
		cmd = primitives.Run("arch-chroot", shared.MountRoot, "chpasswd")
		cmd.Stdin = strings.NewReader(ctx.Username + ":" + string(ctx.Password) + "\n")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("passwd failed: %w: %s", err, string(output))
		}
		for k := range ctx.Password {
			ctx.Password[k] = 0
		}
	}

	cmd = primitives.Run("mkdir", "-p", shared.MountRoot+"/etc/sudoers.d")
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create sudoers.d dir: %w", err)
	}
	// Write with correct permissions directly (0440) to avoid a window where
	// the file is world-readable if the process crashes between write and chmod.
	sudoersPath := shared.MountRoot + "/etc/sudoers.d/wheel"
	if err := os.WriteFile(sudoersPath, []byte("%wheel ALL=(ALL:ALL) ALL\n"), 0o440); err != nil {
		return fmt.Errorf("failed to write sudoers: %w", err)
	}

	return nil
}

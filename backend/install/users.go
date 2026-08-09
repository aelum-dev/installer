package install

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

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
	{Source: "/etc/systemd/system/plymouth-wait-for-animation.service", Dest: "/mnt/etc/systemd/system/plymouth-wait-for-animation.service", Flags: []string{}},
	{Source: shared.DmsWallpaperSource, Dest: shared.DmsWallpaperDest, Flags: []string{}},
}

// ExecuteCopyJobs copies all static files from the live ISO to the installed system.
func ExecuteCopyJobs() error {
	var errs []error
	for _, job := range ISOCopyJobs {
		parentDir := filepath.Dir(job.Dest)
		if _, err := Run("mkdir", "-p", parentDir).CombinedOutput(); err != nil {
			errs = append(errs, fmt.Errorf("failed to create dir %s: %w", parentDir, err))
			continue
		}

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

func copyISOFiles() error {
	return ExecuteCopyJobs()
}

// ============================================================================
// User setup
// ============================================================================

// SetupUsers creates user accounts using sudo.
func SetupUsers(ctx *InstallContext) error {
	if err := validateMountedSystem(); err != nil {
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

	output, err = runChroot("dconf", "update")
	if err != nil {
		return fmt.Errorf("failed to compile dconf: %w: %s", err, string(output))
	}

	output, err = runChroot("useradd", "-m", "-G", "wheel", ctx.Username)
	if err != nil {
		if !strings.Contains(string(output), "already exists") {
			return fmt.Errorf("failed to setup user: %w: %s", err, string(output))
		}
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	if len(ctx.Password) > 0 {
		cmd = Run("arch-chroot", shared.MountRoot, "chpasswd")
		cmd.Stdin = strings.NewReader(ctx.Username + ":" + string(ctx.Password) + "\n")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("passwd failed: %w: %s", err, string(output))
		}
		for k := range ctx.Password {
			ctx.Password[k] = 0
		}
	}

	cmd = Run("mkdir", "-p", shared.MountRoot+"/etc/sudoers.d")
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create sudoers.d dir: %w", err)
	}
	if err := writeFile("/etc/sudoers.d/wheel", "%wheel ALL=(ALL:ALL) ALL\n"); err != nil {
		return err
	}
	cmd = Run("chmod", "0440", shared.MountRoot+"/etc/sudoers.d/wheel")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to chmod sudoers: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	return nil
}

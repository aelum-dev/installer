package users

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// ISO file copying
// ============================================================================

// CopyJob defines a source→destination path pair for ISO-to-target file copying.
type CopyJob struct {
	Source string
	Dest   string
	Flags  []string
	// Optional marks a cosmetic/branding file: if the live ISO does not ship it,
	// the copy is skipped with a warning instead of aborting the install. Files
	// without Optional are required (e.g. /etc/skel, the greeter config): a
	// missing one means the installed desktop would be broken, so the step fails
	// loudly.
	Optional bool
}

// ISOCopyJobs lists all files to copy from the live ISO to the installed system.
var ISOCopyJobs = []CopyJob{
	{Source: "/etc/skel/.", Dest: "/mnt/etc/skel/", Flags: []string{"-a"}},
	{Source: "/etc/dconf/profile/user", Dest: "/mnt/etc/dconf/profile/user", Flags: []string{"-a"}, Optional: true},
	{Source: "/etc/dconf/db/local.d/01-icons", Dest: "/mnt/etc/dconf/db/local.d/01-icons", Flags: []string{"-a"}, Optional: true},
	{Source: "/etc/dconf/db/local.d/02-nautilus-terminal", Dest: "/mnt/etc/dconf/db/local.d/02-nautilus-terminal", Flags: []string{"-a"}, Optional: true}, // nautilus-open-any-terminal: default terminal=kitty
	{Source: "/etc/os-release", Dest: "/mnt/etc/os-release", Flags: []string{}, Optional: true},                                                           // Aelum branding, overwrites the archlinux-release default
	{Source: "/etc/environment", Dest: "/mnt/etc/environment", Flags: []string{}, Optional: true},
	{Source: "/etc/greetd/hypr.lua", Dest: "/mnt/etc/greetd/hypr.lua", Flags: []string{}},
	{Source: "/usr/lib/firefox/distribution/policies.json", Dest: "/mnt/usr/lib/firefox/distribution/policies.json", Flags: []string{}, Optional: true},
	{Source: shared.WallpaperSourceDark, Dest: "/mnt" + shared.WallpaperSourceDark, Flags: []string{}, Optional: true},
	{Source: shared.WallpaperSourceLight, Dest: "/mnt" + shared.WallpaperSourceLight, Flags: []string{}, Optional: true},
}

// ExecuteCopyJobs copies all static files from the live ISO to the installed system.
func ExecuteCopyJobs() error {
	return executeCopyJobs(nil)
}

// executeCopyJobs is ExecuteCopyJobs with a logger for skipped optional files.
func executeCopyJobs(logger shared.Logger) error {
	var errs []error
	for _, job := range ISOCopyJobs {
		// A source missing from the live ISO is a different failure from a failed
		// copy: for optional branding files it is simply skipped, so a missing
		// wallpaper or Firefox policy cannot abort a nearly complete install.
		if !isoSourceExists(job.Source) {
			if job.Optional {
				if logger != nil {
					logger.Error("Warning: optional file missing from the live ISO, skipping: %s", job.Source)
				}
				continue
			}
			errs = append(errs, fmt.Errorf("required file missing from the live ISO: %s", job.Source))
			continue
		}

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
		return fmt.Errorf("%d copy job(s) failed: %w", len(errs), errors.Join(errs...))
	}
	return nil
}

// isoSourceExists reports whether a copy job's source exists on the live ISO.
func isoSourceExists(path string) bool {
	return primitives.Run("test", "-e", path).Run() == nil
}

func copyISOFiles(logger shared.Logger) error {
	return executeCopyJobs(logger)
}

// ============================================================================
// User setup
// ============================================================================

// checkTargetAccountName rejects a username that already exists as a user or
// group in the installed system. This is the same collision useradd enforces,
// but checked before anything is changed so the error is clear. It also covers
// package-created accounts absent from the static early-validation list.
func checkTargetAccountName(name string) error {
	passwd, err := primitives.ReadFile(shared.MountRoot + "/etc/passwd")
	if err != nil {
		return fmt.Errorf("read target passwd: %w", err)
	}
	group, err := primitives.ReadFile(shared.MountRoot + "/etc/group")
	if err != nil {
		return fmt.Errorf("read target group: %w", err)
	}
	if kind, conflict := targetNameConflict(passwd, group, name); conflict {
		return fmt.Errorf("username %q is already used by a %s on the target system; choose another", name, kind)
	}
	return nil
}

// targetNameConflict reports whether name is an existing user or group in the
// given passwd/group file contents.
func targetNameConflict(passwd, group []byte, name string) (kind string, conflict bool) {
	if hasAccountLine(passwd, name) {
		return "user", true
	}
	if hasAccountLine(group, name) {
		return "group", true
	}
	return "", false
}

// hasAccountLine reports whether data has a colon-separated record named name.
func hasAccountLine(data []byte, name string) bool {
	prefix := name + ":"
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

// Run creates the user account and writes the sudoers rule.
func Run(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}

	if ctx.Username == "" {
		return nil
	}

	if err := checkTargetAccountName(ctx.Username); err != nil {
		return err
	}
	if len(ctx.Password) > shared.MaxPasswordLength {
		return fmt.Errorf("password must be at most %d UTF-8 bytes", shared.MaxPasswordLength)
	}
	if strings.ContainsAny(string(ctx.Password), "\n\r\x00") {
		return fmt.Errorf("password cannot contain newlines or NUL characters")
	}

	var cmd *primitives.Command
	var output []byte
	var err error

	if err := copyISOFiles(ctx.Logger); err != nil {
		return err
	}

	// Compile the dconf databases copied from the ISO. This is best-effort: if
	// the dconf binary is not present (it is only a transitive dependency of the
	// desktop packages), the copied keyfiles are inert and the desktop still
	// works with its defaults. A malformed keyfile is surfaced as a warning too,
	// rather than aborting a nearly complete install.
	output, err = primitives.RunChroot("dconf", "update")
	if err != nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("Warning: dconf update failed (non-fatal): %v: %s", err, strings.TrimSpace(string(output)))
		}
	}

	output, err = primitives.RunChroot("useradd", "-m", "-G", "wheel", "--", ctx.Username)
	if err != nil {
		return fmt.Errorf("failed to setup user: %w: %s", err, string(output))
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
	if err := primitives.WriteFileMode(sudoersPath, "%wheel ALL=(ALL:ALL) ALL\n", 0o440); err != nil {
		return fmt.Errorf("failed to write sudoers: %w", err)
	}

	return nil
}

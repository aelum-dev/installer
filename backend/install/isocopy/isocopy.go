// Package isocopy carries the files the live ISO ships into the installed system.
//
// They are not user setup: the seeded home skeleton, the greeter's session config,
// the branding files, the dconf keyfiles and - since the snapshot-restore path became
// live-ISO source - the rollback helper, its polkit action and grant, the dialog that
// asks and the unit that runs it. All of it is source under live-iso/airootfs at the
// same absolute paths it ends up at, so the installer's half of the feature is a copy
// with a mode, not a generated file.
//
// It runs inside SetupUsers because that is where the target's /etc is populated and
// where homes get seeded - and it has to run before ConfigureSystem takes the
// "after installation" snapshot, because a snapshot boots the units, binaries and
// prompt it contains. install/step_test.go is what keeps that order true.
package isocopy

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// Job defines a source→destination path pair for ISO-to-target file copying.
type Job struct {
	Source string
	Dest   string
	Flags  []string
	// Optional marks a cosmetic/branding file: if the live ISO does not ship it,
	// the copy is skipped with a warning instead of aborting the install. Files
	// without Optional are required (e.g. /etc/skel, the greeter config): a
	// missing one means the installed desktop would be broken, so the step fails
	// loudly.
	Optional bool
	// BtrfsOnly marks a file that belongs to the snapshot machinery: an ext4
	// install has no snapshot to restore, so it gets none of them, and
	// verify-installed.sh asserts their absence there. They are Optional as well:
	// an installer newer than the ISO it is running from would otherwise fail an
	// install over files that ISO never shipped.
	BtrfsOnly bool
}

// Jobs lists every file copied from the live ISO to the installed system.
//
// The snapshot-restore group is here rather than written by the installer because it
// is source in the live ISO: the dialog, its translated copy, its user unit, the
// rollback helper and the polkit files that authorise it live in live-iso/airootfs at
// exactly these paths and reach the target unchanged. Same mechanism as the skel
// seeding and the greeter config, same rule as them: a snapshot boots the prompt and
// runs the helper it contains, so they have to be here before the "after
// installation" snapshot is taken.
var Jobs = []Job{
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
	// Restoring a booted snapshot: the helper, the action its OK button runs it
	// through, the grant for that action, and the prompt that asks. The config
	// directory is copied whole (with its i18n/) so a translated dialog is not a
	// list of one-file jobs. Flags -a everywhere: the helper's 0755 is the one
	// thing in this group that a plain copy would lose, and pkexec needs it.
	{Source: shared.SnapshotRestoreScriptPath, Dest: shared.MountRoot + shared.SnapshotRestoreScriptPath, Flags: []string{"-a"}, Optional: true, BtrfsOnly: true},
	{Source: shared.SnapshotRestorePolicyPath, Dest: shared.MountRoot + shared.SnapshotRestorePolicyPath, Flags: []string{"-a"}, Optional: true, BtrfsOnly: true},
	{Source: shared.SnapshotRestoreRulesPath, Dest: shared.MountRoot + shared.SnapshotRestoreRulesPath, Flags: []string{"-a"}, Optional: true, BtrfsOnly: true},
	{Source: shared.SnapshotPromptConfigDir + "/.", Dest: shared.MountRoot + shared.SnapshotPromptConfigDir + "/", Flags: []string{"-a"}, Optional: true, BtrfsOnly: true},
	{Source: shared.SnapshotPromptUnitDir + "/" + shared.SnapshotPromptUnitName, Dest: shared.MountRoot + shared.SnapshotPromptUnitDir + "/" + shared.SnapshotPromptUnitName, Flags: []string{"-a"}, Optional: true, BtrfsOnly: true},
}

// For is Jobs as this install should see it: everything, minus the files that only
// make sense on a filesystem that has snapshots.
func For(ctx *shared.InstallContext) []Job {
	if ctx != nil && ctx.FileSystem == shared.FileSystemBtrfs {
		return Jobs
	}
	jobs := make([]Job, 0, len(Jobs))
	for _, job := range Jobs {
		if job.BtrfsOnly {
			continue
		}
		jobs = append(jobs, job)
	}
	return jobs
}

// Run copies everything this install gets and then makes the copied dconf
// configuration readable by the desktop that will read it.
func Run(ctx *shared.InstallContext) error {
	if err := copy(ctx); err != nil {
		return err
	}
	compileDconf(ctx)
	return nil
}

// copy writes the copied files into the mounted system, one job at a time.
func copy(ctx *shared.InstallContext) error {
	var logger shared.Logger
	if ctx != nil {
		logger = ctx.Logger
	}

	var errs []error
	for _, job := range For(ctx) {
		// A source missing from the live ISO is a different failure from a failed
		// copy: for optional branding files it is simply skipped, so a missing
		// wallpaper or Firefox policy cannot abort a nearly complete install.
		if !sourceExists(job.Source) {
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

// compileDconf builds the dconf databases the copied keyfiles describe. Best-effort:
// if the dconf binary is not present (it is only a transitive dependency of the
// desktop packages) the copied keyfiles are inert and the desktop still works with
// its defaults, and a malformed keyfile is a warning, not a reason to abort a nearly
// complete install.
func compileDconf(ctx *shared.InstallContext) {
	output, err := primitives.RunChroot("dconf", "update")
	if err != nil && ctx != nil && ctx.Logger != nil {
		ctx.Logger.Error("Warning: dconf update failed (non-fatal): %v: %s", err, strings.TrimSpace(string(output)))
	}
}

// sourceExists reports whether a copy job's source exists on the live ISO.
func sourceExists(path string) bool {
	return primitives.Run("test", "-e", path).Run() == nil
}

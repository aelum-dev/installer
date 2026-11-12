package configure

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// Aelum package repository
// ============================================================================

// ConfigurePacmanRepo enables the [aelum] repository and the multilib one in the
// installed system's pacman.conf, trusts the Aelum signing key in the target's
// keyring, and drops the sync databases of repositories the target does not
// configure (see dropForeignSyncDBs). A fresh install can then install the
// distro's own applications, and anything 32-bit, with no hand-editing; the
// [aelum] repository itself is published by repo/publish-repo.sh (repo/REPO.md).
//
// It runs on every filesystem — the repository is not part of the snapshot
// machinery — and it must run before ConfigureSnapper, because the "after
// installation" snapshot is the state a user restores to and has to carry both
// the config block and the trusted key.
func ConfigurePacmanRepo(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}

	content, err := enablePacmanRepo(ctx)
	if err != nil {
		return err
	}
	if content, err = enableMultilibRepo(ctx, content); err != nil {
		return err
	}
	// After both, so the decision is made with the repository blocks the target
	// really has.
	if err := dropForeignSyncDBs(ctx, content); err != nil && ctx.Logger != nil {
		ctx.Logger.Error("Warning: could not clear the live session's package databases (non-fatal): %v", err)
	}
	return trustAelumKey(ctx)
}

// enablePacmanRepo puts the repository block in the target's pacman.conf and
// returns the file as it now reads. The file comes from the pacman package via
// pacstrap; there is no drop-in directory to write instead (see the
// constants' note), so editing the file is the normal Arch way — a modified
// pacman-owned file keeps its edits across updates, pacman only drops a
// .pacnew to diff.
//
// The block goes before [extra], not at the end. Measured with pacman 7.1
// against two throwaway repositories: when the same package name exists in two
// enabled repositories, the repository listed FIRST in the config wins even
// against a higher version elsewhere — order, not version, decides. refind is
// the live case: extra carries 0.14.2-3 and [aelum] our patched 0.14.2-4, and
// with [aelum] appended after [extra] pacman resolved refind from extra every
// time (the ISO the installer shipped proved it). An installed system must get
// the patched one, so [aelum] precedes [extra]. Nothing else in [aelum]
// collides with an official repository, and [core] keeps precedence over us,
// which is the right shape: Arch owns the base, we own what we ship.
func enablePacmanRepo(ctx *shared.InstallContext) (string, error) {
	hostPath := shared.MountRoot + shared.PacmanConfPath
	data, err := primitives.ReadFile(hostPath)
	if err != nil {
		return "", fmt.Errorf("failed to read pacman.conf: %w", err)
	}
	// Idempotent: re-running the installer over an existing system must not
	// stack a second [aelum] section (later sections would shadow the first).
	if hasPacmanSection(string(data), shared.PacmanRepoName) {
		if ctx.Logger != nil {
			ctx.Logger.Info("pacman repository [%s] already enabled", shared.PacmanRepoName)
		}
		return string(data), nil
	}

	block := "# Aelum repository - applications shipped by the distro, listed ahead of\n" +
		"# [extra] so our patched packages win by repository order (see above).\n" +
		"# Signed with the Aelum release key (release@aelum.dev); the installer\n" +
		"# trusts that key in pacman's keyring, so no further setup is needed.\n" +
		fmt.Sprintf("[%s]\n", shared.PacmanRepoName) +
		"SigLevel = " + shared.PacmanRepoSigLevel + "\n" +
		"Server = " + shared.PacmanRepoServer + "\n"
	updated, inserted := insertPacmanSectionBefore(string(data), shared.PacmanExtraRepoName, block)
	if !inserted {
		// No [extra] to precede — a hand-edited file. The old shape: append.
		updated = string(data) + "\n" + block
	}
	if err := primitives.WriteFile(shared.PacmanConfPath, updated); err != nil {
		return "", fmt.Errorf("failed to enable [%s] in pacman.conf: %w", shared.PacmanRepoName, err)
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("pacman repository [%s] enabled: %s", shared.PacmanRepoName, shared.PacmanRepoServer)
	}
	return updated, nil
}

// insertPacmanSectionBefore inserts block on its own lines before the section
// header [before], and reports whether it did. The block must already end in a
// newline; it is separated from the preceding section by a blank line.
func insertPacmanSectionBefore(content, before, block string) (string, bool) {
	header := "[" + before + "]"
	lines := strings.Split(content, "\n")
	for i := range lines {
		if strings.TrimSpace(lines[i]) == header {
			lines = slices.Insert(lines, i, strings.TrimSuffix(block, "\n"), "")
			return strings.Join(lines, "\n"), true
		}
	}
	return content, false
}

// enableMultilibRepo turns the stock file's commented multilib section into a
// live one and returns pacman.conf as it now reads.
//
// multilib is the only source of steam and of the lib32-* libraries it depends
// on, and the package manager's Browse lists exactly the repositories
// pacman.conf names (package-installer/src/db.rs::read_config_repos) — a
// commented section is an invisible one, so an install that left it commented
// could not list or install any of it without the user editing a config file.
// The mirrorlist this installer writes carries $repo, so core, extra and
// multilib are answered by the same mirrors and the section is the two lines the
// stock file already ships, uncommented. No SigLevel line is written: multilib is
// signed with the Arch packager keys the system already trusts, so the stock
// example's "Optional TrustAll" would only weaken a check that works.
func enableMultilibRepo(ctx *shared.InstallContext, content string) (string, error) {
	if hasPacmanSection(content, shared.PacmanMultilibRepoName) {
		if ctx.Logger != nil {
			ctx.Logger.Info("pacman repository [%s] already enabled", shared.PacmanMultilibRepoName)
		}
		return content, nil
	}

	updated, found := uncommentPacmanSection(content, shared.PacmanMultilibRepoName)
	if !found {
		// Nothing commented to uncomment — a pacman.conf someone edited by hand.
		// The section has to be written instead, on the same mirrorlist.
		updated += "\n[" + shared.PacmanMultilibRepoName + "]\n" +
			"Include = " + shared.PacmanMirrorListPath + "\n"
	}
	if err := primitives.WriteFile(shared.PacmanConfPath, updated); err != nil {
		return "", fmt.Errorf("failed to enable [%s] in pacman.conf: %w", shared.PacmanMultilibRepoName, err)
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("pacman repository [%s] enabled through %s",
			shared.PacmanMultilibRepoName, shared.PacmanMirrorListPath)
	}
	return updated, nil
}

// uncommentPacmanSection uncomments a commented-out repository section — its
// header and the commented lines up to the first blank one, which is where that
// example ends — and reports whether it found one. A section left without a
// Server or Include line gets the mirrorlist the rest of the file uses: pacman
// treats a repository with no download source as a config error, and a config
// error stops every pacman command on the system.
func uncommentPacmanSection(content, name string) (string, bool) {
	lines := strings.Split(content, "\n")
	for i := range lines {
		if strings.TrimSpace(lines[i]) != "#["+name+"]" {
			continue
		}
		lines[i] = "[" + name + "]"
		end, hasSource := i+1, false
		for end < len(lines) {
			line := strings.TrimSpace(lines[end])
			// A blank line, a real section header, or the next commented example
			// all mean this section's lines are behind us.
			if line == "" || strings.HasPrefix(line, "[") || strings.HasPrefix(line, "#[") {
				break
			}
			if strings.HasPrefix(line, "#") {
				line = strings.TrimPrefix(line, "#")
				lines[end] = line
			}
			if key, _, ok := strings.Cut(line, "="); ok {
				if k := strings.TrimSpace(key); strings.EqualFold(k, "Server") || strings.EqualFold(k, "Include") {
					hasSource = true
				}
			}
			end++
		}
		if !hasSource {
			lines = slices.Insert(lines, end, "Include = "+shared.PacmanMirrorListPath)
		}
		return strings.Join(lines, "\n"), true
	}
	return content, false
}

// hasPacmanSection reports whether pacman.conf already defines the section.
// Section headers must sit alone on their line, so an exact trimmed match is
// what pacman itself would see.
func hasPacmanSection(content, name string) bool {
	header := "[" + name + "]"
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == header {
			return true
		}
	}
	return false
}

// syncDBSuffixes are the per-repository files pacman writes in its sync
// directory: the package database and its file index.
var syncDBSuffixes = []string{".db", ".files"}

// dropForeignSyncDBs removes every sync database in the target whose
// repository the target's own pacman.conf does not name.
//
// The one that is always there is the live session's. `pacstrap -K` copies no
// pacman.conf and resolves every name through the live session's
// ([aelum-offline], live-iso/airootfs/etc/pacman.conf), and pacman writes the
// database it resolved from under the --root it was given — so a fresh install
// owns /var/lib/pacman/sync/aelum-offline.db for a repository its pacman.conf
// never mentions, and nothing removes it later: -Scc cleans the package cache,
// not the databases. pacman itself ignores the file (measured with pacman 7
// against two throwaway repositories: with zz-first.db in sync/ and only
// zz-second in the config, -Sl lists only zz-second). Anything that reads the
// directory instead of the config does not — which is how the package manager
// came to show one installed package twice, once as "Install" from
// [aelum-offline].
//
// Non-fatal by design: a leftover database costs a duplicate row in a GUI, not
// a broken system.
func dropForeignSyncDBs(ctx *shared.InstallContext, pacmanConf string) error {
	dir := shared.MountRoot + shared.PacmanSyncDir
	names, err := primitives.ReadDirNames(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // no sync directory: there is nothing to clean
		}
		return fmt.Errorf("failed to list %s: %w", dir, err)
	}

	for _, name := range names {
		repo := ""
		for _, suffix := range syncDBSuffixes {
			if rest, ok := strings.CutSuffix(name, suffix); ok {
				repo = rest
				break
			}
		}
		if repo == "" || hasPacmanSection(pacmanConf, repo) {
			continue
		}
		path := dir + "/" + name
		if err := primitives.RemoveAll(path); err != nil {
			return fmt.Errorf("failed to remove %s: %w", path, err)
		}
		if ctx.Logger != nil {
			ctx.Logger.Info("removed the live session's package database: %s", path)
		}
	}
	return nil
}

// trustAelumKey imports the release key into the target's pacman keyring and
// locally signs it. SigLevel = Required is only passable once the key is in
// the keyring with full trust, and pacman-key does not follow WKD — the key
// file itself is shipped by the live ISO and copied by install/isocopy.
func trustAelumKey(ctx *shared.InstallContext) error {
	// The file lands in the target via an isocopy job that is not Optional: a
	// live ISO without it is a broken image, and failing here names the cause
	// instead of surfacing as signature errors on the user's first pacman -S.
	if _, err := primitives.Stat(shared.MountRoot + shared.AelumKeyringPath); err != nil {
		return fmt.Errorf("the Aelum signing key is missing from the target (%s): %w", shared.AelumKeyringPath, err)
	}
	// Inside arch-chroot the argument is the chroot-relative path.
	output, err := primitives.RunChroot("pacman-key", "--add", shared.AelumKeyringPath)
	if err != nil {
		return fmt.Errorf("failed to add the Aelum signing key: %w: %s", err, string(output))
	}
	output, err = primitives.RunChroot("pacman-key", "--lsign-key", shared.AelumSigningKeyFingerprint)
	if err != nil {
		return fmt.Errorf("failed to locally sign the Aelum signing key: %w: %s", err, string(output))
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("Aelum signing key trusted in the target keyring: %s", shared.AelumSigningKeyFingerprint)
	}
	return nil
}

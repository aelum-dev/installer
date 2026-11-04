package configure

import (
	"fmt"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// Aelum package repository
// ============================================================================

// ConfigurePacmanRepo enables the [aelum] repository in the installed system's
// pacman.conf and trusts its signing key in the target's keyring. A fresh
// install can then install the distro's own applications with no hand-editing;
// the repository itself is published by repo/publish-repo.sh (repo/REPO.md).
//
// It runs on every filesystem — the repository is not part of the snapshot
// machinery — and it must run before ConfigureSnapper, because the "after
// installation" snapshot is the state a user restores to and has to carry both
// the config block and the trusted key.
func ConfigurePacmanRepo(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}

	if err := enablePacmanRepo(ctx); err != nil {
		return err
	}
	return trustAelumKey(ctx)
}

// enablePacmanRepo appends the repository block to the target's pacman.conf.
// The file comes from the pacman package via pacstrap; there is no drop-in
// directory to write instead (see the constants' note), so the append is the
// normal Arch way — a modified pacman-owned file keeps its edits across
// updates, pacman only drops a .pacnew to diff.
func enablePacmanRepo(ctx *shared.InstallContext) error {
	hostPath := shared.MountRoot + "/etc/pacman.conf"
	data, err := primitives.ReadFile(hostPath)
	if err != nil {
		return fmt.Errorf("failed to read pacman.conf: %w", err)
	}
	// Idempotent: re-running the installer over an existing system must not
	// stack a second [aelum] section (later sections would shadow the first).
	if hasPacmanSection(string(data), shared.PacmanRepoName) {
		if ctx.Logger != nil {
			ctx.Logger.Info("pacman repository [%s] already enabled", shared.PacmanRepoName)
		}
		return nil
	}

	block := "\n# Aelum repository - applications shipped by the distro.\n" +
		"# Signed with the Aelum release key (release@aelum.dev); the installer\n" +
		"# trusts that key in pacman's keyring, so no further setup is needed.\n" +
		fmt.Sprintf("[%s]\n", shared.PacmanRepoName) +
		"SigLevel = " + shared.PacmanRepoSigLevel + "\n" +
		"Server = " + shared.PacmanRepoServer + "\n"
	if err := primitives.AppendFile("/etc/pacman.conf", block); err != nil {
		return err
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("pacman repository [%s] enabled: %s", shared.PacmanRepoName, shared.PacmanRepoServer)
	}
	return nil
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

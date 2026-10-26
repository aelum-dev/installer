package installbase

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/progress"
	"aelum-installer/backend/shared"
)

// basePackages are the essential system packages installed during the base
// system step. Kept minimal so this step finishes quickly and the progress
// bar moves smoothly.
var BasePackages = []string{
	// Base system
	"base",
	"linux-lts",
	"linux-firmware",
	"mkinitcpio",
	"edk2-shell",
	"memtest86+-efi",

	// Utilities
	"util-linux",
	"gptfdisk",
	"parted",
	"dosfstools",
	"e2fsprogs",
	"btrfs-progs",
	"lvm2",
	"pv",

	// System
	"sudo",
	"networkmanager",
	"iwd",
	"bluez",
	"power-profiles-daemon",
	"zram-generator",

	// Bootloader (installed onto the ESP by the configure step; efibootmgr is
	// pulled in as its dependency)
	"refind",

	// Editor
	"vim",
	"nano",
}

// BtrfsOnlyBasePackages extend the base set only when the install chose btrfs.
// snapper is the snapshot engine the configure step configures for the subvolume
// layout (see configure.ConfigureSnapper); an ext4 install has no subvolumes to
// snapshot, so it gets neither snapper nor its GUI.
var BtrfsOnlyBasePackages = []string{
	"snapper",
}

// AppendBtrfsOnly returns packages plus extra when the install chose btrfs, and
// packages unchanged otherwise. Exported for installdesktop, which gates
// btrfs-assistant the same way.
//
// A btrfs-only package must also be listed in the live ISO's package list
// (live-iso/packages.x86_64): the installer installs from the mirror embedded in
// the ISO, so a package that is not in that mirror cannot be installed.
func AppendBtrfsOnly(ctx *shared.InstallContext, packages, extra []string) []string {
	if ctx.FileSystem != shared.FileSystemBtrfs {
		return packages
	}
	out := make([]string, 0, len(packages)+len(extra))
	out = append(out, packages...)
	return append(out, extra...)
}

// Run installs the Arch Linux base system via pacstrap.
func Run(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}

	return InstallPackages(ctx, AppendBtrfsOnly(ctx, BasePackages, BtrfsOnlyBasePackages), "base system")
}

// InstallPackages runs pacstrap with the given package list.
// Streams stdout to parse real progress from pacstrap's output lines.
// Exported for use by installdesktop package.
func InstallPackages(ctx *shared.InstallContext, packages []string, description string) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}

	if len(packages) == 0 {
		return fmt.Errorf("no packages to install")
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Installing %d %s packages via pacstrap", len(packages), description)
	}

	args := []string{"-K", shared.MountRoot}
	args = append(args, packages...)
	cmd := exec.Command("pacstrap", args...)

	// Stream stdout to parse real progress from pacstrap's lines.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Capture stderr for crash reporting when pacstrap fails.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	var stderrBuf strings.Builder
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		buf := bufio.NewReader(stderr)
		for {
			line, err := buf.ReadString('\n')
			if line != "" {
				stderrBuf.WriteString(line)
			}
			if err != nil {
				break
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start pacstrap: %w", err)
	}

	// Track progress across pacstrap's multiple phases.
	tracker := progress.NewPacstrapProgressTracker()

	// Scan stdout with a bufio.Reader instead of bufio.Scanner: pacstrap's
	// "Packages (N) pkg1 pkg2 ..." line grows with the package list, and Scanner
	// aborts on a token larger than its buffer. That would stop draining the
	// pipe, leaving pacstrap blocked on a full stdout pipe forever.
	scanDone := make(chan struct{})
	var scanErr error
	go func() {
		defer close(scanDone)
		reader := bufio.NewReaderSize(stdout, 64*1024)
		for {
			line, err := reader.ReadString('\n')
			if line != "" {
				line = strings.TrimSuffix(line, "\n")
				line = strings.TrimSuffix(line, "\r")

				// Process all line types: Packages count, downloading, verifying,
				// installing, and (N/M) hook progress.
				pct := tracker.ProcessLine(line)
				if ctx.ProgressCallback != nil {
					ctx.ProgressCallback(pct)
				}

				// Command output appends to the progress line via => prefix.
				if ctx.Logger != nil {
					ctx.Logger.Command(line)
				}
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					scanErr = err
				}
				return
			}
		}
	}()

	select {
	case <-scanDone:
		if scanErr != nil {
			// Stop the child: with its stdout reader gone it could block on a
			// full pipe and never exit.
			_ = cmd.Process.Kill()
		}
		// Wait for the stderr reader to finish before reading stderrBuf: the
		// strings.Builder it writes is not safe for concurrent access.
		<-stderrDone
		waitErr := cmd.Wait()
		if scanErr != nil {
			return fmt.Errorf("pacstrap stdout scan: %w", scanErr)
		}
		if waitErr != nil {
			errMsg := strings.TrimSpace(stderrBuf.String())
			if errMsg != "" {
				return fmt.Errorf("pacstrap failed:\n%s", errMsg)
			}
			return fmt.Errorf("pacstrap failed: %w", waitErr)
		}
		return nil
	case <-ctx.Cancel:
		// Kill the child process so the scanner goroutine unblocks and exits.
		// Cleanup is handled by the Installer's defer, not here.
		_ = cmd.Process.Kill()
		<-scanDone
		<-stderrDone
		// Reap the killed child so it does not linger as a zombie.
		_ = cmd.Wait()
		return fmt.Errorf("interrupted")
	}
}

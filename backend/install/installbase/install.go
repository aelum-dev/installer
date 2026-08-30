package installbase

import (
	"bufio"
	"fmt"
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
	"pv",

	// System
	"sudo",
	"networkmanager",
	"iwd",
	"bluez",
	"power-profiles-daemon",
	"zram-generator",

	// Editor
	"vim",
	"nano",
}

// Run installs the Arch Linux base system via pacstrap.
func Run(ctx *shared.InstallContext) error {
	return InstallPackages(ctx, BasePackages, "base system")
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
	go func() {
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

	// Large buffer for the "Packages (N) pkg1 pkg2 ..." line which can exceed
	// the default 64KB scanner buffer when installing many packages.
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	// Run scanner in a goroutine so we can select on ctx.cancel for clean
	// interrupt handling. Without this, SIGINT would leave the scanner
	// blocked on Read() until pacstrap exits on its own.
	scanDone := make(chan struct{})
	var scanErr error
	go func() {
		for scanner.Scan() {
			line := scanner.Text()

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
		scanErr = scanner.Err()
		close(scanDone)
	}()

	select {
	case <-scanDone:
		if scanErr != nil {
			return fmt.Errorf("pacstrap stdout scan: %w", scanErr)
		}
		if err := cmd.Wait(); err != nil {
			errMsg := strings.TrimSpace(stderrBuf.String())
			if errMsg != "" {
				return fmt.Errorf("pacstrap failed:\n%s", errMsg)
			}
			return fmt.Errorf("pacstrap failed: %w", err)
		}
		return nil
	case <-ctx.Cancel:
		// Kill the child process so the scanner goroutine unblocks and exits.
		// Cleanup is handled by the Installer's defer, not here.
		_ = cmd.Process.Kill()
		<-scanDone
		// Reap the killed child so it does not linger as a zombie.
		_ = cmd.Wait()
		return fmt.Errorf("interrupted")
	}
}

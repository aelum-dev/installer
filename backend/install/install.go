package install

import (
	"bufio"
	"fmt"
	"os/exec"

	"aestheticos-installer/backend/install/progress"
	"aestheticos-installer/backend/shared"
)

// installPackages runs pacstrap with the given package list.
// Streams stdout to parse real progress from pacstrap's output lines.
func installPackages(ctx *InstallContext, packages []string, description string) error {
	if err := validateMountedSystem(); err != nil {
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
			if ctx.Logger != nil {
				ctx.Logger.Command(line)
			}

			// Process all line types: Packages count, downloading, verifying,
			// installing, and (N/M) hook progress.
			pct := tracker.ProcessLine(line)
			if ctx.ProgressCallback != nil {
				ctx.ProgressCallback(pct)
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
			return fmt.Errorf("pacstrap failed: %w", err)
		}
		return nil
	case <-ctx.Cancel:
		// Kill the child process so the scanner goroutine unblocks and exits.
		// Cleanup is handled by the Installer's defer, not here.
		_ = cmd.Process.Kill()
		<-scanDone
		return fmt.Errorf("interrupted")
	}
}



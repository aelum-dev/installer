// aestheticos-installer-cli is a CLI-based Arch Linux installer that performs automatic installation.
//
// Full installation:
//
//	./aestheticos-installer-cli --install /dev/vda [--dry-run]
//
// Individual steps (each takes a disk path):
//
//	./aestheticos-installer-cli --format-disk /dev/vda
//	./aestheticos-installer-cli --mount-filesystem /dev/vda
//	./aestheticos-installer-cli --install-system /dev/vda
//	./aestheticos-installer-cli --setup-users /dev/vda
//	./aestheticos-installer-cli --configure-system /dev/vda
//
// Password is read interactively from the terminal. For non-interactive use, pipe it via stdin:
//
//	echo 'mypassword' | ./aestheticos-installer-cli --install /dev/vda
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"aestheticos-installer/backend/internal/core"
	"aestheticos-installer/backend/internal/shared"

	"golang.org/x/term"
)

// Version is set at build time via ldflags.
var Version = "dev"

func main() {
	// Set custom usage function for --help
	flag.Usage = func() {
		printUsage()
	}

	// Full installation
	installFlag := flag.String("install", "", "Disk to install to (runs all steps)")

	// Individual step flags
	formatDiskFlag := flag.String("format-disk", "", "Disk to format")
	mountFsFlag := flag.String("mount-filesystem", "", "Disk whose partitions to mount")
	installSystemFlag := flag.String("install-system", "", "Disk to install base system to")
	setupUsersFlag := flag.String("setup-users", "", "Disk to set up users on")
	configureSysFlag := flag.String("configure-system", "", "Disk to configure system on")

	dryRun := flag.Bool("dry-run", false, "Simulate installation without making changes")
	rollbackFlag := flag.String("rollback", "", "Disk to roll back (clean up partial install and wipe partitions)")
	fileSystemFlag := flag.String("filesystem", "ext4", "Root filesystem type: ext4 or btrfs")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println(Version)
		os.Exit(0)
	}

	// Layout is now determined by filesystem type (ext4 or btrfs)
	// No separate layout flag needed

	// Rollback is handled separately — it doesn't use resolveDiskAndStep
	if *rollbackFlag != "" {
		fmt.Printf("Rolling back %s...\n", *rollbackFlag)
		if err := shared.ValidateDiskPath(*rollbackFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		helper := core.FindHelper()
		if helper == "" {
			fmt.Fprintf(os.Stderr, "Error: aestheticos-helper not found\n")
			os.Exit(1)
		}
		installer := core.NewInstaller(
			*rollbackFlag, helper, shared.DefaultCLIUsername, "", shared.DefaultHostname,
			shared.GetRegion(shared.DefaultLanguageCode), *fileSystemFlag, false,
		)
		if err := installer.Rollback(); err != nil {
			fmt.Fprintf(os.Stderr, "\n%s[ERROR]%s %v\n", "\033[31m", "\033[0m", err)
			os.Exit(1)
		}
		fmt.Printf("\n%s✓%s Rollback complete. %s is ready for a fresh install.\n", "\033[32m", "\033[0m", *rollbackFlag)
		return
	}

	// Determine which flag was provided (first one wins)
	disk, stepName := resolveDiskAndStep(resolveFlags(
		installFlag, "",
		formatDiskFlag, "format_disk",
		mountFsFlag, "mount_filesystem",
		installSystemFlag, "install",
		setupUsersFlag, "setup_users",
		configureSysFlag, "configure_system",
	)...)

	if disk == "" {
		printUsage()
		os.Exit(1)
	}

	if err := shared.ValidateDiskPath(disk); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := shared.ValidateDiskSize(disk); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Read password from stdin (never accept via flag — avoids /proc, ps, and shell history exposure)
	password, err := readPassword()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to read password: %v\n", err)
		os.Exit(1)
	}
	if password == "" {
		fmt.Fprintf(os.Stderr, "Error: password cannot be empty\n")
		os.Exit(1)
	}

	if *dryRun {
		fmt.Println("[DRY RUN MODE] Installation steps will be simulated.")
	}

	helper := core.FindHelper()
	if helper == "" {
		fmt.Fprintf(os.Stderr, "Error: aestheticos-helper not found\n")
		os.Exit(1)
	}
	fmt.Printf("Found aestheticos-helper: %s\n", helper)

	// Check if sudo is configured to run the helper without a password
	fmt.Println("\nChecking root privileges...")
	rootCheck := core.CheckRoot(helper)
	if !rootCheck.Authorized {
		fmt.Fprintf(os.Stderr, "Error: %s\n", rootCheck.Message)
		os.Exit(1)
	}
	fmt.Println("✓ Root privileges verified")

	installer := core.NewInstaller(
		disk,
		helper,
		shared.DefaultCLIUsername, // username
		password,                  // password (read from stdin)
		shared.DefaultHostname,    // hostname
		shared.GetRegion(shared.DefaultLanguageCode), // region (default)
		*fileSystemFlag,
		*dryRun,
	)

	// CLI progress output
	installer.OnProgress = func(label string, percent int, phase string) {
		fmt.Printf("\r[%3d%%] %s...", percent, label)
		if phase == "done" {
			fmt.Println(" ✓")
		}
	}
	installer.OnError = func(label, message string) {
		fmt.Fprintf(os.Stderr, "\n[ERROR] %s: %s\n", label, message)
	}

	// Ensure cleanup runs on panic or error.
	defer func() {
		if r := recover(); r != nil {
			installer.Cleanup()
			fmt.Fprintf(os.Stderr, "\n%s[ERROR]%s panic recovered: %v\n", "\033[31m", "\033[0m", r)
			os.Exit(1)
		}
	}()

	// Catch Ctrl+C (SIGINT). Cancel any running child process (e.g. pacstrap)
	// so the scanner unblocks, then let Install()'s defer handle cleanup.
	// We set the interrupted flag so Install()'s defer knows to skip its own
	// cleanup attempt, avoiding a race where both the signal handler and the
	// defer try to unmount simultaneously.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)
	go func() {
		<-sigChan
		installer.LogInfo("Interrupted — cancelling child process on %s", disk)
		installer.Cancel()
		installer.SetInterrupted()
	}()

	if stepName != "" {
		// Run a single step
		fmt.Println()
		fmt.Printf("Running step on %s...\n", disk)
		fmt.Println()
		_, _, err = installer.RunStep(stepName)
	} else {
		// Run full installation
		fmt.Println()
		fmt.Printf("Installing Arch Linux to %s...\n", disk)
		fmt.Println()
		err = installer.Install()
	}

	if err != nil {
		// If interrupted by SIGINT, exit with 130 (standard POSIX signal exit code)
		// instead of 1. The signal handler already logged the interruption and
		// performed cleanup, so don't print a duplicate error.
		if installer.Interrupted() {
			os.Exit(130)
		}
		fmt.Fprintf(os.Stderr, "\n%s[ERROR]%s %v\n", "\033[31m", "\033[0m", err)
		os.Exit(1)
	}

	fmt.Println()
	if stepName != "" {
		fmt.Printf("%s✓%s Step completed successfully!\n", "\033[32m", "\033[0m")
	} else {
		fmt.Printf("%s✓%s Installation completed successfully!\n", "\033[32m", "\033[0m")
	}
}

// resolveFlags converts flag pointers and their step names into a flat list.
// Each pair is (flagValue, stepName). Empty stepName means "full install".
func resolveFlags(pairs ...interface{}) []string {
	var result []string
	for i := 0; i < len(pairs); i += 2 {
		flagPtr := pairs[i].(*string)
		stepName := pairs[i+1].(string)
		result = append(result, *flagPtr, stepName)
	}
	return result
}

// resolveDiskAndStep checks the flags in priority order and returns the disk path
// and the internal step name. If no step flag is set, stepName is empty (full install).
func resolveDiskAndStep(args ...string) (disk string, stepName string) {
	for i := 0; i < len(args); i += 2 {
		d := args[i]
		name := args[i+1]
		if d != "" {
			return d, name
		}
	}
	return "", ""
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  %s --install <disk> [--filesystem ext4|btrfs] [--dry-run]   Full installation\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --rollback <disk>                                       Clean up partial install\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --format-disk <disk> [--dry-run]                        Format disk partitions\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --mount-filesystem <disk> [--dry-run]                   Mount partitions\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --install-system <disk> [--dry-run]                     Install base system\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --setup-users <disk> [--dry-run]                        Set up user accounts\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --configure-system <disk> [--dry-run]                   Configure system (fstab, bootloader, hostname, locale, greetd, plymouth, zram, services)\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Password: entered interactively from the terminal (never passed as a flag).\n")
	fmt.Fprintf(os.Stderr, "         Pipe a password via stdin for non-interactive use: echo 'pass' | %s --install /dev/vda\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Examples:\n")
	fmt.Fprintf(os.Stderr, "  %s --install /dev/vda --filesystem btrfs\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --install /dev/vda --filesystem ext4 --dry-run\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --rollback /dev/vda\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  echo 'mypassword' | %s --install /dev/vda\n", os.Args[0])
}

// readPassword prompts the user for a password on the terminal and returns it.
// If stdin is not a terminal (e.g. piped input), it reads the password from stdin directly.
// The password is never stored in a flag and therefore never appears in /proc, ps, or shell history.
func readPassword() (string, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintf(os.Stderr, "Enter password: ")
		bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr) // newline after masked input
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(bytes)), nil
	}
	// Non-interactive: read from stdin (piped or redirected)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
}

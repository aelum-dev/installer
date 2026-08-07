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

	"aestheticos-installer/backend/install"
	"aestheticos-installer/backend/shared"

	"golang.org/x/term"
)

// log is the shared logger used by the CLI for consistent output across CLI and GUI.
var log = install.GetLogger()

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

	// Check for root privileges early — all disk operations require it.
	if os.Getuid() != 0 {
		fmt.Fprintf(os.Stderr, "Error: this command requires root privileges.\n")
		fmt.Fprintf(os.Stderr, "Run with sudo: sudo %s %s\n", os.Args[0], strings.Join(os.Args[1:], " "))
		os.Exit(1)
	}

	// Layout is now determined by filesystem type (ext4 or btrfs)
	// No separate layout flag needed

	// Rollback is handled separately — it doesn't use resolveDiskAndStep
	if *rollbackFlag != "" {
		log.Info("Rolling back %s...", *rollbackFlag)
		if err := shared.ValidateDiskPath(*rollbackFlag); err != nil {
			log.Error("%v", err)
			os.Exit(1)
		}
		installer := install.NewInstaller(
			*rollbackFlag, shared.DefaultCLIUsername, "", shared.DefaultHostname,
			shared.GetRegion(shared.DefaultLanguageCode), *fileSystemFlag, false,
		)
		installer.Logger = log
		if err := installer.Rollback(); err != nil {
			log.Error("%v", err)
			os.Exit(1)
		}
		log.Info("Rollback complete. %s is ready for a fresh install.", *rollbackFlag)
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
		log.Error("%v", err)
		os.Exit(1)
	}

	if err := shared.ValidateDiskSize(disk); err != nil {
		log.Error("%v", err)
		// If lsblk failed with a permission/device error, give a more helpful hint.
		if strings.Contains(err.Error(), "exit status") {
			fmt.Fprintf(os.Stderr, "Hint: ensure the disk path is correct and you have permission to read it.\n")
		}
		os.Exit(1)
	}

	// Read password from stdin (never accept via flag — avoids /proc, ps, and shell history exposure)
	password, err := readPassword()
	if err != nil {
		log.Error("failed to read password: %v", err)
		os.Exit(1)
	}
	if password == "" {
		log.Error("password cannot be empty")
		os.Exit(1)
	}

	if *dryRun {
		log.Info("[DRY RUN MODE] Installation steps will be simulated.")
	}

	installer := install.NewInstaller(
		disk,
		shared.DefaultCLIUsername, // username
		password,                  // password (read from stdin)
		shared.DefaultHostname,    // hostname
		shared.GetRegion(shared.DefaultLanguageCode), // region (default)
		*fileSystemFlag,
		*dryRun,
	)

	// Wire the installer to use the shared logger.
	installer.Logger = log

	// JSON progress events go to stderr (GUI reads them there).
	// Human-readable output goes to stdout (terminal users see clean logs).
	installer.OnProgress = func(label string, percent int, phase string) {
		if phase == "done" {
			log.ProgressDone()
		} else {
			log.Progress(label, percent)
		}
	}
	installer.ProgressLine = func(label string, percent int) {
		log.ProgressLine(label, percent)
	}
	installer.OnError = func(label, message string) {
		log.Error("%s: %s", label, message)
	}

	// Ensure cleanup runs on panic or error.
	defer func() {
		if r := recover(); r != nil {
			installer.RunCleanup()
			log.Error("panic recovered: %v", r)
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
		log.Info("Interrupted — cancelling child process on %s", disk)
		installer.Cancel()
		installer.SetInterrupted()
	}()

	if stepName != "" {
		// Run a single step
		log.Info("Running step on %s...", disk)
		_, _, err = installer.RunStep(stepName)
	} else {
		// Run full installation
		log.Info("Installing Arch Linux to %s...", disk)
		err = installer.Install()
	}

	if err != nil {
		// If interrupted by SIGINT, exit with 130 (standard POSIX signal exit code)
		// instead of 1. The signal handler already logged the interruption and
		// performed cleanup, so don't print a duplicate error.
		if installer.Interrupted() {
			os.Exit(130)
		}
		log.Error("%v", err)
		os.Exit(1)
	}

	if stepName != "" {
		log.Info("Step completed successfully!")
	} else {
		log.Info("Installation completed successfully!")
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

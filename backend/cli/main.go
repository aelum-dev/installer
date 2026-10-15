// aelum-installer-cli is a CLI-based Arch Linux installer that performs automatic installation.
//
// Full installation:
//
//	./aelum-installer-cli --install /dev/vda [--dry-run]
//
// Individual steps (each takes a disk path):
//
//	./aelum-installer-cli --format-disk /dev/vda
//	./aelum-installer-cli --mount-filesystem /dev/vda
//	./aelum-installer-cli --install-system /dev/vda
//	./aelum-installer-cli --setup-users /dev/vda
//	./aelum-installer-cli --configure-system /dev/vda
//
// Password is read interactively from the terminal. For non-interactive use, pipe it via stdin:
//
//	echo 'mypassword' | ./aelum-installer-cli --install /dev/vda
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"aelum-installer/backend/install"
	"aelum-installer/backend/install/partition"
	"aelum-installer/backend/shared"

	"golang.org/x/term"
)

// log is the shared logger used by the CLI for consistent output across CLI and GUI.
var log = install.GetLogger()

// Version is set at build time via ldflags.
var Version = "dev"

// exitCode is set by run() to control the exit code. Made variable for testing.
var exitCode = func(code int) {
	os.Exit(code)
}

// readUsernameFunc is a variable for testing — allows mocking username reading.
var readUsernameFunc = readUsername

// readPasswordFunc is a variable for testing — allows mocking password reading.
var readPasswordFunc = readPassword

// validateDiskPathFunc is a variable for testing — allows mocking disk validation.
var validateDiskPathFunc = shared.ValidateDiskPath

// validateDiskSizeFunc is a variable for testing — allows mocking disk size validation.
var validateDiskSizeFunc = shared.ValidateDiskSize

// cleanupMountsFunc is a variable for testing — allows mocking cleanup.
var cleanupMountsFunc = install.CleanupMounts

// rollbackFunc is a variable for testing — allows mocking rollback.
var rollbackFunc = func(logger install.Logger, diskID string) error {
	return install.Rollback(logger, diskID)
}

// run performs the CLI installation logic. Returns an exit code (0 for success,
// 1 for error, 130 for SIGINT). This is separated from main() to enable testing.
func run(args []string) int {
	// Save and restore os.Args for proper flag parsing in tests.
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = args

	// Create a new flag set for each run to avoid flag redefinition in tests.
	fs := flag.NewFlagSet("aelum-installer-cli", flag.ContinueOnError)

	// Set custom usage function for --help
	fs.Usage = func() {
		printUsage()
	}

	// Full installation
	installFlag := fs.String("install", "", "Disk to install to (runs all steps)")

	// Individual step flags
	formatDiskFlag := fs.String("format-disk", "", "Disk to format")
	mountFsFlag := fs.String("mount-filesystem", "", "Disk whose partitions to mount")
	installSystemFlag := fs.String("install-system", "", "Disk to install base system to")
	setupUsersFlag := fs.String("setup-users", "", "Disk to set up users on")
	configureSysFlag := fs.String("configure-system", "", "Disk to configure system on")
	verifySystemFlag := fs.String("verify-system", "", "Disk to verify after install (runs only the verification step)")

	dryRun := fs.Bool("dry-run", false, "Simulate installation without making changes")
	rollbackFlag := fs.String("rollback", "", "Disk to roll back (clean up partial install and wipe partitions)")
	cleanupFlag := fs.Bool("cleanup", false, "Unmount /mnt and clean up stale mounts")
	fileSystemFlag := fs.String("filesystem", shared.DefaultFileSystem, "Root filesystem type: ext4 or btrfs")
	languageFlag := fs.String("language", "", "Language code (e.g. en_US, de_DE)")
	usernameFlag := fs.String("username", "", "Username for the new user account")
	manualLayoutFlag := fs.String("manual-layout", "", "JSON manual partition layout for dualboot installs")
	luksFlag := fs.Bool("luks", false, "Encrypt the root partition with LUKS2 (prompts for a passphrase on stdin)")
	lvmFlag := fs.Bool("lvm", false, "Place root and home on LVM logical volumes (aelum VG)")
	versionFlag := fs.Bool("version", false, "Print version and exit")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		return 1
	}

	if *versionFlag {
		fmt.Println(Version)
		return 0
	}

	// Check for root privileges early — all disk operations require it.
	// Skip in dry-run mode — no real privileged commands are executed.
	if !*dryRun && os.Getuid() != 0 {
		fmt.Fprintf(os.Stderr, "Error: this command requires root privileges.\n")
		fmt.Fprintf(os.Stderr, "Run with sudo: sudo %s %s\n", os.Args[0], strings.Join(os.Args[1:], " "))
		return 1
	}

	// Layout is now determined by filesystem type (ext4 or btrfs)
	// No separate layout flag needed

	// Cleanup — unmount /mnt without any other operations.
	// Skip in dry-run mode — no real mounts exist.
	if *cleanupFlag {
		return handleCleanup(dryRun)
	}

	// Rollback is handled separately — it doesn't use resolveDiskAndStep
	// Skip in dry-run mode — no real disk operations.
	if *rollbackFlag != "" {
		return handleRollback(*rollbackFlag, dryRun)
	}

	// Determine which flag was provided (first one wins)
	disk, stepName := resolveDiskAndStep(resolveFlags(
		installFlag, "",
		formatDiskFlag, "format_disk",
		mountFsFlag, "mount_filesystem",
		installSystemFlag, "install",
		setupUsersFlag, "setup_users",
		configureSysFlag, "configure_system",
		verifySystemFlag, "verify_system",
	)...)

	if disk == "" {
		printUsage()
		return 1
	}

	// Resolve username: CLI flag takes priority; otherwise read from stdin.
	// In dry-run mode, skip username — no real accounts are created.
	username, err := resolveUsername(*usernameFlag, *dryRun)
	if err != nil {
		return 1
	}

	// Run the installation
	return runInstallation(disk, stepName, username, *fileSystemFlag, *languageFlag, *dryRun, *manualLayoutFlag, *luksFlag, *lvmFlag)
}

// resolveUsername resolves the username from flag or stdin.
func resolveUsername(usernameFlag string, dryRun bool) (string, error) {
	if !dryRun {
		if usernameFlag == "" {
			username, err := readUsernameFunc()
			if err != nil {
				log.Error("failed to read username: %v", err)
				return "", err
			}
			return username, nil
		}
		return usernameFlag, nil
	}
	return usernameFlag, nil
}

// runInstallation performs validation and runs the installation.
func runInstallation(disk, stepName, username, fileSystem, language string, dryRun bool, manualLayoutJSON string, luks, lvm bool) int {
	if err := validateDiskPathFunc(disk); err != nil {
		log.Error("%v", err)
		return 1
	}

	// Skip size validation in dry-run mode — no real disk is used.
	if !dryRun {
		if err := validateDiskSizeFunc(disk); err != nil {
			log.Error("%v", err)
			// If lsblk failed with a permission/device error, give a more helpful hint.
			if strings.Contains(err.Error(), "exit status") {
				fmt.Fprintf(os.Stderr, "Hint: ensure the disk path is correct and you have permission to read it.\n")
			}
			return 1
		}
	}

	// Read password from stdin (never accept via flag — avoids /proc, ps, and shell history exposure).
	// In dry-run mode, skip password — no real accounts are created.
	var password string
	var pwErr error
	// The verification step only reads the installed system; it creates nothing,
	// so it needs no password. Skip the read for it (and for dry-run).
	if !dryRun && stepName != "verify_system" {
		password, pwErr = readPasswordFunc()
		if pwErr != nil {
			log.Error("failed to read password: %v", pwErr)
			return 1
		}
		if password == "" {
			log.Error("password cannot be empty")
			return 1
		}
	}

	if dryRun {
		log.Info("[DRY RUN MODE] Installation steps will be simulated.")
	}

	// LUKS passphrase: read after the account password, matching the CLI's
	// sequential stdin reads (username flag → account pw → LUKS pw). Skipped in
	// dry-run and for the read-only verify step.
	var luksPassphrase string
	if luks && !dryRun && stepName != "verify_system" {
		luksPassphrase, pwErr = readPasswordFunc()
		if pwErr != nil {
			log.Error("failed to read LUKS passphrase: %v", pwErr)
			return 1
		}
		if luksPassphrase == "" {
			log.Error("LUKS passphrase cannot be empty")
			return 1
		}
	}

	lang := language
	if lang == "" {
		lang = shared.DefaultLanguageCode
	}
	installer := install.NewInstaller(
		disk,
		username,               // username (from --username flag or stdin)
		password,               // password (read from stdin)
		shared.DefaultHostname, // hostname
		shared.GetRegion(lang), // region (from --language or default)
		fileSystem,
		luks,           // encrypt root with LUKS2
		luksPassphrase, // LUKS passphrase (read from stdin)
		lvm,            // place root/home on LVM logical volumes
		dryRun,
	)

	// Wire the installer to use the shared logger.
	installer.Logger = log

	// Manual (dualboot) mode: parse and validate the JSON layout, then hand it
	// to the installer so the format step only touches the Boot/Root partitions.
	if manualLayoutJSON != "" {
		layout, err := parseManualLayout(manualLayoutJSON)
		if err != nil {
			log.Error("invalid manual layout: %v", err)
			return 1
		}
		if err := partition.ValidateFeasible(*layout); err != nil {
			log.Error("%v", err)
			return 1
		}
		installer.SetManualLayout(layout)
	}

	// JSON progress events go to stderr (GUI reads them there).
	// Human-readable output goes to stdout (terminal users see clean logs).
	installer.OnProgress = func(label string, percent int, phase string) {
		if phase == "done" {
			log.ProgressDone()
		} else {
			log.Progress(label, percent)
		}
	}

	// Ensure cleanup runs on panic or error.
	defer func() {
		if r := recover(); r != nil {
			installer.RunCleanup()
			log.Error("panic recovered: %v", r)
			os.Exit(1)
		}
	}()

	// If running in a terminal and not already interrupted, handle SIGINT so
	// Ctrl+C during a GUI-launched CLI installation also cancels and cleans up.
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
		_, _, err := installer.RunStep(stepName)
		if err != nil {
			if installer.Interrupted() {
				return 130
			}
			log.Error("%v", err)
			return 1
		}
		return 0
	}

	// Run full installation
	log.Info("Installing Aelum to %s...", disk)
	err := installer.Install()
	if err != nil {
		// If interrupted by SIGINT, exit with 130 (standard POSIX signal exit code)
		// instead of 1. The signal handler already logged the interruption and
		// performed cleanup, so don't print a duplicate error.
		if installer.Interrupted() {
			return 130
		}
		log.Error("%v", err)
		return 1
	}

	return 0
}

// handleCleanup performs cleanup operations (unmount /mnt).
func handleCleanup(dryRun *bool) int {
	if *dryRun {
		log.Info("Cleanup complete (dry-run).")
		return 0
	}
	if err := cleanupMountsFunc(log); err != nil {
		log.Error("%v", err)
		return 1
	}
	log.Info("Cleanup complete.")
	return 0
}

// handleRollback performs rollback operations (unmount and wipe partitions).
func handleRollback(disk string, dryRun *bool) int {
	if *dryRun {
		log.Info("Rollback complete (dry-run). %s is ready for a fresh install.", disk)
		return 0
	}
	log.Info("Rolling back %s...", disk)
	if err := validateDiskPathFunc(disk); err != nil {
		log.Error("%v", err)
		return 1
	}
	installer := install.NewInstaller(
		disk, shared.DefaultCLIUsername, "", shared.DefaultHostname,
		shared.GetRegion(shared.DefaultLanguageCode), "ext4", false, "", false, false,
	)
	installer.Logger = log
	if err := rollbackFunc(installer.Logger, disk); err != nil {
		log.Error("%v", err)
		return 1
	}
	log.Info("Rollback complete. %s is ready for a fresh install.", disk)
	return 0
}

func main() {
	exitCode(run(os.Args))
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
	fmt.Fprintf(os.Stderr, "  %s --install <disk> [--filesystem ext4|btrfs] [--language <code>] [--username <user>] [--luks] [--lvm] [--dry-run]   Full installation\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --rollback <disk>                                              Clean up partial install\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --format-disk <disk> [--dry-run]                               Format disk partitions\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --mount-filesystem <disk> [--dry-run]                          Mount partitions\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --install-system <disk> [--dry-run]                            Install base system\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --setup-users <disk> [--dry-run]                               Set up user accounts\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --configure-system <disk> [--dry-run]                          Configure system (fstab, bootloader, hostname, locale, greetd, plymouth, zram, services)\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --verify-system <disk> [--dry-run]                           Verify the installed system (fstab, kernel, packages, user, hostname)\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --cleanup                                                    Unmount /mnt and clean up stale mounts\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Username: passed via --username flag or read interactively from the terminal.\n")
	fmt.Fprintf(os.Stderr, "Password: entered interactively from the terminal (never passed as a flag).\n")
	fmt.Fprintf(os.Stderr, "         Pipe a password via stdin for non-interactive use: echo 'pass' | %s --install /dev/vda\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "When --luks is set, a second passphrase is read (same rules) and used to\n")
	fmt.Fprintf(os.Stderr, "encrypt the root partition with LUKS2. The passphrase is never a flag.\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "When --lvm is set, the root partition becomes an LVM physical volume in the\n")
	fmt.Fprintf(os.Stderr, "\"aelum\" volume group with root and home logical volumes (both btrfs).\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Examples:\n")
	fmt.Fprintf(os.Stderr, "  %s --install /dev/vda --filesystem btrfs --language de_DE\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --install /dev/vda --manual-layout '<json>'   Dualboot: only format the Boot/Root partitions in the layout\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --install /dev/vda --filesystem ext4 --dry-run\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --rollback /dev/vda\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  echo 'mypassword' | %s --install /dev/vda\n", os.Args[0])
}

// readPassword prompts the user for a password on the terminal and returns it.
// If stdin is not a terminal (e.g. piped input), it reads the password from stdin directly.
// The password is never stored in a flag and therefore never appears in /proc, ps, or shell history.
func readUsername() (string, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintf(os.Stderr, "Enter username: ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			return strings.TrimSpace(scanner.Text()), nil
		}
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("empty input")
	}
	// Non-interactive: read from stdin (piped or redirected). Use the shared
	// buffered reader so this line does not drain input meant for later reads.
	return readStdinLine()
}

// parseManualLayout decodes a manual partition layout from JSON. It is used by
// the --manual-layout flag for dualboot installs driven by the GUI.
func parseManualLayout(jsonStr string) (*shared.ManualLayout, error) {
	var layout shared.ManualLayout
	if err := json.Unmarshal([]byte(jsonStr), &layout); err != nil {
		return nil, fmt.Errorf("failed to parse manual layout JSON: %w", err)
	}
	return &layout, nil
}

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
	// Non-interactive: read from stdin (piped or redirected). Use the shared
	// buffered reader so this line does not drain input meant for later reads
	// (username → account password → LUKS passphrase).
	return readStdinLine()
}

// stdinReader is a single buffered reader shared by every non-interactive stdin
// read (username, account password, LUKS passphrase). Each bufio.Scanner/
// Reader issues its own read() syscall; the first one to touch a piped stdin
// drains the whole buffered chunk into its own buffer, leaving later reads at
// EOF. Reusing one reader lets each sequential read get its own line.
var stdinReader *bufio.Reader

func sharedStdin() *bufio.Reader {
	if stdinReader == nil {
		stdinReader = bufio.NewReader(os.Stdin)
	}
	return stdinReader
}

// readStdinLine returns the next line from the shared stdin buffer, trimmed of
// surrounding whitespace (and the trailing newline). A final line without a
// trailing newline is still returned; clean EOF with no data returns "", nil.
func readStdinLine() (string, error) {
	line, err := sharedStdin().ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

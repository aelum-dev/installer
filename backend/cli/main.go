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
//	./aestheticos-installer-cli --generate-fstab /dev/vda
//	./aestheticos-installer-cli --install-bootloader /dev/vda
//	./aestheticos-installer-cli --setup-users /dev/vda
//	./aestheticos-installer-cli --configure-system /dev/vda
package main

import (
	"flag"
	"fmt"
	"os"

	"aesthetic-os-installer/backend/core"
)

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
	genFstabFlag := flag.String("generate-fstab", "", "Disk to generate fstab for")
	bootloaderFlag := flag.String("install-bootloader", "", "Disk to install bootloader on")
	setupUsersFlag := flag.String("setup-users", "", "Disk to set up users on")
	configureSysFlag := flag.String("configure-system", "", "Disk to configure system on")

	dryRun := flag.Bool("dry-run", false, "Simulate installation without making changes")
	flag.Parse()
	// Pairs are (disk, stepName). Empty stepName means "full install".
	disk, stepName := resolveDiskAndStep(
		*installFlag,
		"",                  // full install = run all steps
		*formatDiskFlag,
		"format_disk",
		*mountFsFlag,
		"mount_filesystem",
		*installSystemFlag,
		"install",
		*genFstabFlag,
		"generate_fstab",
		*bootloaderFlag,
		"configure_bootloader",
		*setupUsersFlag,
		"setup_users",
		*configureSysFlag,
		"configure_system",
	)

	if disk == "" {
		printUsage()
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

	installer := core.NewInstaller(
		disk,
		helper,
		"aestheticos", // username
		"aestheticos", // password
		"en_US.UTF-8", // locale
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

	var err error
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
	fmt.Fprintf(os.Stderr, "  %s --install <disk> [--dry-run]                    Full installation\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --format-disk <disk> [--dry-run]                Format disk partitions\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --mount-filesystem <disk> [--dry-run]           Mount partitions\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --install-system <disk> [--dry-run]             Install base system\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --generate-fstab <disk> [--dry-run]             Generate fstab\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --install-bootloader <disk> [--dry-run]         Install bootloader\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --setup-users <disk> [--dry-run]                Set up user accounts\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s --configure-system <disk> [--dry-run]           Configure system\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Example:\n")
	fmt.Fprintf(os.Stderr, "  %s --install-bootloader /dev/vda\n", os.Args[0])
}

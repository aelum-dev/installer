// aestheticos-cli is a CLI-based Arch Linux installer that performs automatic installation.
// Usage: ./aestheticos-cli --install /dev/vda [--dry-run]
package main

import (
	"flag"
	"fmt"
	"os"

	"aesthetic-os-installer/backend/core"
)

func main() {
	installFlag := flag.String("install", "", "Disk to install to (e.g., /dev/vda)")
	dryRun := flag.Bool("dry-run", false, "Simulate installation without making changes")
	flag.Parse()

	if *installFlag == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s --install <disk> [--dry-run]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s --install /dev/vda\n", os.Args[0])
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
		*installFlag,
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

	fmt.Println()
	fmt.Printf("Installing Arch Linux to %s...\n", *installFlag)
	fmt.Println()

	err := installer.Install()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%s[ERROR]%s Installation failed: %v\n", "\033[31m", "\033[0m", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("%s✓%s Installation completed successfully!\n", "\033[32m", "\033[0m")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Remove the installation media")
	fmt.Println("  2. Boot from the new system")
}

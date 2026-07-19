// root-helper is a privileged helper binary that performs system operations.
// It is invoked via pkexec by the main aesthetic-os-installer GUI.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// HelperCommand represents a privileged operation the helper can execute.
type HelperCommand struct {
	Name  string
	Usage string
	Help  string
	Run   func(args []string) error
}

var commands = []HelperCommand{
	{
		Name:  "format",
		Usage: "format <disk>",
		Help:  "Format a disk with ext4",
		Run:   cmdFormat,
	},
	{
		Name:  "mount",
		Usage: "mount <disk> <mountpoint>",
		Help:  "Mount a partition",
		Run:   cmdMount,
	},
	{
		Name:  "umount",
		Usage: "umount <mountpoint>",
		Help:  "Unmount a partition",
		Run:   cmdUmount,
	},
	{
		Name:  "pacstrap",
		Usage: "pacstrap <mountpoint> <packages...>",
		Help:  "Install base packages with pacstrap",
		Run:   cmdPacstrap,
	},
	{
		Name:  "genfstab",
		Usage: "genfstab <mountpoint>",
		Help:  "Generate fstab",
		Run:   cmdGenfstab,
	},
	{
		Name:  "grub-install",
		Usage: "grub-install <disk>",
		Help:  "Install GRUB bootloader",
		Run:   cmdGrubInstall,
	},
	{
		Name:  "grub-mkconfig",
		Usage: "grub-mkconfig -o <output>",
		Help:  "Generate GRUB config",
		Run:   cmdGrubMkconfig,
	},
	{
		Name:  "passwd",
		Usage: "passwd <username>",
		Help:  "Set user password",
		Run:   cmdPasswd,
	},
	{
		Name:  "useradd",
		Usage: "useradd <username>",
		Help:  "Create a new user",
		Run:   cmdUseradd,
	},
	{
		Name:  "run",
		Usage: "run <command> [args...]",
		Help:  "Run any command as root",
		Run:   cmdRun,
	},
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	for _, cmd := range commands {
		if cmd.Name == command {
			if err := cmd.Run(args); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
	printUsage()
	os.Exit(1)
}

func printUsage() {
	fmt.Println("Usage: root-helper <command> [args...]")
	fmt.Println()
	fmt.Println("Commands:")
	for _, cmd := range commands {
		fmt.Printf("  %-15s %s - %s\n", cmd.Name, cmd.Usage, cmd.Help)
	}
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}

func cmdFormat(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: format <disk>")
	}
	return runCommand("mkfs.ext4", "-F", "-L", "aesthetic-os", args[0])
}

func cmdMount(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: mount <disk> <mountpoint>")
	}
	return runCommand("mount", args[0], args[1])
}

func cmdUmount(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: umount <mountpoint>")
	}
	return runCommand("umount", args[0])
}

func cmdPacstrap(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: pacstrap <mountpoint> <packages...>")
	}
	cmdArgs := append([]string{args[0]}, args[1:]...)
	return runCommand("pacstrap", cmdArgs...)
}

func cmdGenfstab(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: genfstab <mountpoint>")
	}
	return runCommand("genfstab", "-U", args[0])
}

func cmdGrubInstall(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: grub-install <disk>")
	}
	return runCommand("grub-install", "--target=i386-pc", args[0])
}

func cmdGrubMkconfig(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: grub-mkconfig -o <output>")
	}
	// Find the -o flag and output path
	for i, arg := range args {
		if arg == "-o" && i+1 < len(args) {
			output := args[i+1]
			return runCommand("grub-mkconfig", "-o", output)
		}
	}
	return fmt.Errorf("usage: grub-mkconfig -o <output>")
}

func cmdPasswd(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: passwd <username>")
	}
	return runCommand("passwd", args[0])
}

func cmdUseradd(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: useradd <username>")
	}
	return runCommand("useradd", "-m", "-s", "/bin/bash", args[0])
}

func cmdRun(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: run <command> [args...]")
	}
	return runCommand(args[0], args[1:]...)
}

// HelperPath returns the path to the root-helper binary next to the main binary.
func HelperPath() string {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	dir := strings.TrimSuffix(exe, "aesthetic-os-installer")
	return dir + "root-helper"
}

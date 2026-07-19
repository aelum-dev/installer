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
		Name:  "parted",
		Usage: "parted <disk>",
		Help:  "Create partition table and partitions (EFI + root)",
		Run:   cmdParted,
	},
	{
		Name:  "format",
		Usage: "format <partition>",
		Help:  "Format a partition with ext4",
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
		Name:  "bootctl-install",
		Usage: "bootctl-install <esp>",
		Help:  "Install systemd-boot to ESP",
		Run:   cmdBootctlInstall,
	},
	{
		Name:  "bootctl-set-default",
		Usage: "bootctl-set-default <entry>",
		Help:  "Set default boot entry",
		Run:   cmdBootctlSetDefault,
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

func cmdParted(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: parted <disk>")
	}
	disk := args[0]
	// Create GPT partition table
	if err := runCommand("sgdisk", "-Z", disk); err != nil {
		return fmt.Errorf("failed to clear disk: %w", err)
	}
	// Create EFI partition (128MB, FAT32, boot flag)
	if err := runCommand("sgdisk", "-n", "1:0:0", "-t", "1:EF00", "-c", "1:EFI", disk); err != nil {
		return fmt.Errorf("failed to create EFI partition: %w", err)
	}
	// Create root partition (rest of disk, ext4)
	if err := runCommand("sgdisk", "-n", "2:0:0", "-t", "2:8300", "-c", "2:Root", disk); err != nil {
		return fmt.Errorf("failed to create root partition: %w", err)
	}
	// Re-read partition table
	return runCommand("partprobe", disk)
}

func cmdFormat(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: format <partition>")
	}
	partition := args[0]
	// Determine filesystem based on partition name
	if strings.HasSuffix(partition, "1") || strings.Contains(partition, "efi") || strings.Contains(partition, "EFI") {
		// EFI partition - FAT32
		return runCommand("mkfs.fat", "-F", "32", "-n", "EFI", partition)
	}
	// Root partition - ext4
	return runCommand("mkfs.ext4", "-F", "-L", "aesthetic-os", partition)
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

func cmdBootctlInstall(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: bootctl-install <esp>")
	}
	// Install systemd-boot to the EFI System Partition
	if err := runCommand("bootctl", "install", "--path="+args[0]); err != nil {
		return err
	}
	// Create default loader configuration
	return runCommand("bootctl", "setup", "--path="+args[0])
}

func cmdBootctlSetDefault(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: bootctl-set-default <entry>")
	}
	// Set the default boot entry
	loaderConfig := args[0] + "/boot/loader/loader.conf"
	content := "default aesthetic-os.conf\ntimeout 5\n"
	return os.WriteFile(loaderConfig, []byte(content), 0644)
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

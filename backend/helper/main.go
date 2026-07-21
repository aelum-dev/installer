// aestheticos-helper is a privileged helper binary that performs system operations.
// It is invoked via sudo by the main aestheticos-installer GUI.
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
		Usage: "format <partition> <fstype>",
		Help:  "Format a partition (fstype: ext4, btrfs)",
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
		Name:  "mountpoints",
		Usage: "mountpoints <device>",
		Help:  "List mountpoints for a device (one per line)",
		Run:   cmdMountpoints,
	},
	{
		Name:  "lsblk-partitions",
		Usage: "lsblk-partitions <device>",
		Help:  "List partitions with mountpoints (NAME|MOUNTPOINT per line)",
		Run:   cmdLsblkPartitions,
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
		Name:  "efibootmgr",
		Usage: "efibootmgr [list|delete <num>]",
		Help:  "Manage EFI boot entries (list or delete by number)",
		Run:   cmdEfibootmgr,
	},
	{
		Name:  "run",
		Usage: "run <command> [args...]",
		Help:  "Run any command as root",
		Run:   cmdRun,
	},
	{
		Name:  "reboot",
		Usage: "reboot",
		Help:  "Reboot the system",
		Run:   cmdReboot,
	},
	{
		Name:  "btrfs-subvol-create",
		Usage: "btrfs-subvol-create <mountpoint> <subvol...>",
		Help:  "Create btrfs subvolumes (e.g., @ and @home)",
		Run:   cmdBtrfsSubvolCreate,
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
	fmt.Println("Usage: aestheticos-helper <command> [args...]")
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
	// Clear disk (removes partition table and all GPT data)
	if err := runCommand("sgdisk", "-Z", disk); err != nil {
		return fmt.Errorf("failed to clear disk: %w", err)
	}
	return nil
}

func cmdFormat(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: format <partition> <fstype>")
	}
	partition := args[0]
	fstype := args[1]

	// Determine filesystem based on partition name
	if strings.HasSuffix(partition, "1") || strings.Contains(partition, "efi") || strings.Contains(partition, "EFI") {
		// EFI partition - FAT32 (always)
		return runCommand("mkfs.fat", "-F", "32", "-n", "EFI", partition)
	}

	// Root partition - use specified filesystem type
	switch fstype {
	case "ext4":
		return runCommand("mkfs.ext4", "-F", "-L", "aestheticos", partition)
	case "btrfs":
		return runCommand("mkfs.btrfs", "-f", "-L", "aestheticos", partition)
	default:
		return fmt.Errorf("unsupported filesystem: %s (supported: ext4, btrfs)", fstype)
	}
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
	// Use -l (lazy) to unmount even if busy
	return runCommand("umount", "-l", args[0])
}

func cmdMountpoints(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: mountpoints <device>")
	}
	device := args[0]
	// Use lsblk to get all mountpoints for the disk and its children
	cmd := exec.Command("lsblk", "-lnbo", "MOUNTPOINT", device)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, mp := range lines {
		mp = strings.TrimSpace(mp)
		if mp != "" && mp != "[" {
			fmt.Println(mp)
		}
	}
	return nil
}

func cmdLsblkPartitions(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: lsblk-partitions <device>")
	}
	device := args[0]
	// Output: NAME|MOUNTPOINT for each partition
	cmd := exec.Command("lsblk", "-ln", "-o", "NAME,MOUNTPOINT", device)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "NAME") {
			continue
		}
		// Parse: NAME     MOUNTPOINT (whitespace-separated, may have trailing -)
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			mountpoint := fields[1]
			if mountpoint != "-" {
				fmt.Println(mountpoint)
			}
		}
	}
	return nil
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
	// Install systemd-boot to the EFI System Partition.
	// Per bootctl(1): "--esp-path= Path to the EFI System Partition (ESP)."
	return runCommand("bootctl", "install", "--esp-path="+args[0])
}

func cmdBootctlSetDefault(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: bootctl-set-default <esp> <entry-id>")
	}
	// Per bootctl(1): "set-default ID — Sets the default boot loader entry."
	return runCommand("bootctl", "set-default", "--esp-path="+args[0], args[1])
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

func cmdReboot(args []string) error {
	return runCommand("reboot")
}

func cmdEfibootmgr(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: efibootmgr [list|delete <num>]")
	}
	subcmd := args[0]
	switch subcmd {
	case "list":
		// List all EFI boot entries, output: BOOTNUM|Title
		cmd := exec.Command("efibootmgr", "-v")
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("efibootmgr list failed: %w", err)
		}
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// Parse lines like: Boot0009* Linux Boot Manager\t\tPCIms\t\tVEN=...\t\tUUID=...
			// We want to extract the boot number and title
			if strings.HasPrefix(line, "Boot") && strings.Contains(line, "*") {
				// Extract boot number: Boot0009*
				bootNum := ""
				title := ""
				for i, ch := range line {
					if ch >= '0' && ch <= '9' {
						bootNum += string(ch)
					} else if ch == '*' {
						// Rest is the title (possibly with tabs/spaces)
						title = strings.ReplaceAll(strings.TrimSpace(line[i+1:]), "\t", " ")
						break
					}
				}
				if bootNum != "" {
					fmt.Printf("%s|%s\n", bootNum, title)
				}
			}
		}
		return nil
	case "delete":
		if len(args) < 2 {
			return fmt.Errorf("usage: efibootmgr delete <num>")
		}
		bootNum := args[1]
		cmd := exec.Command("efibootmgr", "--delete-bootnum", "--bootnum", bootNum)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("efibootmgr delete Boot%s failed: %w: %s", bootNum, err, string(output))
		}
		fmt.Printf("Deleted Boot%s\n", bootNum)
		return nil
	default:
		return fmt.Errorf("unknown efibootmgr subcommand: %s (use list or delete)", subcmd)
	}
}

func cmdBtrfsSubvolCreate(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: btrfs-subvol-create <mountpoint> <subvol1> [subvol2 ...]")
	}
	mountpoint := args[0]
	subvols := args[1:]

	for _, subvol := range subvols {
		// Create subvolume (e.g., "@" or "@home")
		cmd := exec.Command("btrfs", "subvolume", "create", mountpoint+"/"+subvol)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to create subvolume %s: %w", subvol, err)
		}
		fmt.Printf("Created subvolume: %s/%s\n", mountpoint, subvol)
	}
	return nil
}

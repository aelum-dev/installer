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
		Usage: "format <partition> <fstype> [options...]",
		Help:  "Format a partition (fstype: vfat, ext4, btrfs)",
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
		Usage: "umount [-R] <mountpoint>",
		Help:  "Unmount a partition (lazy). Use -R for recursive",
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
	{
		Name:  "btrfs-subvol-set-default",
		Usage: "btrfs-subvol-set-default <mountpoint> <subvol>",
		Help:  "Set the default btrfs subvolume (e.g., @)",
		Run:   cmdBtrfsSubvolSetDefault,
	},
	{
		Name:  "btrfs-set-compression",
		Usage: "btrfs-set-compression <mountpoint> <subvol> <algorithm>",
		Help:  "Set persistent compression on a btrfs subvolume (e.g., zstd)",
		Run:   cmdBtrfsSetCompression,
	},
	{
		Name:  "udev-settle",
		Usage: "udev-settle",
		Help:  "Wait for udev to finish processing all events",
		Run:   cmdUdevSettle,
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
		return fmt.Errorf("usage: format <partition> <fstype> [options...]")
	}
	partition := args[0]
	fstype := args[1]

	// Validate partition path
	if partition == "" || !(strings.HasPrefix(partition, "/dev/") || strings.HasPrefix(partition, "/mnt/")) {
		return fmt.Errorf("invalid partition path: %s", partition)
	}

	// Validate fstype
	if fstype != "vfat" && fstype != "ext4" && fstype != "btrfs" {
		return fmt.Errorf("unsupported filesystem: %s (supported: vfat, ext4, btrfs)", fstype)
	}

	// Collect any extra options from args[2:]
	extraArgs := args[2:]

	// Wipe filesystem signatures before formatting
	// Use -f (force) to bypass "device or resource busy" errors
	// wipefs also calls BLKRRPART ioctl to notify kernel of changes
	if err := runCommand("wipefs", "--all", "-f", partition); err != nil {
		return fmt.Errorf("failed to wipe filesystem signatures: %w", err)
	}

	// Format based on the fstype argument — trust the caller, don't guess from partition name
	switch fstype {
	case "vfat":
		cmdArgs := []string{"-F", "32", "-n", "EFI"}
		cmdArgs = append(cmdArgs, extraArgs...)
		cmdArgs = append(cmdArgs, partition)
		return runCommand("mkfs.fat", cmdArgs...)
	case "ext4":
		cmdArgs := []string{"-F", "-L", "aestheticos"}
		cmdArgs = append(cmdArgs, extraArgs...)
		cmdArgs = append(cmdArgs, partition)
		return runCommand("mkfs.ext4", cmdArgs...)
	case "btrfs":
		cmdArgs := []string{"-f", "-L", "aestheticos"}
		cmdArgs = append(cmdArgs, extraArgs...)
		cmdArgs = append(cmdArgs, partition)
		return runCommand("mkfs.btrfs", cmdArgs...)
	default:
		return fmt.Errorf("unsupported filesystem: %s (supported: vfat, ext4, btrfs)", fstype)
	}
}

func cmdMount(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: mount [-o options] <disk> <mountpoint>")
	}

	// Parse and validate arguments
	var options []string
	device := ""
	mountpoint := ""
	i := 0
	for i < len(args) {
		if args[i] == "-o" {
			if i+1 >= len(args) {
				return fmt.Errorf("usage: mount [-o options] <disk> <mountpoint>")
			}
			options = append(options, args[i+1])
			i += 2
		} else {
			if device == "" {
				device = args[i]
			} else if mountpoint == "" {
				mountpoint = args[i]
				i++
				break
			} else {
				return fmt.Errorf("too many arguments")
			}
			i++
		}
	}

	if device == "" || mountpoint == "" {
		return fmt.Errorf("usage: mount [-o options] <disk> <mountpoint>")
	}

	// Validate paths
	if !strings.HasPrefix(device, "/dev/") {
		return fmt.Errorf("invalid device path: %s", device)
	}
	if !strings.HasPrefix(mountpoint, "/") {
		return fmt.Errorf("invalid mountpoint: %s", mountpoint)
	}

	// Build mount command
	cmdArgs := []string{"mount"}
	if len(options) > 0 {
		cmdArgs = append(cmdArgs, "-o", strings.Join(options, ","))
	}
	cmdArgs = append(cmdArgs, device, mountpoint)
	return runCommand(cmdArgs[0], cmdArgs[1:]...)
}

func cmdUmount(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: umount [-R] <mountpoint>")
	}

	// Parse flags
	recursive := false
	mountpoint := ""
	for _, arg := range args {
		if arg == "-R" {
			recursive = true
		} else {
			mountpoint = arg
		}
	}

	if mountpoint == "" {
		return fmt.Errorf("usage: umount [-R] <mountpoint>")
	}

	// Build umount arguments: -l (lazy) always, -R (recursive) if requested
	umountArgs := []string{"-l"}
	if recursive {
		umountArgs = append(umountArgs, "-R")
	}
	umountArgs = append(umountArgs, mountpoint)

	return runCommand("umount", umountArgs...)
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
		// Using -p (parent reference) for efficiency — mirrors archinstall's approach
		// This creates the subvolume as a copy of its parent, which is faster
		cmd := exec.Command("btrfs", "subvolume", "create", "-p", mountpoint+"/"+subvol)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to create subvolume %s: %w", subvol, err)
		}
		fmt.Printf("Created subvolume: %s/%s\n", mountpoint, subvol)
	}
	return nil
}

func cmdBtrfsSubvolSetDefault(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: btrfs-subvol-set-default <mountpoint> <subvol>")
	}
	mountpoint := args[0]
	subvol := args[1]

	// Find the subvolume ID by listing and matching the name
	cmd := exec.Command("btrfs", "subvolume", "list", mountpoint)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list subvolumes: %w", err)
	}

	// Parse output to find the subvolume ID
	// Format: ID 258 gen 12345 top level 5 path @
	var targetID string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 5 && fields[len(fields)-1] == subvol {
			// fields[1] is the ID
			for i, f := range fields {
				if f == "path" && i+1 < len(fields) && fields[i+1] == subvol {
					targetID = fields[1]
					break
				}
			}
		}
	}

	if targetID == "" {
		return fmt.Errorf("subvolume %s not found in %s", subvol, mountpoint)
	}

	// Set as default subvolume
	cmd = exec.Command("btrfs", "subvolume", "set-default", targetID, mountpoint)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set default subvolume %s (ID %s): %w", subvol, targetID, err)
	}
	fmt.Printf("Set default subvolume: %s (ID %s)\n", subvol, targetID)
	return nil
}

func cmdBtrfsSetCompression(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: btrfs-set-compression <mountpoint> <subvol> <algorithm>")
	}
	mountpoint := args[0]
	subvol := args[1]
	algorithm := args[2] // e.g., "zstd"

	// Set compression property on the subvolume
	// This persists regardless of mount options
	cmd := exec.Command("btrfs", "property", "set", mountpoint+"/"+subvol, "compression", algorithm)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set compression on %s/%s: %w", mountpoint, subvol, err)
	}
	fmt.Printf("Set compression on %s/%s: %s\n", mountpoint, subvol, algorithm)
	return nil
}

func cmdUdevSettle(args []string) error {
	// Wait for udev to finish processing all pending events.
	// This ensures the kernel has fully registered new partition device nodes
	// before subsequent operations (like mkfs) try to access them.
	// This is the standard approach used by archinstall and other installers.
	return runCommand("udevadm", "settle")
}

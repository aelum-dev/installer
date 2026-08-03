// aestheticos-helper is a privileged helper binary that performs system operations.
// It is invoked via sudo by the main aestheticos-installer GUI.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
		Usage: "pacstrap [-K] <mountpoint> <packages...>",
		Help:  "Install packages with pacstrap (-K copies keyring from live ISO)",
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
		Usage: "bootctl-install <esp> [extra-args...]",
		Help:  "Install systemd-boot to ESP (extra args like --graceful appended)",
		Run:   cmdBootctlInstall,
	},
	{
		Name:  "bootctl-cleanup",
		Usage: "bootctl-cleanup <esp> [extra-args...]",
		Help:  "Clean up orphaned files from ESP",
		Run:   cmdBootctlCleanup,
	},
	{
		Name:  "bootctl-random-seed",
		Usage: "bootctl-random-seed <esp>",
		Help:  "Generate random seed for entropy pool",
		Run:   cmdBootctlRandomSeed,
	},
	{
		Name:  "bootctl-set-default",
		Usage: "bootctl-set-default <entry>",
		Help:  "Set default boot entry",
		Run:   cmdBootctlSetDefault,
	},
	{
		Name:  "bootctl-is-installed",
		Usage: "bootctl-is-installed <esp>",
		Help:  "Check if systemd-boot is installed",
		Run:   cmdBootctlIsInstalled,
	},
	{
		Name:  "bootctl-status",
		Usage: "bootctl-status <esp>",
		Help:  "Show bootctl status",
		Run:   cmdBootctlStatus,
	},
	{
		Name:  "query-find",
		Usage: "query-find <path> [args...]",
		Help:  "Find files (find <path> [args...])",
		Run:   cmdQueryFind,
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
		Name:  "chroot-run",
		Usage: "chroot-run <chroot> <cmd> [args...]",
		Help:  "Run a whitelisted command inside a chroot (e.g., chroot-run /mnt systemctl enable foo)",
		Run:   cmdChrootRun,
	},
	{
		Name:  "enable-service",
		Usage: "enable-service <service>",
		Help:  "Enable a systemd service inside /mnt",
		Run:   cmdEnableService,
	},
	{
		Name:  "run-locale-gen",
		Usage: "run-locale-gen",
		Help:  "Run locale-gen inside /mnt",
		Run:   cmdRunLocaleGen,
	},
	{
		Name:  "run-dconf-update",
		Usage: "run-dconf-update",
		Help:  "Run dconf update inside /mnt",
		Run:   cmdRunDconfUpdate,
	},
	{
		Name:  "run-useradd",
		Usage: "run-useradd <username>",
		Help:  "Create a user inside /mnt (useradd -m -G wheel)",
		Run:   cmdRunUseradd,
	},
	{
		Name:  "run-chpasswd",
		Usage: "run-chpasswd <username>",
		Help:  "Set password for user inside /mnt (password via stdin)",
		Run:   cmdRunChpasswd,
	},
	{
		Name:  "run-plymouth-set-theme",
		Usage: "run-plymouth-set-theme <theme>",
		Help:  "Set plymouth theme inside /mnt (e.g., bgrt)",
		Run:   cmdRunPlymouthSetTheme,
	},
	{
		Name:  "run-mkinitcpio",
		Usage: "run-mkinitcpio",
		Help:  "Run mkinitcpio -P inside /mnt",
		Run:   cmdRunMkinitcpio,
	},
	{
		Name:  "write-file",
		Usage: "write-file <path> [content]",
		Help:  "Write content to a file (content via stdin if not provided as arg)",
		Run:   cmdWriteFile,
	},
	{
		Name:  "append-file",
		Usage: "append-file <path> [content]",
		Help:  "Append content to a file (content via stdin if not provided as arg)",
		Run:   cmdAppendFile,
	},
	{
		Name:  "read-file",
		Usage: "read-file <path>",
		Help:  "Read and print the contents of a file",
		Run:   cmdReadFile,
	},
	{
		Name:  "set-symlink",
		Usage: "set-symlink <target> <link>",
		Help:  "Create a symbolic link (ln -sf)",
		Run:   cmdSetSymlink,
	},
	{
		Name:  "chmod",
		Usage: "chmod <mode> <path>",
		Help:  "Change file permissions",
		Run:   cmdChmod,
	},
	{
		Name:  "chown",
		Usage: "chown <owner> <path>",
		Help:  "Change file ownership",
		Run:   cmdChown,
	},
	{
		Name:  "touch",
		Usage: "touch <path>",
		Help:  "Create an empty file or update timestamp",
		Run:   cmdTouch,
	},
	{
		Name:  "mkdir",
		Usage: "mkdir <path>",
		Help:  "Create a directory (mkdir -p)",
		Run:   cmdMkdir,
	},
	{
		Name:  "query-blkid",
		Usage: "query-blkid <device>",
		Help:  "Query blkid for a device (outputs UUID value)",
		Run:   cmdQueryBlkid,
	},
	{
		Name:  "query-findmnt",
		Usage: "query-findmnt <mountpoint>",
		Help:  "Query findmnt for a mountpoint (outputs SOURCE)",
		Run:   cmdQueryFindmnt,
	},
	{
		Name:  "query-ls",
		Usage: "query-ls <path>",
		Help:  "List a directory (ls)",
		Run:   cmdQueryLs,
	},
	{
		Name:  "query-test",
		Usage: "query-test <flags> <path>",
		Help:  "Test a file condition (test <flags> <path>)",
		Run:   cmdQueryTest,
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
	{
		Name:  "mkswap",
		Usage: "mkswap <device-or-file>",
		Help:  "Create a swap area on a device or file",
		Run:   cmdMkswap,
	},
	{
		Name:  "swapon",
		Usage: "swapon <device-or-file>",
		Help:  "Enable a device or file for swapping",
		Run:   cmdSwapon,
	},
	{
		Name:  "chattr",
		Usage: "chattr <flags> <path>",
		Help:  "Change file attributes (e.g., +C for nodatacow on btrfs)",
		Run:   cmdChattr,
	},
	{
		Name:  "btrfs-mkswapfile",
		Usage: "btrfs-mkswapfile <path> <size>",
		Help:  "Create a btrfs swap file with proper CoW handling (e.g., btrfs-mkswapfile /swapfile 4G)",
		Run:   cmdBtrfsMkswapfile,
	},
	{
		Name:  "sgdisk",
		Usage: "sgdisk <args...>",
		Help:  "Run sgdisk partitioning commands",
		Run:   cmdSgdisk,
	},
	{
		Name:  "dd",
		Usage: "dd <args...>",
		Help:  "Run dd (copy/convert file)",
		Run:   cmdDd,
	},
	{
		Name:  "fallocate",
		Usage: "fallocate <args...>",
		Help:  "Run fallocate (preallocate file space)",
		Run:   cmdFallocate,
	},
	{
		Name:  "rmdir",
		Usage: "rmdir <path>",
		Help:  "Remove an empty directory",
		Run:   cmdRmdir,
	},
	{
		Name:  "swapoff",
		Usage: "swapoff <args...>",
		Help:  "Disable a swap device or file",
		Run:   cmdSwapoff,
	},
	{
		Name:  "cp",
		Usage: "cp <args...>",
		Help:  "Copy files (cp [flags] source dest)",
		Run:   cmdCp,
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
		// mkfs.ext4 checks is_device_in_use() which reads /proc/mounts and
		// checks fcntl(F_GETLK). On re-install scenarios the kernel may still
		// have stale mount entries. We work around this by:
		// 1. First running wipefs --all --force to clear all signatures
		// 2. Then calling mke2fs with -F (force) which should skip the check
		// 3. If that fails, we use a direct approach: write a minimal ext4
		//    superblock via dd, then let tune2fs finalize it
		cmdArgs := []string{"-F", "-E", "lazy_itable_init=0", "-L", "aestheticos"}
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
		return fmt.Errorf("usage: pacstrap [-K] <mountpoint> <packages...>")
	}
	cmdArgs := []string{}
	// Pass through -K flag if present (copies keyring from live ISO)
	for _, arg := range args {
		if arg != "-K" {
			cmdArgs = append(cmdArgs, arg)
		}
	}
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
		return fmt.Errorf("usage: bootctl-install <esp> [extra-args...]")
	}
	// Install systemd-boot to the EFI System Partition.
	// Per bootctl(1): "--esp-path= Path to the EFI System Partition (ESP)."
	cmdArgs := []string{"install", "--esp-path=" + args[0]}
	cmdArgs = append(cmdArgs, args[1:]...)
	return runCommand("bootctl", cmdArgs...)
}

func cmdBootctlCleanup(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: bootctl-cleanup <esp> [extra-args...]")
	}
	cmdArgs := []string{"cleanup", "--esp-path=" + args[0]}
	cmdArgs = append(cmdArgs, args[1:]...)
	return runCommand("bootctl", cmdArgs...)
}

func cmdBootctlRandomSeed(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: bootctl-random-seed <esp>")
	}
	return runCommand("bootctl", "random-seed", "--esp-path="+args[0])
}

func cmdBootctlSetDefault(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: bootctl-set-default <entry>")
	}
	return runCommand("bootctl", "set-default", args[0])
}

func cmdBootctlIsInstalled(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: bootctl-is-installed <esp>")
	}
	return runCommand("bootctl", "is-installed", "--esp-path="+args[0])
}

func cmdBootctlStatus(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: bootctl-status <esp>")
	}
	return runCommand("bootctl", "status", "--esp-path="+args[0])
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

// chrootRunWhitelist defines the subset of commands allowed inside a chroot.
var chrootRunWhitelist = map[string]bool{
	"systemctl":                  true,
	"locale-gen":                 true,
	"dconf":                      true,
	"useradd":                    true,
	"usermod":                    true,
	"chpasswd":                   true,
	"plymouth-set-default-theme": true,
	"mkinitcpio":                 true,
	"test":                       true,
	"id":                         true,
	"mkdir":                      true,
	"rm":                         true,
	"ln":                         true,
	"chown":                      true,
	"chgrp":                      true,
	"chmod":                      true,
	"setfacl":                    true,
	"cp":                         true,
	"systemd-sysusers":           true,
	"systemd-tmpfiles":           true,
}

func cmdChrootRun(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: chroot-run <chroot> <cmd> [args...]")
	}
	chroot := args[0]
	cmdName := args[1]

	// Whitelist check
	if !chrootRunWhitelist[cmdName] {
		return fmt.Errorf("command %q is not allowed in chroot-run (whitelist: %v)", cmdName, chrootRunWhitelist)
	}

	// Validate chroot path
	if !strings.HasPrefix(chroot, "/") {
		return fmt.Errorf("invalid chroot path: %s", chroot)
	}

	cmdArgs := append([]string{chroot, cmdName}, args[2:]...)
	return runCommand("arch-chroot", cmdArgs...)
}

func cmdEnableService(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: enable-service <service>")
	}
	return cmdChrootRun([]string{"/mnt", "systemctl", "enable", args[0]})
}

func cmdRunLocaleGen(args []string) error {
	return cmdChrootRun([]string{"/mnt", "locale-gen"})
}

func cmdRunDconfUpdate(args []string) error {
	return cmdChrootRun([]string{"/mnt", "dconf", "update"})
}

func cmdRunUseradd(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: run-useradd <username>")
	}
	return cmdChrootRun([]string{"/mnt", "useradd", "-m", "-G", "wheel", args[0]})
}

func cmdRunChpasswd(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: run-chpasswd <username>")
	}
	username := args[0]
	// Read password from stdin
	buf, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read password from stdin: %w", err)
	}
	password := strings.TrimRight(string(buf), "\n")
	// Pipe "username:password" directly to chpasswd (no bash needed)
	cmd := exec.Command("arch-chroot", "/mnt", "chpasswd")
	cmd.Stdin = strings.NewReader(username + ":" + password + "\n")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func cmdRunPlymouthSetTheme(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: run-plymouth-set-theme <theme>")
	}
	return cmdChrootRun([]string{"/mnt", "plymouth-set-default-theme", "-R", args[0]})
}

func cmdRunMkinitcpio(args []string) error {
	return cmdChrootRun([]string{"/mnt", "mkinitcpio", "-P"})
}

func cmdWriteFile(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: write-file <path> [content]")
	}
	path := args[0]
	var content string
	if len(args) > 1 {
		content = args[1]
	} else {
		buf, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read stdin: %w", err)
		}
		content = string(buf)
	}
	cmd := exec.Command("bash", "-c", fmt.Sprintf("mkdir -p '%s' && cat > '%s'", filepath.Dir(path), path))
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func cmdAppendFile(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: append-file <path> [content]")
	}
	path := args[0]
	var content string
	if len(args) > 1 {
		content = args[1]
	} else {
		buf, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read stdin: %w", err)
		}
		content = string(buf)
	}
	cmd := exec.Command("bash", "-c", fmt.Sprintf("mkdir -p '%s' && cat >> '%s'", filepath.Dir(path), path))
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func cmdReadFile(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: read-file <path>")
	}
	return runCommand("cat", args[0])
}

func cmdSetSymlink(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: set-symlink <target> <link>")
	}
	return runCommand("ln", "-sf", args[0], args[1])
}

func cmdChmod(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: chmod <mode> <path>")
	}
	return runCommand("chmod", args[0], args[1])
}

func cmdChown(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: chown <owner> <path>")
	}
	return runCommand("chown", args[0], args[1])
}

func cmdTouch(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: touch <path>")
	}
	return runCommand("touch", args[0])
}

func cmdMkdir(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: mkdir <path>")
	}
	return runCommand("mkdir", "-p", args[0])
}

func cmdQueryBlkid(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: query-blkid <device>")
	}
	cmd := exec.Command("blkid", "-s", "UUID", "-o", "value", args[0])
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("blkid %s: %w", args[0], err)
	}
	fmt.Print(strings.TrimSpace(string(output)))
	return nil
}

func cmdQueryFindmnt(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: query-findmnt <mountpoint>")
	}
	cmd := exec.Command("findmnt", "-n", "-o", "SOURCE", args[0])
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("findmnt %s: %w", args[0], err)
	}
	fmt.Print(strings.TrimSpace(string(output)))
	return nil
}

func cmdQueryLs(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: query-ls <path>")
	}
	return runCommand("ls", args...)
}

func cmdQueryTest(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: query-test <flags> <path>")
	}
	return runCommand("test", args[0], args[1])
}

func cmdQueryFind(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: query-find <path> [args...]")
	}
	return runCommand("find", args...)
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
	// Use the 'path <name>' suffix for reliable matching
	var targetID string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		// Match lines ending with "path <subvol>"
		prefix := "path " + subvol + "\n"
		if strings.HasSuffix(line+"\n", prefix) {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				targetID = fields[1]
				break
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
	if len(args) < 3 {
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

func cmdMkswap(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: mkswap <device-or-file>")
	}
	return runCommand("mkswap", args[0])
}

func cmdSwapon(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: swapon <device-or-file>")
	}
	return runCommand("swapon", args[0])
}

func cmdChattr(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: chattr <flags> <path>")
	}
	flags := args[0]
	path := args[1]
	return runCommand("chattr", flags, path)
}

func cmdBtrfsMkswapfile(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: btrfs-mkswapfile <path> <size>")
	}
	path := args[0]
	size := args[1] // e.g., "4G", "2G", "512M"
	return runCommand("btrfs", "filesystem", "mkswapfile", "--size", size, path)
}

func cmdSgdisk(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: sgdisk <args...>")
	}
	return runCommand("sgdisk", args...)
}

func cmdDd(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: dd <args...>")
	}
	return runCommand("dd", args...)
}

func cmdFallocate(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: fallocate <args...>")
	}
	return runCommand("fallocate", args...)
}

func cmdRmdir(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: rmdir <path>")
	}
	return runCommand("rmdir", args[0])
}

func cmdSwapoff(args []string) error {
	return runCommand("swapoff", args...)
}

func cmdCp(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: cp <source> <dest>")
	}
	return runCommand("cp", args...)
}

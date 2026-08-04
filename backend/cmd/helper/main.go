// aestheticos-helper is a privileged helper binary that performs system operations.
// It is invoked via sudo by the main aestheticos-installer GUI.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
		Name:  "bootctl",
		Usage: "bootctl <subcommand> <esp> [extra-args...]",
		Help:  "Run bootctl subcommand (install|cleanup|random-seed|set-default|is-installed|status)",
		Run:   cmdBootctl,
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
		Name:  "run-chpasswd",
		Usage: "run-chpasswd <username>",
		Help:  "Set password for user inside /mnt (password via stdin)",
		Run:   cmdRunChpasswd,
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
		Name:  "btrfs-mkswapfile",
		Usage: "btrfs-mkswapfile <path> <size>",
		Help:  "Create a btrfs swap file with proper CoW handling (e.g., btrfs-mkswapfile /swapfile 4G)",
		Run:   cmdBtrfsMkswapfile,
	},
	{
		Name:  "udev-settle",
		Usage: "udev-settle",
		Help:  "Wait for udev to finish processing all events",
		Run:   cmdUdevSettle,
	},
	{
		Name:  "run",
		Usage: "run <command> [args...]",
		Help:  "Run a whitelisted host command (e.g., run blkid /dev/vda1)",
		Run:   cmdRun,
	},
}

// runWhitelist defines the subset of host commands allowed via the generic `run` command.
var runWhitelist = map[string]bool{
	// System queries
	"blkid":   true,
	"findmnt": true,
	"ls":      true,
	"test":    true,
	"find":    true,
	// File operations
	"chmod":     true,
	"chown":     true,
	"touch":     true,
	"mkdir":     true,
	"rmdir":     true,
	"cp":        true,
	"dd":        true,
	"fallocate": true,
	// Partitioning
	"sgdisk":    true,
	"wipefs":    true,
	"partprobe": true,
	// Swap
	"mkswap":  true,
	"swapon":  true,
	"swapoff": true,
	// System
	"reboot": true,
	// Users
	"passwd":  true,
	"useradd": true,
	// Attributes
	"chattr": true,
	// Bootloader
	"bootctl": true,
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
	if err := runCommand("wipefs", "--all", "-f", partition); err != nil {
		return fmt.Errorf("failed to wipe filesystem signatures: %w", err)
	}

	switch fstype {
	case "vfat":
		cmdArgs := []string{"-F", "32", "-n", "EFI"}
		cmdArgs = append(cmdArgs, extraArgs...)
		cmdArgs = append(cmdArgs, partition)
		return runCommand("mkfs.fat", cmdArgs...)
	case "ext4":
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

	if !strings.HasPrefix(device, "/dev/") {
		return fmt.Errorf("invalid device path: %s", device)
	}
	if !strings.HasPrefix(mountpoint, "/") {
		return fmt.Errorf("invalid mountpoint: %s", mountpoint)
	}

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
	if _, err := buildPacstrapArgs(args); err != nil {
		return err
	}
	return runCommand("pacstrap", args...)
}

// buildPacstrapArgs validates pacstrap arguments and returns the mountpoint.
// It ensures at least a mountpoint and one package are provided.
func buildPacstrapArgs(args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("usage: pacstrap [-K] <mountpoint> <packages...>")
	}
	return args[0], nil
}

func cmdGenfstab(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: genfstab <mountpoint>")
	}
	return runCommand("genfstab", "-U", args[0])
}

func cmdBootctl(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: bootctl <subcommand> <esp> [extra-args...]")
	}
	subcommand := args[0]
	esp := args[1]

	validSubcommands := map[string]bool{
		"install":      true,
		"cleanup":      true,
		"random-seed":  true,
		"set-default":  true,
		"is-installed": true,
		"status":       true,
	}
	if !validSubcommands[subcommand] {
		return fmt.Errorf("unknown bootctl subcommand: %s (valid: install, cleanup, random-seed, set-default, is-installed, status)", subcommand)
	}

	cmdArgs := []string{subcommand, "--esp-path=" + esp}
	cmdArgs = append(cmdArgs, args[2:]...)
	return runCommand("bootctl", cmdArgs...)
}

func cmdChrootRun(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: chroot-run <chroot> <cmd> [args...]")
	}
	chroot := args[0]
	cmdName := args[1]

	// Whitelist check
	chrootRunWhitelist := map[string]bool{
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
	if !chrootRunWhitelist[cmdName] {
		return fmt.Errorf("command %q is not allowed in chroot-run (whitelist: %v)", cmdName, chrootRunWhitelist)
	}

	if !strings.HasPrefix(chroot, "/") {
		return fmt.Errorf("invalid chroot path: %s", chroot)
	}

	cmdArgs := append([]string{chroot, cmdName}, args[2:]...)
	return runCommand("arch-chroot", cmdArgs...)
}

func cmdRunChpasswd(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: run-chpasswd <username>")
	}
	username := args[0]
	buf, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read password from stdin: %w", err)
	}
	password := strings.TrimRight(string(buf), "\n")
	cmd := exec.Command("arch-chroot", "/mnt", "chpasswd")
	cmd.Stdin = strings.NewReader(username + ":" + password + "\n")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(path), err)
	}
	// Write content directly — no shell involved
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
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
	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(path), err)
	}
	// Append content directly — no shell involved
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open %s for appending: %w", path, err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("failed to append to %s: %w", path, err)
	}
	return nil
}

func cmdReadFile(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: read-file <path>")
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}

func cmdSetSymlink(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: set-symlink <target> <link>")
	}
	return runCommand("ln", "-sf", args[0], args[1])
}

func cmdRun(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: run <command> [args...]")
	}
	cmdName := args[0]
	if !runWhitelist[cmdName] {
		return fmt.Errorf("command %q is not allowed in run (whitelist: %v)", cmdName, runWhitelist)
	}
	return runCommand(cmdName, args[1:]...)
}

func cmdEfibootmgr(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: efibootmgr [list|delete <num>]")
	}
	subcmd := args[0]
	switch subcmd {
	case "list":
		cmd := exec.Command("efibootmgr")
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("efibootmgr list failed: %w", err)
		}
		re := regexp.MustCompile(`^(Boot[0-9A-Fa-f]+)\*\t+(.+)$`)
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			matches := re.FindStringSubmatch(line)
			if matches != nil {
				bootNum := strings.TrimPrefix(strings.TrimSuffix(matches[1], "*"), "Boot")
				title := matches[2]
				fmt.Printf("%s|%s\n", bootNum, title)
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
		return fmt.Errorf("usage: btrfs-subvol-create <mountpoint> <subvol...>")
	}
	mountpoint := args[0]
	subvols := args[1:]

	for _, subvol := range subvols {
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

	cmd := exec.Command("btrfs", "subvolume", "list", mountpoint)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list subvolumes: %w", err)
	}

	var targetID string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
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
	algorithm := args[2]

	cmd := exec.Command("btrfs", "property", "set", mountpoint+"/"+subvol, "compression", algorithm)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set compression on %s/%s: %w", mountpoint, subvol, err)
	}
	fmt.Printf("Set compression on %s/%s: %s\n", mountpoint, subvol, algorithm)
	return nil
}

func cmdBtrfsMkswapfile(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: btrfs-mkswapfile <path> <size>")
	}
	path := args[0]
	size := args[1]
	return runCommand("btrfs", "filesystem", "mkswapfile", "--size", size, path)
}

func cmdUdevSettle(args []string) error {
	return runCommand("udevadm", "settle")
}

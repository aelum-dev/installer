package installbase

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/progress"
	"aelum-installer/backend/shared"
)

// basePackages are the essential system packages installed during the base
// system step. Kept minimal so this step finishes quickly and the progress
// bar moves smoothly.
var BasePackages = []string{
	// Base system
	"base",
	"linux-lts",
	"linux-firmware",
	"mkinitcpio",
	"edk2-shell",
	"memtest86+-efi",

	// Utilities
	"util-linux",
	"gptfdisk",
	"parted",
	"dosfstools",
	"e2fsprogs",
	"btrfs-progs",
	"lvm2",
	"pv",

	// System
	"sudo",
	"networkmanager",
	"iwd",

	// VPN. Dux Shell carries a VPN toggle — the bar widget, the control-center
	// tile and the Settings → Network → VPN page are all one client of
	// NetworkManager: the shell (dux-gop) shows profiles, connects and answers
	// auth prompts for exactly the connection types whose plugins NM can
	// register, so an install with no plugin carries a VPN panel with nothing
	// to switch. Remote workers are the first users, and the tunnel has to be
	// there the way the firewall is. These three are the protocols the shell
	// integrates end to end, measured on the dux-shell 2.x tree: the toggle
	// appears only when the backend is NetworkManager (which every install
	// enables); the core execs `nmcli` and `openconnect`; the VPN page imports
	// `.ovpn` and WireGuard `.conf` files; the auth dialogs — OpenVPN username,
	// OpenConnect password, GlobalProtect/Fortinet SAML — are the shell's own,
	// which is why the *plugin* packages ship rather than their GTK
	// editor/auth-dialog parents: the plugin half is what NM activates.
	//   networkmanager-vpn-plugin-openvpn      OpenVPN (+ the openvpn daemon)
	//   networkmanager-vpn-plugin-openconnect  AnyConnect / GlobalProtect /
	//     Fortinet (+ the openconnect binary the shell execs for password auth)
	// WireGuard needs no plugin — it is built into NetworkManager and into the
	// kernel — but its tools do: `wg`/`wg-quick` are what a provider's setup
	// doc names, and `wg show` is what a remote worker reads when the tunnel
	// stays silent. IKEv2/L2TP stay a package install away (the shell lists
	// whatever plugin a transaction puts in place); so does `tailscaled`, the
	// one daemon the shell's Tailscale tab drives but no install starts.
	"networkmanager-vpn-plugin-openvpn",
	"networkmanager-vpn-plugin-openconnect",
	"wireguard-tools",
	"bluez",
	"power-profiles-daemon",
	"zram-generator",

	// The firewall. A desktop is carried onto networks nobody owns, and until now
	// an install answered everything on them: `iproute2` has always pulled
	// `iptables → nftables`, so the tools sat on every disk and nothing ever
	// loaded a policy. firewalld is the policy — inbound is refused unless a rule
	// accepts it, and which rules apply is a property of the network, so a café
	// and a home network are not the same machine. Its unit is enabled by
	// configure.EnableServices.
	//
	// firewall-config is the same product's window (Arch ships them at the same
	// version): this desktop asks a question in a dialog, not in a man page, and a
	// person who needs a port open should not have to learn firewall-cmd first. It
	// costs about 100 KiB on top — firewalld's python stack and its gtk3 are
	// already in the closure of the list above.
	"firewalld",
	"firewall-config",

	// Build tools. The package installer's AUR page builds what it installs on
	// this machine, and makepkg refuses to run without these; an install that
	// lacks them can only take repository packages. This list is what puts them
	// in the image's mirror — the ISO build reads it through
	// backend/scripts/pkg-list.go — so the live session needs no compiler.
	"base-devel",

	// Things every Arch console is expected to answer with. `git` is how a
	// PKGBUILD tree, a VCS AUR build and a dotfiles repo arrive: without it the
	// base-devel above can build what is handed to it but fetch nothing, and an
	// offline install cannot get it afterwards any more than it can get
	// base-devel. Manual pages: the `base` group ships no `man` and no pages,
	// so without these the installed system's only documentation is a browser.
	"git",
	"man-db",
	"man-pages",

	// Bootloader (installed onto the ESP by the configure step; efibootmgr is
	// pulled in as its dependency)
	"refind",

	// Editor
	"vim",
	"nano",
}

// BtrfsOnlyBasePackages extend the base set only when the install chose btrfs.
// snapper is the snapshot engine the configure step configures for the subvolume
// layout (see configure.ConfigureSnapper); an ext4 install has no subvolumes to
// snapshot, so it gets neither snapper nor its GUI.
var BtrfsOnlyBasePackages = []string{
	"snapper",
}

// AppendBtrfsOnly returns packages plus extra when the install chose btrfs, and
// packages unchanged otherwise. Exported for installdesktop, which gates
// btrfs-assistant the same way.
//
// A btrfs-only package needs no second listing. pkg-list.go prints the btrfs-only
// lists unconditionally and the ISO build bakes their union with
// live-iso/packages.x86_64 into the mirror embedded in the image, which is what
// the installer installs from — so a package no list here or there names is the
// only kind that cannot be installed.
func AppendBtrfsOnly(ctx *shared.InstallContext, packages, extra []string) []string {
	if ctx.FileSystem != shared.FileSystemBtrfs {
		return packages
	}
	out := make([]string, 0, len(packages)+len(extra))
	out = append(out, packages...)
	return append(out, extra...)
}

// Run installs the Arch Linux base system via pacstrap.
func Run(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}

	return InstallPackages(ctx, AppendBtrfsOnly(ctx, BasePackages, BtrfsOnlyBasePackages), "base system")
}

// InstallPackages runs pacstrap with the given package list.
// Streams stdout to parse real progress from pacstrap's output lines.
// Exported for use by installdesktop package.
func InstallPackages(ctx *shared.InstallContext, packages []string, description string) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}

	if len(packages) == 0 {
		return fmt.Errorf("no packages to install")
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Installing %d %s packages via pacstrap", len(packages), description)
	}

	args := []string{"-K", shared.MountRoot}
	args = append(args, packages...)
	cmd := exec.Command("pacstrap", args...)

	// pacstrap is a shell script that hands the work to arch-chroot and then to
	// pacman, and those grandchildren are what hold the two pipes below. Give it a
	// process group of its own so the cancellation paths can stop the whole tree at
	// once — see primitives.KillProcessGroup.
	cmd.SysProcAttr = primitives.OwnProcessGroup()

	// Stream stdout to parse real progress from pacstrap's lines.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Capture stderr for crash reporting when pacstrap fails.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	var stderrBuf strings.Builder
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		buf := bufio.NewReader(stderr)
		for {
			line, err := buf.ReadString('\n')
			if line != "" {
				stderrBuf.WriteString(line)
			}
			if err != nil {
				break
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start pacstrap: %w", err)
	}

	// Track progress across pacstrap's multiple phases.
	tracker := progress.NewPacstrapProgressTracker()

	// Scan stdout with a bufio.Reader instead of bufio.Scanner: pacstrap's
	// "Packages (N) pkg1 pkg2 ..." line grows with the package list, and Scanner
	// aborts on a token larger than its buffer. That would stop draining the
	// pipe, leaving pacstrap blocked on a full stdout pipe forever.
	scanDone := make(chan struct{})
	var scanErr error
	go func() {
		defer close(scanDone)
		reader := bufio.NewReaderSize(stdout, 64*1024)
		for {
			line, err := reader.ReadString('\n')
			if line != "" {
				line = strings.TrimSuffix(line, "\n")
				line = strings.TrimSuffix(line, "\r")

				// Process all line types: Packages count, downloading, verifying,
				// installing, and (N/M) hook progress.
				pct := tracker.ProcessLine(line)
				if ctx.ProgressCallback != nil {
					ctx.ProgressCallback(pct)
				}

				// Command output appends to the progress line via => prefix.
				if ctx.Logger != nil {
					ctx.Logger.Command(line)
				}
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					scanErr = err
				}
				return
			}
		}
	}()

	select {
	case <-scanDone:
		if scanErr != nil {
			// Stop the tree: with its stdout reader gone it could block on a full
			// pipe and never exit.
			_ = primitives.KillProcessGroup(cmd.Process.Pid)
		}
		// Wait for the stderr reader to finish before reading stderrBuf: the
		// strings.Builder it writes is not safe for concurrent access.
		<-stderrDone
		waitErr := cmd.Wait()
		if scanErr != nil {
			return fmt.Errorf("pacstrap stdout scan: %w", scanErr)
		}
		if waitErr != nil {
			errMsg := strings.TrimSpace(stderrBuf.String())
			if errMsg != "" {
				return fmt.Errorf("pacstrap failed:\n%s", errMsg)
			}
			return fmt.Errorf("pacstrap failed: %w", waitErr)
		}
		return nil
	case <-ctx.Cancel:
		// Kill the whole tree so the scanner goroutine unblocks and exits: the
		// pacman doing the work is a grandchild of this process, and it holds the
		// output pipe, so killing only the direct child would leave the step
		// waiting for the install nobody asked for any more.
		// Cleanup is handled by the Installer's defer, not here. The error is the
		// shared one so the step loop reports a cancelled install rather than a
		// pacstrap that went wrong.
		_ = primitives.KillProcessGroup(cmd.Process.Pid)
		<-scanDone
		<-stderrDone
		// Reap the killed child so it does not linger as a zombie.
		_ = cmd.Wait()
		return shared.ErrInterrupted
	}
}

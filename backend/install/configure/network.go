package configure

import (
	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// DNS: /etc/resolv.conf as a link into /run
// ============================================================================

const (
	// resolvConfPath is where every resolver on the system looks for nameservers.
	resolvConfPath = "/etc/resolv.conf"
	// networkManagerResolvConf is NetworkManager's own view of the resolver. It
	// writes this file in every boot, whatever its configuration says about
	// managing /etc/resolv.conf (NetworkManager.conf(5): "Regardless of this
	// setting, NetworkManager will always write its version of resolv.conf to its
	// runtime state directory as /run/NetworkManager/resolv.conf"), and /run is a
	// tmpfs — writable in a boot whose root is not.
	networkManagerResolvConf = "/run/NetworkManager/resolv.conf"
)

// ConfigureNetworkDNS replaces the installed system's /etc/resolv.conf with a
// symbolic link into /run.
//
// The file the base image ships is a regular file **inside the root**, and
// NetworkManager updates it the way any well-behaved program updates a file:
// create a temp file in the same directory, rename it over. That is the whole
// story for a system whose root can be read-only — a snapshot boot mounts a
// read-only btrfs subvolume as /, the temp file is EROFS, and the boot runs with
// whatever resolver that snapshot happened to capture:
//
//   - the factory snapshot ("after installation", the state a broken system is
//     restored to, and the one the boot menu is required to offer) was taken
//     before the installed system ever booted with a network, so its
//     resolv.conf is the empty placeholder Arch ships and **nothing resolves**;
//   - any other snapshot carries the nameserver of the network it was taken on,
//     which is wrong the moment the machine is on a different one.
//
// A link into /run makes the resolver belong to the boot instead of to the
// snapshot: the link is part of the root (so it is in every snapshot and every
// restored system), and what it points at is written fresh, every boot, on a
// filesystem that is never read-only. Measured on a real snapshot boot: the
// link survives, `/run/NetworkManager/resolv.conf` holds the current
// nameserver, and `getent hosts` resolves.
//
// The absolute target is deliberate rather than the relative one systemd uses for
// its own resolved link. NetworkManager on Arch is built with rc-manager
// defaulting to "symlink", and it treats a link to *exactly this path* as its
// own: it refreshes the link when DNS changes, which is what emits the inotify
// notification other programs watch. In a snapshot boot that refresh fails and
// NetworkManager logs one warning — the link it wanted is already the link that
// is there, so the failure is cosmetic where it used to be functional.
//
// Non-fatal by design: an install that could not create this link is an install
// with the old behaviour, not a broken system.
func ConfigureNetworkDNS(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}

	hostPath := shared.MountRoot + resolvConfPath

	if ctx.Logger != nil {
		ctx.Logger.Info("Linking %s to %s", resolvConfPath, networkManagerResolvConf)
	}

	// The base package ships a regular file here, and a symlink does not replace
	// a file in place: it needs the name to be free.
	if err := primitives.RemoveAll(hostPath); err != nil && ctx.Logger != nil {
		ctx.Logger.Error("Warning: could not remove %s (non-fatal): %v", resolvConfPath, err)
	}

	if err := primitives.Symlink(networkManagerResolvConf, hostPath); err != nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("Warning: could not link %s to %s (non-fatal): %v",
				resolvConfPath, networkManagerResolvConf, err)
		}
		return nil
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Resolver configuration now follows the running system, not the root it was installed into")
	}
	return nil
}

# Security Policy

## Supported Versions

Only the latest release is supported. Please upgrade to the most recent version before reporting issues.

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability, please report it responsibly.

**Do not open a public issue.** Instead, choose one of these channels:

1. **GitHub**: Report through [GitHub's private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability) on the affected repository.
2. **Email**: If GitHub is not an option, use the contact published on [aelum.dev](https://aelum.dev).

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Your contact information for follow-up

### What to Expect

- **Acknowledgment** within 48 hours
- **Investigation** timeline communicated within 1 week
- **Disclosure** coordinated with you — we won't publish details until you've had time to prepare

## Security Design

The installer uses a **CLI-spawn pattern** to limit root access:

- The GUI runs as a **normal user** and spawns `aelum-installer-cli` via `sudo`
- The CLI binary handles all privileged operations directly via `exec.Command` and `arch-chroot`
- No separate helper binary is needed — the CLI is the privileged component
- Passwords are read from stdin (never CLI flags) to avoid `/proc`, `ps`, and shell history exposure
- The CLI emits JSON progress events to stderr; the GUI reads these to update the UI

### Sudo Configuration

On the live ISO, the user must have passwordless sudo for the CLI binary. The exact sudoers rule depends on the ISO configuration.

On the installed system, `/etc/sudoers.d/wheel` is written for the new user:

```
%wheel ALL=(ALL:ALL) ALL
```

## ISO Build & Download Trust Model

The Aelum live ISO is built to run **fully offline**, and that design choice has
security implications worth stating explicitly.

### Build-time package trust

- The ISO is assembled inside a privileged `archlinux` container by the `docker/` build
  (`make local-build` / `make build`, which run `scripts/build-iso.sh`). Official packages
  are downloaded with `pacman -Syw` into an embedded offline mirror
  (`airootfs/var/cache/aelum/offline-mirror/`), and AUR packages are compiled from source
  via `makepkg`.
- During the offline build, `SigLevel` is set to `Never` in
  `live-iso/pacman-offline.conf`. This is intentional: the mirror is a fixed,
  snapshot bundle, and enforcing signature verification against the live Arch
  keyring would require network access — which the offline build must not have.
- AUR packages are cloned directly from `aur.archlinux.org` and built with
  `--skipinteg`, so their integrity is not cryptographically verified at build
  time. Trust is placed in the upstream package names and in the pinned
  installer/live-iso commit the build clones.

### Post-install network trust

Once installed, the system is a normal Arch machine. The installer writes a
per-region `mirrorlist` (see `installer/backend/shared/mirrorlist/`) so future
online updates resolve against official Arch mirrors with their normal
signature enforcement. The offline `SigLevel = Never` applies **only** to the
build-time mirror, never to the installed system.

### Day-one inbound filtering

The trust placed in a network is not the installer's to grant, and a desktop is
carried onto networks nobody owns. So every install ships an inbound policy from
the first boot:

- `firewalld` is in the base package set and `firewalld.service` is enabled as a
  **required** service (`install/configure/services.go`). A machine that was
  supposed to get a firewall behaves exactly like a machine that has one until
  somebody tries its ports, so a failed enable aborts the install and names the
  unit instead of logging a warning.
- The default zone is `public`: unsolicited inbound traffic is dropped unless a
  rule accepts it, and outbound is unrestricted. Because NetworkManager leaves
  connections unzoned, this zone answers for every network the machine joins until
  a person assigns one per connection — the mechanism that keeps a home network and
  a café network from being the same machine.
- `firewall-config` ships beside it ("Firewall" in the launcher). Opening a port is
  a click in a window, not a man page; `firewall-cmd` is there for anyone who
  prefers it.
- Nothing installed listens by default: `openssh` arrives as a dependency of the
  desktop set, and no step enables `sshd` or `sshd.socket`.
- The posture is verified, not assumed: `install/verify/verify.go` asks the target's
  own systemd whether the unit is enabled and reads the default zone out of
  `/etc/firewalld/firewalld.conf`, refusing `trusted` — the one zone that accepts
  everything. `docker/scripts/verify-installed.sh` proves the same against a real
  installed disk. The installer never *writes* that config file (it is in the
  package's backup list, so an edit there becomes a merge prompt on every firewalld
  upgrade and a pinned value would outlive a changed upstream default), which is
  precisely why the value is read rather than trusted.

Two things about that zone are worth knowing before they are reported as bugs:

- Outbound discovery still works, so the machine finds printers, receivers and phones
  on the LAN, but it no longer *answers* being discovered: `public` carries no `mdns`
  allowance. Anything that has to reach this machine — a phone's KDE Connect, a
  printer polling it — needs a rule, which is what the Firewall window is for.
- The zone is shipped **exactly as the package defines it**
  (`/usr/lib/firewalld/zones/public.xml`), and Arch's copy carries two allowances:
  `dhcpv6-client` (stateless DHCPv6 replies arrive inbound, so dropping it would
  quietly cost address information on some networks) and `ssh`. That second one is
  why a fresh install has no listener and, the moment someone enables `sshd` from a
  wiki, why it is reachable on every network it joins. Closing it is one permanent
  rule (`firewall-cmd --permanent --zone=public --remove-service=ssh`), and the
  packaged behaviour is kept deliberately: the firewall does what its own
  documentation and every distro guide say it does, and the QEMU harness reaches the
  *installed* test system over exactly that allowance.

### The live ISO's SSH

The live image enables `sshd.service` with root login and an empty password, because
the whole QEMU test harness (`docker/`) drives the guest through it. That is a
support channel, not a service: an image booted on a network nobody owns and left
running through an install would otherwise present an unauthenticated root login on
its only interface. So the unit is gated on the host's 9p share
(`live-iso/airootfs/etc/systemd/system/sshd.service.d/aelum-live-ssh.conf`) — only a
boot that carries that share, which is a harness boot and nothing else, starts
`sshd`. With no share, nothing listens on 22 at all.

### Verifying a released ISO

ISOs are published to Cloudflare R2 (too large for GitHub releases) and linked
from the website. Each release ships:

- `SHA-256` and `SHA-512` checksum files
- a detached ASCII-armored GPG signature (`.asc`)

`make release` in `docker/` signs these artifacts (`scripts/sign-iso.sh`) and uploads
them to R2. Users can verify a download with:

```sh
sha256sum -c aelum-*-x86_64.iso.sha256
gpg --verify aelum-*-x86_64.iso.asc aelum-*-x86_64.iso
```

The maintainer's public signing key is published via WKD at
`https://aelum.dev/.well-known/openpgpkey/` — fetch it from there rather than from a
hosted key file, and verify the fingerprint before trusting a signature.

## Known Security Considerations

- The CLI binary requires passwordless sudo for privileged operations — ensure the sudoers rule is restricted to the CLI binary path only
- Disk paths are validated with regex to prevent shell injection, but the CLI still runs commands as root — trust the installer binary
- The installation log (`/var/log/aelum-install-<timestamp>.log`, mirrored to `/shared` when present) may contain sensitive information — it is not encrypted
- Passwords are zeroed from memory (`[]byte`) after being copied to `InstallContext` to minimize exposure time

## Responsible Disclosure Timeline

| Step | Target |
|------|--------|
| Acknowledge receipt | 48 hours |
| Confirm vulnerability | 1 week |
| Fix development | 2–4 weeks (depends on severity) |
| Publish fix + advisory | Coordinated with reporter |

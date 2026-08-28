# Security Policy

## Supported Versions

Only the latest release is supported. Please upgrade to the most recent version before reporting issues.

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability, please report it responsibly.

**Do not open a public issue.** Instead, choose one of these channels:

1. **Email**: Send a description to the project maintainer's email (see [README](README.md))
2. **GitLab**: Report through [GitLab's vulnerability reporting](https://docs.gitlab.com/ee/user/application_security/advisories/#reporting-vulnerabilities), or email the maintainer directly.

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

- The GUI runs as root and spawns `aelum-installer-cli` via `sudo`
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

- The ISO is assembled by `docker/build.sh` inside a privileged `archlinux`
  container. Official packages are downloaded with `pacman -Syw` into an embedded
  offline mirror (`airootfs/var/cache/aelum/offline-mirror/`), and AUR packages
  are compiled from source via `makepkg`.
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

### Verifying a released ISO

ISOs are published to Cloudflare R2 (too large for GitLab releases) and linked
from the website. Each release ships:

- `SHA-256` and `SHA-512` checksum files
- a detached ASCII-armored GPG signature (`.asc`)

Use `docker/make-release.sh` to generate these artifacts locally before upload.
Users can verify a download with:

```sh
sha256sum -c aelum-*-x86_64.iso.sha256
gpg --verify aelum-*-x86_64.iso.asc aelum-*-x86_64.iso
```

The maintainer's public signing key is published alongside releases; pin and
verify it before trusting a signature.

## Known Security Considerations

- The CLI binary requires passwordless sudo for privileged operations — ensure the sudoers rule is restricted to the CLI binary path only
- Disk paths are validated with regex to prevent shell injection, but the CLI still runs commands as root — trust the installer binary
- The installation log (`~/.local/share/aelum-installer/installation.log`) may contain sensitive information — it is not encrypted
- Passwords are zeroed from memory (`[]byte`) after being copied to `InstallContext` to minimize exposure time

## Responsible Disclosure Timeline

| Step | Target |
|------|--------|
| Acknowledge receipt | 48 hours |
| Confirm vulnerability | 1 week |
| Fix development | 2–4 weeks (depends on severity) |
| Publish fix + advisory | Coordinated with reporter |

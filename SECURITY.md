# Security Policy

## Supported Versions

Only the latest release is supported. Please upgrade to the most recent version before reporting issues.

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability, please report it responsibly.

**Do not open a public issue.** Instead, choose one of these channels:

1. **Email**: Send a description to the project maintainer's email (see [README](README.md))
2. **GitHub Security Advisories**: Use the [Report a vulnerability](https://github.com/AestheticOS/AestheticOS-Installer/security/advisories/new) button

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

The installer uses a **privileged helper binary** pattern to limit root access:

- The main GUI/CLI binaries run as the user — no root required
- Only `aestheticos-helper` runs via `sudo`, and only with explicitly whitelisted commands
- The helper uses two whitelists:
  - `runWhitelist` — host-level commands (blkid, sgdisk, wipefs, etc.)
  - `chrootRunWhitelist` — commands allowed inside `arch-chroot /mnt`
- All privileged operations go through the helper; no direct root execution elsewhere

## Known Security Considerations

- The helper binary requires passwordless sudo for specific commands — ensure the sudoers rule is restricted to the helper path only
- Disk paths are validated with regex to prevent shell injection, but the helper still runs commands as root — trust the installer binary
- The installation log (`~/.local/share/aestheticos-installer/installation.log`) may contain sensitive information — it is not encrypted

## Responsible Disclosure Timeline

| Step | Target |
|------|--------|
| Acknowledge receipt | 48 hours |
| Confirm vulnerability | 1 week |
| Fix development | 2–4 weeks (depends on severity) |
| Publish fix + advisory | Coordinated with reporter |

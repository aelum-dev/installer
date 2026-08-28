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

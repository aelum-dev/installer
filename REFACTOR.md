# Helper Refactor Plan

## Problem

The `aestheticos-helper` binary has a generic `run` command that executes **any** root command via sudo. This is a security risk and makes the sudoers configuration harder to reason about. Every `bash -c`, `cat`, `mkdir`, `cp`, `findmnt`, `blkid` call goes through this backdoor.

### Current `run` Usage

| Category | Examples | Call Count |
|----------|----------|------------|
| Chroot ops | `arch-chroot /mnt systemctl enable ...`, `arch-chroot /mnt locale-gen`, `arch-chroot /mnt dconf update`, `arch-chroot /mnt bash -c "useradd ..."` | ~15 |
| File ops | `cat`, `cp`, `mkdir -p`, `touch`, `chown`, `chmod`, `sed -i`, `echo >`, `ln -sf` | ~15 |
| System queries | `findmnt`, `blkid`, `ls`, `test -f`, `find` | ~8 |
| Swap ops | `fallocate`, `dd`, `mkswap`, `swapon` | ~4 |
| Other | `sgdisk`, `rmdir`, `bash -c` scripts | ~5 |

### Architecture Constraint

The GUI installer runs as a **normal user** — it explicitly refuses to run as root. The helper is the **only** root path. All privileged operations must go through it. Chroot operations cannot be moved to the main binary.

---

## Proposed Solution

Replace the generic `run` command with dedicated, purpose-built commands. Group them into four categories.

### 1. Chroot Commands (whitelisted, specific)

These run commands inside `arch-chroot /mnt` with a strict whitelist of allowed commands.

| Command | Args | Replaces |
|---------|------|----------|
| `chroot-run <cmd> [args...]` | Whitelisted cmd only | `run arch-chroot /mnt <cmd> [args...]` |
| `enable-service <service>` | service name | `run arch-chroot /mnt systemctl enable <service>` |
| `run-locale-gen` | none | `run arch-chroot /mnt locale-gen` |
| `run-dconf-update` | none | `run arch-chroot /mnt dconf update` |
| `run-useradd <username>` | username | `run arch-chroot /mnt useradd -m -G wheel <username>` |
| `run-chpasswd <username>` | username (password via stdin) | `run arch-chroot /mnt chpasswd` |
| `run-plymouth-set-theme <theme>` | theme name | `run arch-chroot /mnt plymouth-set-default-theme -R <theme>` |
| `run-mkinitcpio` | none | `run arch-chroot /mnt mkinitcpio -P` |

**Whitelist for `chroot-run`:** `systemctl`, `locale-gen`, `dconf`, `useradd`, `usermod`, `chpasswd`, `plymouth-set-default-theme`, `mkinitcpio`, `test`, `id`, `mkdir`, `rm`, `ln`, `chown`, `chgrp`, `chmod`, `setfacl`, `cp`

### 2. Target Filesystem Ops (writes to /mnt)

These operate on paths inside the installed system (`/mnt/...`).

| Command | Args | Replaces |
|---------|------|----------|
| `write-file <path> <content>` | absolute path + content via stdin | `bash -c "echo ... > <path>"` |
| `append-file <path> <content>` | absolute path + content via stdin | `bash -c "echo ... >> <path>"` |
| `read-file <path>` | absolute path | `cat <path>` |
| `set-symlink <target> <link>` | target + link path | `bash -c "ln -sf ..."` |
| `chmod <mode> <path>` | mode + path | `chmod <mode> <path>` |
| `chown <owner> <path>` | owner + path | `chown <owner> <path>` |
| `touch <path>` | path | `touch <path>` |
| `mkdir <path>` | path | `mkdir -p <path>` |

### 3. Host-Level Queries

These query the live ISO host system.

| Command | Args | Replaces |
|---------|------|----------|
| `query-blkid <device>` | device path | `run blkid -s UUID -o value <device>` |
| `query-findmnt <mountpoint>` | mountpoint | `run findmnt -n -o SOURCE <mountpoint>` |
| `query-ls <path>` | path | `run ls <path>` |
| `query-test <flags> <path>` | flags + path | `run test <flags> <path>` |

### 4. Keep Existing Commands

No changes needed:
- `parted`, `format`, `mount`, `umount`
- `pacstrap`, `genfstab`
- `bootctl-install`
- `passwd`, `useradd`
- `efibootmgr`
- `btrfs-subvol-create`, `btrfs-subvol-set-default`, `btrfs-set-compression`, `btrfs-mkswapfile`
- `udev-settle`, `mkswap`, `swapon`, `chattr`
- `reboot`
- `mountpoints`, `lsblk-partitions`

### Remove

- `run` — the generic backdoor

---

## Migration Path

### Phase 1: Add new commands (backward compatible)
1. Add all new commands to `helper/main.go`
2. Add `HelperCmd` constants to `install/config.go`
3. Do NOT remove `run` yet

### Phase 2: Migrate call sites
1. Replace `run` usages in `checks.go` → `query-findmnt`
2. Replace `run` usages in `format.go` → `query-blkid`
3. Replace `run` usages in `fstab.go` → `read-file`
4. Replace `run` usages in `copy.go` → `mkdir` + `copy-file`
5. Replace `run` usages in `system.go` → `write-file`, `append-file`, `set-symlink`, `run-locale-gen`
6. Replace `run` usages in `users.go` → `run-useradd`, `run-chpasswd`, `run-dconf-update`, `write-file`, `append-file`
7. Replace `run` usages in `greetd.go` → `chroot-run`, `run-usermod`, `mkdir`, `chown`, `chmod`, `set-symlink`, `copy-file`
8. Replace `run` usages in `plymouth.go` → `run-plymouth-set-theme`, `touch`, `chown`, `read-file`
9. Replace `run` usages in `bootloader.go` → `query-ls`, `query-test`, `read-file`, `query-findmnt`
10. Replace `run` usages in `swap.go` → dedicated swap commands (already mostly there)

### Phase 3: Remove `run`
1. Remove `run` command from `helper/main.go`
2. Remove `HelperCmdRun` constant
3. Update sudoers if needed
4. Run full test suite

---

## Sudoers Impact

Current sudoers likely grants access to `helper run *`. After refactor:
```
%wheel ALL=(root) NOPASSWD: /usr/local/bin/aestheticos-helper *
```
This becomes safe because the helper only does specific, bounded operations — no arbitrary command execution.

---

## Files to Change

| File | Change |
|------|--------|
| `backend/cmd/helper/main.go` | Add new commands, remove `run` |
| `backend/internal/install/config.go` | Add `HelperCmd` constants |
| `backend/internal/install/checks.go` | Use `query-findmnt` |
| `backend/internal/install/format.go` | Use `query-blkid` |
| `backend/internal/install/fstab.go` | Use `read-file` |
| `backend/internal/install/copy.go` | Use `mkdir` + `copy-file` |
| `backend/internal/install/system.go` | Use `write-file`, `append-file`, `set-symlink`, `run-locale-gen` |
| `backend/internal/install/users.go` | Use `run-useradd`, `run-chpasswd`, `run-dconf-update` |
| `backend/internal/install/greetd.go` | Use `chroot-run`, `run-usermod`, etc. |
| `backend/internal/install/plymouth.go` | Use `run-plymouth-set-theme`, `touch`, `chown` |
| `backend/internal/install/bootloader.go` | Use `query-ls`, `query-test`, `read-file` |
| `backend/internal/install/command.go` | Update `runChroot` to use `chroot-run` |

---

## Testing Strategy

1. Unit tests for each new helper command
2. Integration tests for migrated call sites
3. Dry-run installation test to verify end-to-end
4. Sudoers validation test

---

## Out of Scope

- JSON/RPC protocol overhaul (future)
- Moving chroot ops to main binary (not possible — GUI runs as user)
- Helper binary refactoring beyond command additions/removals

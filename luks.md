# LUKS Disk Encryption — Implementation Plan

Graphical Arch installer gains optional full-root encryption via LUKS2. Only the
root partition is encrypted (the EFI partition stays plaintext — required for
UEFI boot). Everything else in the pipeline (partitioning, pacstrap, users,
desktop, greetd) is unchanged; the decrypted mapper device is what gets mounted,
populated, and referenced by fstab/bootloader.

Crypto parameters are pinned to current (2026) best practices — see
§11 for the verified rationale.

---

## 1. UX / State Model (the toggle + dialog flow)

The requirement: a LUKS toggle sits next to the FS pills on the Disk Selection
screen. Toggling it does **not** flip on immediately — it opens a popup to enter
and confirm a passphrase. The toggle only becomes "enabled" once a valid
passphrase is stored in memory. Disabling erases the passphrase; re-enabling
re-opens the popup.

### Shared store state (`frontend/src/lib/store.svelte.ts`, added to `installState`)
```ts
luksEnabled:   false,   // encryption committed/active — drives the toggle's on/off look
luksPassphrase: '',     // passphrase held in memory ('' = none held)
luksDialogOpen: false,  // passphrase entry popup visible
luksError:      '',     // inline validation error inside the popup
```

### Store actions (added to `store.svelte.ts`)
- `requestEnableLuks()` — sets `luksDialogOpen = true`. Does **not** set `luksEnabled`.
- `commitLuksPassphrase(pass, confirm)` — validates (`pass !== ''`, `pass === confirm`);
  on success sets `luksPassphrase = pass`, `luksEnabled = true`, `luksDialogOpen = false`,
  `luksError = ''`; on failure keeps the popup open and sets `luksError`.
- `disableLuks()` — clears `luksPassphrase = ''`, `luksEnabled = false`. Guarded by a
  confirm dialog (see §4) because it discards encryption.

### Validation rule
Passphrase must be non-empty and match the confirm field. (No hard minimum length —
the passphrase is the user's choice; a hint about length is shown but not enforced.)

### Data flow (frontend → install)
```
DiskTopBar toggle ──click(off)──▶ LuksPassphraseDialog ──valid──▶ store.commitLuksPassphrase
DiskTopBar toggle ──click(on)──▶ ConfirmDialog(disable) ──yes──▶ store.disableLuks
…
ConfirmationPage reads installState.luksEnabled  → shows "Encrypted (LUKS2)"
…
InstallationPage / GUI reads installState.luksEnabled + installState.luksPassphrase
  ─Wails binding▶ App.SetLuks(bool) / App.SetLuksPassphrase(string)
  ─spawn CLI▶ aelum-installer-cli --luks …  (passphrase written to stdin after user pw)
```

The passphrase lives only in memory from entry until the CLI consumes it, then is
zeroed — mirroring the existing account-password handling. It never hits a flag,
a log, or disk (except as the LUKS key slot).

---

## 2. Frontend changes

### 2.1 `frontend/src/components/LuksPassphraseDialog.svelte` (new)
Modal matching `ConfirmDialog` styling (`.dialog-overlay` / `.dialog`). Fields:
`Passphrase` and `Confirm passphrase`, each `type="password"`, show/hide toggle.
Live inline validation: error state when fields diverge or empty. Submit calls
`store.commitLuksPassphrase`. Cancel/Escape/backdrop sets `luksDialogOpen = false`.
Localized via `t('…')`.

### 2.2 `frontend/src/components/DiskTopBar.svelte`
Add a compact **Encrypt (LUKS2)** toggle switch to the right of the FS pills (the
`fs-bar`), per the "next to fs pills" requirement. Props added:
`luksEnabled`, `onEnableLuks`, `onDisableLuks` (all with safe defaults so other
callers are unaffected).
- Off switch → `onEnableLuks()` (opens the popup).
- On switch → `onDisableLuks()` (opens the disable confirm).
The switch reflects `luksEnabled` only — it does not animate to on until the
passphrase is committed (matches "becoming enabled only if … in memory now").

### 2.3 `frontend/src/pages/DiskSelectionPage.svelte`
Render `<LuksPassphraseDialog bind:show={installState.luksDialogOpen} />` (shown in
both automatic and manual mode, since LUKS is supported in both). Pass
`luksEnabled={installState.luksEnabled}` plus the two callbacks to `<DiskTopBar />`.

### 2.4 `frontend/src/pages/ConfirmationPage.svelte`
When `installState.luksEnabled`, add a tag/badge in the Disk summary:
"Encrypted with LUKS2" (`t('page.confirmation.luksEncrypted')`).

### 2.5 `frontend/src/lib/store.svelte.ts`
Add the state + actions from §1. No change to `isPageValid` — the dialog itself
guarantees a valid passphrase before `luksEnabled` is set, so disk-selection
validity (diskId + fileSystem) is unaffected.

### 2.6 Translations
Add keys to all 28 locale files (fall back to English for any missing):
`disk.encryptLabel`, `disk.encryptHint`,
`luks.dialog.title`, `luks.dialog.passphrase`, `luks.dialog.confirmPassphrase`,
`luks.dialog.confirmPlaceholder`, `luks.dialog.errorMatch`, `luks.dialog.errorEmpty`,
`luks.disable.title`, `luks.disable.message`, `page.confirmation.luksEncrypted`.

### 2.7 Wails bindings (REQUIRED — see §10)
Regenerate `frontend/wailsjs/go/gui/*` after adding the Go methods in §3.2.

---

## 3. Backend changes

### 3.1 `backend/install/installer.go`
- `NewInstaller` signature gains two params **before** `dryRun`:
  `NewInstaller(diskID, username, password, hostname string, region shared.Region,
  fileSystem string, luks bool, luksPassphrase string, dryRun bool)`.
- Store `luks` / copy `luksPassphrase` into the `InstallContext` (then zero the
  installer's copy, as is done for the account password).
- `RunCleanup()` closes the LUKS mapper (`luks.LuksClose(shared.LuksMapperName)`)
  when `ctx.Luks` is set, so a failed install can be retried (no leaked dm device).

### 3.2 `backend/gui/app.go` / `navigation.go`
Add fields to the `App` struct: `luks bool`, `luksPassphrase []byte`.
Add Wails-bindable methods:
- `SetLuks(enabled bool)` — sets `a.luks`.
- `SetLuksPassphrase(passphrase string)` — copies into `a.luksPassphrase` (zeroed
  buffer; never logged).

### 3.3 `backend/gui/installation.go`
- In `StartInstallation`, append `--luks` to the CLI args when `a.luks`.
- When writing stdin: after the account password line, if `a.luks && !a.dryRun`,
  write the LUKS passphrase line (`passphrase + "\n"`) **before** closing stdin.
  (Order matches the CLI's sequential reads: username flag → account pw → LUKS pw.)

### 3.4 `backend/cli/main.go`
- New flag `--luks` (bool).
- After reading the account password, if `*luksFlag && !dryRun && stepName != "verify_system"`,
  read a second passphrase via the existing masked `readPassword` path (TTY masked,
  stdin if piped). Store as `luksPassphrase`.
- Pass `*luksFlag, luksPassphrase` into `NewInstaller`.
- `handleRollback` unchanged (ext4, no encryption).
- Update `printUsage()` to document `--luks`.

### 3.5 `backend/shared/config.go`
- `LuksMapperName = "aelum"` (the `/dev/mapper/<name>` target).
- Optional explicitness constants (defaults already correct — see §11):
  `LuksCipher = "aes-xts-plain64"`, `LuksHash = "sha256"`, `LuksKeySizeBits = 512`
  (AES-256 under XTS), `LuksPBKDF = "argon2id"`, `LuksIterTimeMs = 2000`.

### 3.6 `backend/shared/types.go`
- `InstallContext`: add `Luks bool`, `LuksPassphrase []byte` (zeroed after use).
- `PartitionInfo`: add `RootMapper string` (e.g. `/dev/mapper/aelum`) and
  `LuksUUID string` (the LUKS **header** UUID, used for the boot cmdline — NOT the
  filesystem UUID). `RootUUID` becomes the **mapper** filesystem UUID for fstab.

### 3.7 `backend/install/luks/luks.go` (new package)
- `IsLuks(path) bool` — `cryptsetup isLuks path`.
- `LuksFormat(logger, path, passphrase)` — pipes passphrase via stdin:
  `cryptsetup --batch-mode --type luks2 --key-size 512 --key-file=- luksFormat path`.
- `LuksOpen(path, mapperName, passphrase)` — `cryptsetup open --type luks --key-file=- path mapperName`.
- `LuksClose(mapperName)` — `cryptsetup close mapperName`.
- `LuksUUID(path) (string, error)` — reads `cryptsetup luksUUID path`.
All commands go through `primitives.Run`; passphrase is fed via a stdin pipe (never
a flag). A failed `LuksOpen` right after `LuksFormat` validates the passphrase
(equivalent to `--verify-passphrase`) before any filesystem is written.

### 3.8 `backend/install/format/format.go` + `BuildPartitionInfo`
When `ctx.Luks`, after creating/formatting the raw root partition:
1. `luks.LuksFormat(logger, rootPart, passphrase)`.
2. `luks.LuksOpen(rootPart, mapperName, passphrase)` → mapper path.
3. `mkfs` (ext4/btrfs) on the **mapper** device, not the raw partition.
4. btrfs subvolumes created on the **mapper** (as today, but on `/dev/mapper/aelum`).
5. Close the mapper after formatting (keep it closed; `mount` reopens it) — or keep
   open and rely on cleanup to close. UUID reads target the mapper for `RootUUID`.
Set `ctx.Partitions.RootMapper` and `RootUUID` = mapper filesystem UUID.

### 3.9 `backend/install/partition/partition.go` (`Apply`, manual mode)
Same LUKS sequence on the designated **System** row: `luksFormat` → `luksOpen` →
mkfs on mapper → btrfs subvolumes on mapper. EFI/Preserve rows untouched.
Populate `RootMapper`/`LuksUUID` via `BuildPartitionInfo`.

### 3.10 `backend/install/mount/mount.go`
When `ctx.Luks`: `luks.LuksOpen(rootPart, mapperName, passphrase)` (idempotent —
skip if already open), then mount the **mapper** path to `/mnt` (and subvolumes from
the mapper) instead of `p.RootPart`. `verifyMount` compares against the mapper path.

### 3.11 `backend/install/configure/configure.go`
- **Boot entry** (`writeBootEntry`): replace `root=UUID=<rootUUID>` with
  `rd.luks.name=<LuksUUID>=aelum root=/dev/mapper/aelum` (+ keep `rootflags=subvol=@`
  for btrfs). The LUKS header UUID comes from `ctx.Partitions.LuksUUID`.
- **fstab** (`generateExt4Fstab` / `generateBtrfsFstab`): use the **mapper** UUID
  (`RootUUID`) as the root source.
- **initramfs hook**: ensure `sd-encrypt` is present in `/mnt/etc/mkinitcpio.conf`
  HOOKS, placed after `block` and before `filesystems`. Detect the initramfs base:
  if the HOOKS line contains `systemd`, add `sd-encrypt` (and `sd-vconsole` already
  precedes it, so the keymap applies at the prompt); if it contains `udev` (busybox),
  add `encrypt` and ensure a keymap hook precedes it. Then run an explicit
  `arch-chroot /mnt mkinitcpio -P`. This runs in `ConfigureSystemBase`, which
  precedes `ConfigurePlymouth`, so the plymouth regen preserves the hook.

### 3.12 `backend/install/verify/verify.go`
When `ctx.Luks`: verify `root=`/`rd.luks.name` reference the mapper, the mapper UUID
is in fstab, and `sd-encrypt`/`encrypt` is present in `/mnt/etc/mkinitcpio.conf`.
Read-only.

---

## 4. Disable-encryption confirm
Disabling is destructive (discards the passphrase → install proceeds unencrypted),
so clicking the on-switch opens a `ConfirmDialog` (reuse the existing component with
custom `title`/`message`/labels localized under `luks.disable.*`) before calling
`store.disableLuks`. Re-enabling simply re-opens `LuksPassphraseDialog` (the normal
off→click flow).

---

## 5. live-iso change
`live-iso/packages.x86_64`: add `cryptsetup`. It is required in the live environment
to run `luksFormat`/`open`, and adding it pulls it into the offline mirror so the
installed system gets the `sd-encrypt` hook's binary.

---

## 6. Tests

### Go
- `backend/install/luks/luks_test.go` — unit tests for format/open/close/UUID using a
  mocked `primitives.Run` (assert exact args: `--batch-mode`, `--type luks2`,
  `--key-size 512`, `--key-file=-`, `luksFormat`/`open`/`close`).
- `format` + LUKS and `mount` + LUKS integration tests (with `AELUM_DRY_RUN` / fake
  disks where real cryptsetup is unavailable).
- `partition.Apply` + LUKS for the manual path.
- Update `backend/install/installer_integration_test.go` (9+ `NewInstaller(...)` calls)
  and any CLI tests for the new signature.
- `go test ./...` + `golangci-lint run ./... --fix` before building.

### Frontend
- `store.svelte.ts` tests: `commitLuksPassphrase` (match/no-match/empty) and
  `disableLuks` (clears passphrase + flag).
- `LuksPassphraseDialog.svelte` component test (validation + submit path).
- `cd frontend && npm run check` + `npm run test`.

---

## 7. Build & verify order
1. `go test ./...` (expect integration-test signature updates to pass).
2. `golangci-lint run ./... --fix`.
3. Regenerate Wails bindings: `wails generate module gui` (or `wails build`).
4. `go run ./backend/scripts/gen-ts-constants.go` (if new constants added).
5. `cd frontend && npm run check && npm run test`.
6. `./build.sh` → produces `build/bin/aelum-installer` + `...-cli`.
7. **VM-only end-to-end** (never on host): `docker/build` then `docker/run.sh`, boot,
   enable LUKS, install to a disk, and confirm the **boot-time passphrase prompt**
   appears and the desktop launches. The initramfs hook correctness is the
   make-or-break check — verify first.

---

## 8. Security notes
- Passphrase: stdin-only, never a flag, never logged; zeroed in the Go struct after
  copying to `InstallContext`; erased on disable. Matches the account-password policy.
- Only the root partition is encrypted; EFI stays plaintext (mandatory for UEFI).
- Swap stays on zram → **no encrypted swap, no `resume=` device needed**.
- Recommend (optional, out of v1 scope): ship a LUKS header backup
  (`cryptsetup luksHeaderBackup`) and note the "forget passphrase = unrecoverable"
  risk in the confirmation copy.

---

## 9. Files changed (summary)
| Area | Files |
|------|-------|
| Frontend | `store.svelte.ts`, `DiskTopBar.svelte`, `DiskSelectionPage.svelte`, `ConfirmationPage.svelte`, new `LuksPassphraseDialog.svelte`, 28 translation files, regenerated `constants.ts` + Wails bindings |
| Backend core | `installer/installer.go`, `shared/config.go`, `shared/types.go`, new `install/luks/luks.go` |
| Install steps | `format/format.go`, `partition/partition.go`, `mount/mount.go`, `configure/configure.go`, `verify/verify.go` |
| CLI / GUI | `cli/main.go`, `gui/app.go`, `gui/navigation.go`, `gui/installation.go` |
| Tests | `install/luks/luks_test.go`, `format`/`mount`/`partition` LUKS tests, `installer_integration_test.go`, frontend store/dialog tests |
| ISO | `live-iso/packages.x86_64` (add `cryptsetup`) |

---

## 10. Binding regeneration (important)
`frontend/wailsjs/go/gui/App.d.ts` / `App.js` / `models.ts` are checked in and
consumed by the frontend (and its tests). Adding `SetLuks` / `SetLuksPassphrase`
to the Go `App` struct **requires** regenerating them (`wails generate module gui`,
or a full `wails build`). Without this, `DiskTopBar`/`LuksPassphraseDialog` imports
of the new bindings fail to type-check and the feature is dead. Hand-editing these
files is acceptable only as a fallback; prefer the generator.

---

## 11. Verified crypto parameters (2026 best practices)
Sourced from ArchWiki *dm-crypt/Device_encryption* + *dm-crypt/System_configuration*,
the current `mkinitcpio.conf` defaults, and cryptsetup's documented 2.4.0+ defaults.

- **LUKS2** — current version, default since cryptsetup 2.1.0. ✓
- **`argon2id`** KDF — the default and recommended choice (memory-hard; resists
  GPU/ASIC brute force). `pbkdf2` is strictly weaker and was **dropped** from the
  earlier draft — the modern systemd initramfs supports argon2id with no issue. ✓
- **`aes-xts-plain64`** cipher — default. ✓
- **`--key-size 512`** — XTS splits the key into two AES keys, so 512 = AES-256
  (256 would be AES-128). Explicitly pinned to guarantee AES-256. ✓
- **`sha256`** hash for KDF — default and secure; sha512's only downside (GRUB
  slowness) doesn't apply since we use systemd-boot. ✓
- **`--iter-time 2000`** ms — the modern minimum default for the volume-key KDF. ✓
- **initramfs hook** — `sd-encrypt` (systemd-based default), placed after `block`,
  before `filesystems`. `sd-vconsole` already precedes it, so the user's keymap
  (`/etc/vconsole.conf`) is loaded before the passphrase prompt. Regenerate with
  `mkinitcpio -P`. ✓
- **boot cmdline** — `rd.luks.name=<LUKS-header-UUID>=aelum root=/dev/mapper/aelum`
  (systemd-cryptsetup-generator syntax; the legacy `cryptdevice=` form is for the
  busybox `encrypt` hook only). ✓

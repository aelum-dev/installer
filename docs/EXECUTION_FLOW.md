# Execution Flow

How every path flows through the codebase. Function names over line numbers —
line numbers go stale the moment anything above them moves.

---

## Table of Contents

- [Startup](#startup)
- [Installation Flow (GUI → CLI)](#installation-flow-gui--cli)
- [Signal Handling](#signal-handling)
- [Cleanup Paths](#cleanup-paths)
- [Error Paths](#error-paths)
- [Goroutine Map](#goroutine-map)
- [Key Data Structures](#key-data-structures)

---

## Startup

```
main() [main.go]
  │
  ├─> dryRun = env AELUM_DRY_RUN == "1"
  │
  ├─> gui.NewApp(dryRun)           # Create app struct
  │
  ├─> SetStartPage from AELUM_START_PAGE env (debug)
  │
  ├─> signal.Notify(SIGINT, SIGTERM, SIGHUP)  # BEFORE wails.Run
  │   └─> goroutine: <-sigChan → CancelInstallation() → WaitForCleanup() (10s timeout) → os.Exit(0)
  │
  └─> wails.Run(&options.App{       # Block here until app exits
      ├─> OnStartup:   app.StartUp(ctx)   # CheckRoot → dry-run fallback, GetUILocale
      ├─> OnBeforeClose: app.OnBeforeClose(ctx)
      └─> Bind: app                 # Exposes methods to frontend
      })
```

**Key point:** the signal handler starts BEFORE `wails.Run()`, so it is active from program start.

---

## Installation Flow (GUI → CLI)

### 1. User clicks "Install" on ConfirmationPage

```
Frontend: ConfirmationPage.svelte
  └─> navigate to InstallationPage
      └─> wailsjs/go/gui/App.StartInstallation()
          │
          └─> gui/installation.go StartInstallation():
              ├─> CheckRoot()                    # Verify sudo access
              ├─> findCLIBinary()                # Locate aelum-installer-cli
              ├─> context.WithCancel()
              ├─> exec.CommandContext(ctx, "sudo", cli, --install <disk>, …)
              ├─> cmd.Start()
              ├─> state: installCmd / installCancel / installDone / installationRunning
              └─> Start 4 goroutines:
                  ├─> [G1] Write password to stdin (then the LUKS passphrase when encrypting)
                  ├─> [G2] Forward stdout → os.Stderr
                  ├─> [G3] Read stderr → emit JSON progress events to frontend
                  └─> [G4] cmd.Wait() → close(installDone) → clear state
```

### 2. CLI Process Starts

```
backend/cli/main.go
  ├─> flag.Parse()  (--install / per-step flags / --rollback / --cleanup, see printUsage)
  ├─> Check root: os.Getuid() != 0 → exit(1)     # skipped in dry-run
  ├─> Resolve disk + step name
  ├─> readUsername() / readPassword()             # From stdin (masked); LUKS passphrase after it
  ├─> NewInstaller(...)
  ├─> Wire logger + callbacks
  ├─> defer panic recovery → RunCleanup()
  ├─> signal.Notify(SIGINT)                        # CLI signal handler
  │   └─> goroutine: <-sigChan → Cancel() → SetInterrupted()
  └─> installer.Install()  OR  installer.RunStep(stepName)
```

### 3. Install() Execution

```
install/installer.go Install()
  ├─> defer RunCleanup()              # Runs on ANY exit path (sync.Once)
  ├─> context()                       # InstallContext with cancel channel
  └─> for each step in Steps():       # install/step.go owns the order + weights
      ├─> step.Run(ctx)
      │   ├─> format_disk        → format.Run / partition.Apply (manual mode)
      │   ├─> mount_filesystem   → mount.Run
      │   ├─> install_base       → installbase.Run      (pacstrap, streaming progress)
      │   ├─> install_desktop    → installdesktop.Run   (pacstrap, streaming progress)
      │   ├─> setup_users        → users.Run            (isocopy + weather + useradd)
      │   ├─> configure_system   → configure.Run        (fstab → … → snapper last; see OVERVIEW)
      │   ├─> clear_cache        → clearcache.Run
      │   └─> verify_system      → verify.Run           (read-only checks)
      └─> if err != nil → return      # defer RunCleanup fires
```

---

## Signal Handling

### GUI Signal Handler (main.go)

**Trigger:** Ctrl+C in the terminal (SIGINT), `kill`/WM close (SIGTERM), terminal hangup (SIGHUP).

```
<-sigChan
  ├─> app.CancelInstallation()
  │     └─> a.installCmd.Process.Signal(os.Interrupt)   # SIGINT to the CLI
  │         (NOT the CommandContext cancel — that would SIGKILL before cleanup)
  ├─> WaitForCleanup()      # Block on <-installDone (closed when cmd.Wait() returns)
  │     └─> with a 10s timeout: on expiry, primitives.CleanupMounts() force-unmounts /mnt
  └─> os.Exit(0)
```

### CLI Signal Handler (cli/main.go)

**Trigger:** SIGINT (from the GUI's CancelInstallation or the user's Ctrl+C).

```
<-sigChan (SIGINT)
  ├─> installer.Cancel()          # close(cancel channel)
  │     └─> installPackages() sees <-ctx.Cancel → kills pacstrap → returns "interrupted"
  ├─> installer.SetInterrupted()  # atomic.Bool
  └─> Install() returns error → main checks Interrupted() → os.Exit(130)
```

---

## Cleanup Paths

Cleanup runs through `Installer.RunCleanup()`, guarded by `sync.Once` — only the first
caller does work:

1. **Normal install failure** — `Install()`'s `defer RunCleanup()` fires.
2. **SIGINT during installation** — the signal cancels the running step, `Install()`
   returns an error, and the same defer fires.
3. **Window close** — `OnBeforeClose` cancels + waits, then runs a fallback
   `sudo aelum-installer-cli --cleanup` (unmounts `/mnt` even if no CLI is running).
4. **Panic recovery** — the CLI's `defer recover()` calls `RunCleanup()`.

`RunCleanup()` ends in `primitives.CleanupMounts()`: find PIDs holding mounts
(`fuser -m -M /mnt`), kill them, `umount -R /mnt`.

---

## Error Paths

| Error | Where | Outcome |
|-------|-------|---------|
| Sudo access required | `StartInstallation` → `CheckRoot()` | Error to the frontend banner; no install starts |
| Installation already running | `StartInstallation` (`installationRunning`) | Second call refused |
| CLI spawn failure | `cmd.Start()` error | `cancel()`, error returned |
| Step failure | `step.Run()` error, not an interrupt | `emitError(step.Label(), err)` → frontend; `Install()` returns; defer cleanup |
| Interrupted install | `step.Run()` error is `signal: interrupt` / exit 130 | CLI exits 130; GUI reports cancellation |

---

## Goroutine Map

```
GUI side: main goroutine blocks in wails.Run()
  ├─> [Signal Handler]  <-sigChan → CancelInstallation → WaitForCleanup
  ├─> [G1] write password to stdin        ├─> [G2] stdout passthrough
  ├─> [G3] stderr → JSON events → frontend ├─> [G4] cmd.Wait → close(installDone)

CLI side: main goroutine runs the steps
  ├─> [Signal Handler]  <-sigChan → Cancel → SetInterrupted
  └─> during pacstrap: [Scanner] parses progress lines; on <-ctx.Cancel:
        cmd.Process.Kill() → wait for scanner → return "interrupted"
```

---

## Key Data Structures (main fields)

```go
// gui.App — GUI state
currentPage PageID
userAccount UserAccountData; diskSelection DiskSelectionData; fileSystem string
installCmd *exec.Cmd; installCancel context.CancelFunc
installDone chan struct{}; installationRunning atomic.Bool
ctx context.Context; dryRun bool

// install.Installer — shared logic (CLI + GUI)
diskID, username, hostname, region, timezone, fileSystem, luks/lvm/gpu/inputMethod options
Logger; OnProgress; OnError
interrupted atomic.Bool; cancelOnce, cleanupOnce sync.Once; cancel chan struct{}

// shared.InstallContext — per-install state threaded through all steps
DiskID, ESP, Username, Password []byte, Hostname, Region, Timezone, FileSystem
Luks, Lvm, ManualMode, ManualLayout, GPUDriver/DriverPackages, IMPackages
Logger; ProgressCallback; Cancel chan struct{}
```

New state goes into `InstallContext`, not the `Installer` struct. `Timezone` is concrete by the
time the step loop starts: `Installer.Install` resolves an empty one to the region's own clock, so
no step reads an empty field and falls back on its own.

---

## Notes

- **No cancel button in the frontend** — a running install is cancelled via Ctrl+C or window close.
- **Password never in flags** — stdin only (masked in a terminal, piped for automation);
  the LUKS passphrase follows the account password on the same stdin.
- **The GUI runs as a normal user** — all privileged work happens in the `sudo`-spawned CLI.
- **The CLI signal handler works even when launched by the GUI** — the signal goes to a
  specific PID, not the process group.
- **`CancelInstallation` deliberately does not use the `exec.CommandContext` cancel** —
  that SIGKILLs the CLI before it can run its cleanup defers.

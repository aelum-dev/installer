# Execution Flow

Complete map of how every path flows through the codebase.

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
main() [main.go:31]
  │
  ├─> dryRun = env.AELUM_DRY_RUN == "1"
  │
  ├─> gui.NewApp(dryRun)           # Create app struct
  │
  ├─> signal.Notify(SIGINT, SIGTERM)  # Start signal handler BEFORE Wails
  │   └─> goroutine: <-sigChan → CancelInstallation() → WaitForCleanup() → os.Exit(0)
  │
  └─> wails.Run(&options.App{       # Block here until app exits
      │
      ├─> OnStartup: app.StartUp(ctx)
      │
      ├─> OnBeforeClose: app.OnBeforeClose(ctx)  # User closes window
      │
      └─> Bind: app  # Exposes methods to frontend
```

**Key point:** The signal handler starts BEFORE `wails.Run()`, so it's active from program start.

---

## Installation Flow (GUI → CLI)

### 1. User clicks "Install" on ConfirmationPage

```
Frontend: ConfirmationPage.svelte
  │
  └─> dispatchEvent('go-next')
      │
      └─> App.svelte navigates to InstallationPage
          │
          └─> InstallationPage.svelte calls:
              │
              └─> wailsjs/go/gui/App.StartInstallation()
                  │
                  └─> gui/installation.go:28
                      │
                      ├─> CheckRoot()  # Verify sudo access
                      │
                      ├─> findCLIBinary()  # Locate aelum-installer-cli
                      │
                      └─> StartInstallation():
                          │
                          ├─> context.WithCancel()  # Create cancelable context
                          ├─> exec.CommandContext(ctx, "sudo", "aelum-installer-cli", ...)
                          │
                          ├─> cmd.Start()  # Spawn CLI process
                          │
                          ├─> Set state:
                          │   ├─> a.installCmd = cmd
                          │   ├─> a.installCancel = cancel
                          │   ├─> a.installDone = make(chan struct{})
                          │   └─> a.installationRunning.Store(true)
                          │
                          └─> Start 4 goroutines:
                              │
                              ├─> [G1] Write password to stdin
                              ├─> [G2] Forward stdout → os.Stderr
                              ├─> [G3] Read stderr → emit JSON events to frontend
                              └─> [G4] cmd.Wait() → close(installDone) → clear state
```

### 2. CLI Process Starts

```
backend/cli/main.go
  │
  ├─> flag.Parse()
  │
  ├─> Check root: os.Getuid() != 0 → exit(1)
  │
  ├─> Resolve disk + step name
  │
  ├─> readUsername() / readPassword()  # From stdin (masked)
  │
  ├─> NewInstaller(disk, username, password, ...)
  │
  ├─> Wire logger + callbacks
  │
  ├─> defer panic recovery → RunCleanup()
  │
  ├─> signal.Notify(SIGINT)  # Start CLI signal handler
  │   └─> goroutine: <-sigChan → Cancel() → SetInterrupted()
  │
  └─> installer.Install()  # OR installer.RunStep(stepName)
```

### 3. Install() Execution

```
install/installer.go:100
  │
  ├─> defer RunCleanup()  # Runs on ANY exit path
  │
  ├─> context()  # Create InstallContext with cancel channel
  │
  └─> for each step in DefaultPlan():
      │
      ├─> step.Run(ctx)
      │   │
      │   ├─> FormatDiskStep → FormatDisk()
      │   ├─> MountFilesystemStep → MountFilesystem()
      │   ├─> InstallBaseStep → Install() → installPackages()
      │   │   └─> pacstrap with progress tracking
      │   ├─> InstallDesktopStep → InstallDesktop() → installPackages()
      │   ├─> SetupUsersStep → SetupUsers()
      │   └─> ConfigureSystemStep → ConfigureSystem()
      │       (merged: fstab → system → initramfs → bootloader → greetd → plymouth
      │        → zram → resolver link → services → snapper; snapper is last so the "after
      │        installation" snapshot is the finished system - see configure.go)
      │
      └─> if err != nil → return (defer RunCleanup fires)
```

---

## Signal Handling

### GUI Signal Handler (main.go:40)

**Trigger:** User presses Ctrl+C in terminal OR sends SIGTERM

```
<-sigChan (SIGINT or SIGTERM)
  │
  ├─> fmt.Println("Interrupted — cancelling installation...")
  │
  ├─> app.CancelInstallation()
  │   │
  │   └─> a.installCmd.Process.Signal(os.Interrupt)  # Send SIGINT to CLI
  │       (NOTE: Does NOT cancel exec.CommandContext — that would SIGKILL)
  │
  ├─> WaitForCleanup()  # Block until CLI exits
  │   └─> <-a.installDone  # Closed when cmd.Wait() returns
  │
  └─> os.Exit(0)
```

**Safety timeout (main.go:50):** If CLI doesn't exit in 10s, force-unmount `/mnt`.

### CLI Signal Handler (cli/main.go:149)

**Trigger:** CLI receives SIGINT (from GUI's CancelInstallation OR user Ctrl+C)

```
<-sigChan (SIGINT)
  │
  ├─> log.Info("Interrupted — cancelling child process on %s", disk)
  │
  ├─> installer.Cancel()
  │   └─> close(inst.cancel)  # Close cancel channel
  │       └─> installPackages() sees <-ctx.Cancel → kills pacstrap → returns "interrupted"
  │
  └─> installer.SetInterrupted()  # Set atomic.Bool = true
      │
      └─> Install() returns error → main checks Interrupted() → exit(130)
```

---

## Cleanup Paths

Cleanup is triggered by **4 different paths**, all calling `RunCleanup()` (guarded by `sync.Once`):

### Path 1: Normal Install Failure

```
Install() returns error
  │
  └─> defer RunCleanup() fires
      │
      └─> primitives.CleanupMounts(logger)
          ├─> fuser -m -M /mnt  # Find holding PIDs
          ├─> kill -KILL <pids>  # Kill processes holding mounts
          └─> umount -R /mnt  # Recursive unmount
```

### Path 2: SIGINT During Installation

```
GUI signal handler → CancelInstallation()
  │
  ├─> SIGINT sent to CLI
  │
  ├─> CLI signal handler → Cancel() → close(cancel channel)
  │
  ├─> installPackages() sees <-ctx.Cancel → returns
  │
  ├─> Install() returns error
  │
  └─> defer RunCleanup() fires
```

### Path 3: Window Close (OnBeforeClose)

```
User closes window
  │
  └─> OnBeforeClose(ctx) bool
      │
      ├─> if installationRunning:
      │   ├─> CancelInstallation()  # Send SIGINT to CLI
      │   ├─> WaitForCleanup()  # Wait for CLI to exit
      │   └─> logger.Close()
      │
      └─> exec.Command("sudo", "aelum-installer-cli", "--cleanup")
          └─> primitives.CleanupMounts(logger)  # Fallback: unmount /mnt
```

### Path 4: Panic Recovery

```
Panic in Install()
  │
  └─> defer func() { if r := recover(); r != nil { RunCleanup() } }()
      │
      └─> RunCleanup()
```

---

## Error Paths

### Error 1: Sudo Access Required

```
StartInstallation()
  │
  └─> CheckRoot() → Unauthorized
      │
      └─> return error "sudo access required"
          (GUI shows error banner, no installation starts)
```

### Error 2: Installation Already Running

```
StartInstallation()
  │
  └─> installationRunning.Load() == true
      │
      └─> return error "an installation is already in progress"
```

### Error 3: CLI Spawn Failure

```
StartInstallation()
  │
  └─> cmd.Start() → error
      │
      └─> cancel()  # Cancel context
          └─> return error "failed to start CLI"
```

### Error 4: Step Failure

```
Install() → step.Run(ctx) → error
  │
  ├─> if !isInterruptError(err):
  │   └─> emitError(step.Label(), err.Error())  # Frontend shows error
  │
  ├─> return error "step X failed"
  │
  └─> defer RunCleanup() fires
```

### Error 5: Interrupted Installation

```
Install() → step.Run(ctx) → error (interrupted)
  │
  ├─> isInterruptError(err) == true  # "signal: interrupt" or exit code 130
  │
  ├─> return error "step X failed"
  │
  ├─> CLI main checks: installer.Interrupted() == true
  │
  └─> os.Exit(130)  # Standard POSIX SIGINT exit code
```

---

## Goroutine Map

### GUI Side (during installation)

```
main goroutine: wails.Run() → blocks
  │
  ├─> [Signal Handler] <-sigChan → CancelInstallation() → WaitForCleanup()
  │
  ├─> [G1] Write password to stdin (defer stdin.Close())
  │
  ├─> [G2] Forward stdout → os.Stderr
  │
  ├─> [G3] Read stderr → JSON events → frontend
  │
  └─> [G4] cmd.Wait() → close(installDone) → clear state
```

### CLI Side (during installation)

```
main goroutine: installer.Install() → runs steps
  │
  └─> [Signal Handler] <-sigChan → Cancel() → SetInterrupted()
```

### installPackages() Goroutines

```
main goroutine: installPackages() → select on scanDone vs ctx.Cancel
  │
  └─> [Scanner Goroutine] scanner.Scan() → progress tracking
      │
      └─> If ctx.Cancel closed:
          ├─> cmd.Process.Kill()  # Kill pacstrap
          ├─> <-scanDone  # Wait for scanner to exit
          └─> return "interrupted"
```

---

## Key Data Structures

### App (GUI State)

```go
type App struct {
    // Navigation
    currentPage PageID
    
    // Installation state
    installCmd *exec.Cmd           # CLI process
    installCancel context.CancelFunc  # Cancel function (NOT used in CancelInstallation)
    installDone chan struct{}      # Closed when CLI exits
    installationRunning atomic.Bool
    
    // User data
    userAccount UserAccountData
    diskSelection DiskSelectionData
    fileSystem string
    
    // Context
    ctx context.Context
    dryRun bool
}
```

### Installer (Shared Logic)

```go
type Installer struct {
    diskID, username, password, hostname string
    region shared.Region
    fileSystem string
    dryRun bool
    
    Logger Logger
    OnProgress func(label string, percent int, phase string)
    OnError func(label, message string)
    
    interrupted atomic.Bool
    cancelOnce sync.Once
    cleanupOnce sync.Once
    cancel chan struct{}  # Closed on SIGINT
}
```

### InstallContext (Per-Step State)

```go
type InstallContext struct {
    DiskID, ESP, Username, Password, Hostname string
    Region shared.Region
    FileSystem string
    Logger Logger
    
    ProgressCallback func(percent int)
    Cancel chan struct{}  # Same as Installer.cancel
}
```

---

## Critical Paths Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         STARTUP                                  │
│  main() → signal.Notify() → wails.Run()                         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    USER CLICKS INSTALL                           │
│  ConfirmationPage → go-next → InstallationPage                  │
│  → wailsjs/go/gui/App.StartInstallation()                       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                  START_INSTALLATION()                            │
│  1. CheckRoot()                                                 │
│  2. findCLIBinary()                                             │
│  3. exec.CommandContext(ctx, "sudo", "cli", ...)               │
│  4. cmd.Start()                                                 │
│  5. Start 4 goroutines (password, stdout, stderr, wait)        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      CLI PROCESS                                 │
│  1. flag.Parse()                                                │
│  2. Check root                                                  │
│  3. readUsername() / readPassword()                             │
│  4. NewInstaller(...)                                           │
│  5. signal.Notify(SIGINT)                                       │
│  6. installer.Install()                                         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     INSTALL() LOOP                               │
│  defer RunCleanup()                                             │
│  for each step:                                                 │
│    step.Run(ctx)                                                │
│      ├─> FormatDisk                                            │
│      ├─> MountFilesystem                                       │
│      ├─> InstallBase (pacstrap)                                │
│      ├─> InstallDesktop (pacstrap)                             │
│      ├─> SetupUsers                                            │
│      └─> ConfigureSystem (merged)                              │
│  if error → return (defer cleanup fires)                       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    ON SUCCESS                                    │
│  CLI: os.Exit(0)                                                │
│  GUI: [G4] close(installDone) → installationRunning=false       │
│  Frontend: navigation to FinishPage                             │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    ON FAILURE                                    │
│  CLI: os.Exit(1) or os.Exit(130) (if interrupted)              │
│  GUI: [G4] close(installDone) → installationRunning=false       │
│  Frontend: error banner                                         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                  ON Ctrl+C (SIGINT)                              │
│  GUI signal handler:                                            │
│    1. CancelInstallation() → SIGINT to CLI                      │
│    2. WaitForCleanup() → <-installDone                          │
│    3. os.Exit(0)                                                │
│                                                                 │
│  CLI signal handler:                                            │
│    1. Cancel() → close(cancel channel)                          │
│    2. SetInterrupted()                                          │
│    3. installPackages() returns "interrupted"                   │
│    4. Install() returns error                                   │
│    5. defer RunCleanup() → unmount /mnt                         │
│    6. os.Exit(130)                                              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                  ON WINDOW CLOSE                                 │
│  OnBeforeClose():                                               │
│    1. if installationRunning:                                   │
│       ├─> CancelInstallation()                                  │
│       ├─> WaitForCleanup()                                      │
│       └─> logger.Close()                                        │
│    2. sudo cli --cleanup (fallback unmount)                     │
└─────────────────────────────────────────────────────────────────┘
```

---

## Cleanup Guarantee

**`RunCleanup()` is called from 4 paths, guarded by `sync.Once`:**

1. `Install()` defer (normal failure)
2. `RunStep()` defer (single step failure)
3. CLI panic recovery defer
4. `OnBeforeClose` fallback (sudo cli --cleanup)

**Only the first caller does work.** Subsequent calls are no-ops.

---

## Notes

- **No cancel button in frontend** — user can only cancel via Ctrl+C or window close
- **Password never in flags** — read from stdin (masked in terminal)
- **GUI runs as normal user** — all privileged ops via `sudo aelum-installer-cli`
- **CLI signal handler works even when launched by GUI** — signal is sent to specific PID, not process group
- **exec.CommandContext cancel is NOT used in CancelInstallation()** — would SIGKILL before cleanup

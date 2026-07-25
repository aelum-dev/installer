# Project Rules

## Git & Commits
- **Do not auto-commit changes.** Always ask for approval before committing.
- **Don't overthink.** You're thinking budget is limited.
- Binaries go in `build/bin/` — never in project root.

## Building
- **User always runs `build.sh` before testing.** Never assume user is running old binaries or manual `go build` commands.
- When building Go binaries, always use `-o` flag to specify output location:
  ```bash
  go build -o build/bin/aestheticos-installer-cli ./backend/cli/
  go build -o build/bin/aestheticos-helper ./backend/helper/
  ```

## Research & Documentation
- **Do not guess how tools work.** If unsure:
  1. Check `man` pages or `--help` flags
  2. If no documentation available, **ask the user** for documentation on the specific tool
- Never assume behavior of external tools (parted, bootctl, pacstrap, etc.) without verifying
- **Prefer standard tools over custom implementations.** Before writing custom logic for something a well-known tool already handles (e.g., `arch-chroot` instead of manual `/proc` mounting), proactively suggest the standard tool and explain why it's better. Don't just patch within existing patterns — step back and consider if there's a better established approach first.
- **If stuck, ask.** Don't improvise or guess at solutions. If you're unsure about the right approach, stop and ask the user questions before continuing.

## Testing Environments
- **Host machine** = main development system (where you develop the installer)
- **VM** = QEMU virtual machine booting from ISO (testing environment)
- `/home/power007/Projects/shared` is shared between host and VM
- `/shared` is mounted on a VM

## Testing
- Run `go test ./...` before building to catch regressions

## Architecture & Code Organization
- **Follow Project Patterns First:** Before implementing any function, look at how similar operations are already done in the codebase. Match the existing style, conventions, and approaches. Don't introduce new patterns without discussing them first.
- **Single Responsibility:** Each function should do ONE thing. A function named `Validate*` should ONLY validate — never save state, never execute business logic.
- **Separation of Concerns:** Don't mix layers. Navigation functions (`goNext()`) should not save data. Validation functions should not mutate state. Keep each function in its proper layer.
- **Workflow Orchestration:** When you need to do multiple steps (validate → save), create a separate orchestration function. Don't cram multiple responsibilities into one function.
- **Verify Before Implementing:** Before implementing a feature listed in TODOs or issues, check if it's already done. Don't assume something is missing.
- **Ask Before Adding UI:** When adding user-facing features, confirm the UI/UX with the user first. Don't add selectors or options the user didn't ask for.
- **Embrace Corrections:** When the user corrects your architecture, they're usually right. Don't defend your implementation — fix it and learn.
- **Keep It Simple:** Don't add abstractions unless the user explicitly asks for them. If you find yourself creating 5+ files for a "flexible" system when 2 would do, stop and ask.
- **Clean Code:** Use only clean code patterns — no magic numbers, no hardcoded strings, no duplicated logic. Extract constants, use named functions, and keep everything DRY.
- **Verify Each Change:** Build and test after every modification. Don't push 10 changes at once and hope they all work.

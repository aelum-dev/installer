import type { DiskInfo, FileSystem, ManualLayout } from '$lib/types';
import { DefaultFileSystem } from '$lib/constants';
import { t } from '$lib/i18n';
import { uiState } from '$lib/ui.svelte';
import { SetLuks, SetLuksPassphrase, SetLvm, SetManualMode } from '$lib/backend';

// ── Installation State (shared across pages) ──

// Durable wizard data. Ephemeral view state (dialog flags, previews, inline
// errors) lives in uiState (ui.svelte.ts), not here.
export const installState = $state({
	// Language (what the system reads and writes) and location (where the machine
	// is: clock, console keymap, Arch mirrors). The backend pre-fills the location
	// from the language's country, so only someone whose language and country
	// differ has to touch it.
	locale: '',
	region: '',
	// regionChosen records that the person picked a location by hand. A language
	// change re-prefills the location from its country only until then — an
	// explicit choice must not quietly disappear when someone edits their language.
	regionChosen: false as boolean,

	// User account
	username: '',
	password: '',
	confirmPassword: '',

	// Disk selection
	diskId: '',
	disk: null as DiskInfo | null,
	fileSystem: DefaultFileSystem as FileSystem | '',

	// LUKS2 root encryption. luksEnabled is the committed/active flag that
	// drives the toggle look; luksPassphrase is the passphrase held in memory
	// ('' = none held) until the CLI consumes it.
	luksEnabled: false as boolean,
	luksPassphrase: '' as string,

	// LVM root layout. Independent of LUKS: LUKS always sits below LVM, so
	// enabling both encrypts the single physical volume.
	lvmEnabled: false as boolean,

	// Manual partitioning mode
	manualMode: false as boolean,
	manualLayout: null as ManualLayout | null,

	// Dry-run mode (set by backend on startup)
	isDryRun: false,
});

// ── Navigation State ──

export const navState = $state({
	currentPage: 0,
});

// ── Actions ──

export function setDryRun(value: boolean): void {
	installState.isDryRun = value;
}

export function getIsDryRun(): boolean {
	return installState.isDryRun;
}

// ── LUKS encryption helpers ────────────────────────────────────────────────

// requestEnableLuks opens the passphrase entry popup. It does NOT set
// luksEnabled — the toggle only flips on once a valid passphrase is committed.
export function requestEnableLuks(): void {
	uiState.luksDialogOpen = true;
	uiState.luksError = '';
}

// commitLuksPassphrase validates (non-empty, matches confirm) and, on success,
// stores the passphrase and enables encryption. On failure it keeps the popup
// open and sets luksError.
export function commitLuksPassphrase(pass: string, confirm: string): void {
	if (pass === '') {
		uiState.luksError = t('luks.dialog.errorEmpty');
		uiState.luksDialogOpen = true;
		return;
	}
	if (pass !== confirm) {
		uiState.luksError = t('luks.dialog.errorMatch');
		uiState.luksDialogOpen = true;
		return;
	}
	installState.luksPassphrase = pass;
	installState.luksEnabled = true;
	uiState.luksDialogOpen = false;
	uiState.luksError = '';
}

// disableLuks erases the held passphrase and disables encryption. Callers guard
// it with a confirm dialog because it discards encryption.
export function disableLuks(): void {
	installState.luksPassphrase = '';
	installState.luksEnabled = false;
	uiState.luksError = '';
}

// ── LVM helpers ────────────────────────────────────────────────────────────

// toggleLvm flips the LVM layout option. Unlike LUKS it holds no secret, so the
// toggle applies immediately; the choice reaches the backend in
// syncStorageToBackend before the install starts.
export function toggleLvm(): void {
	installState.lvmEnabled = !installState.lvmEnabled;
}

// ── Leaving the disk page backwards ────────────────────────────────────────

// Wipes every answer the disk page owns: the selected disk, its filesystem,
// the LUKS and LVM switches, and manual mode with its plan — back to the
// defaults a first visit shows. Callers: only resetDiskChoicesIfStale below,
// so a fresh visit to the page starts clean. A return from the Confirmation
// page never marks the choices stale: there the person only stepped back to
// read the summary, and the selection stays.
function resetDiskChoices(): void {
	installState.diskId = '';
	installState.disk = null;
	installState.fileSystem = DefaultFileSystem;
	// The passphrase is dropped with the choice it belongs to — a held secret
	// must not outlive the option that asked for it.
	installState.luksPassphrase = '';
	installState.luksEnabled = false;
	installState.lvmEnabled = false;
	installState.manualMode = false;
	installState.manualLayout = null;
	uiState.pendingAddMB = 0;
	uiState.luksError = '';
}

// The wipe is deferred, not instant: on Back the disk page is still on screen
// fading out, and clearing the shared state then would make the choices the
// person just made visibly collapse under the transition. So leaving the page
// backwards only marks the answers stale, and the next mount of the page —
// before its first paint — performs the actual reset. The backend copy is
// cleared immediately by navigation.goBack: it is never on screen, and no
// stale LUKS passphrase should sit in it while the wizard is back on an
// earlier page.
let diskChoicesStale = false;

export function markDiskChoicesStale(): void {
	diskChoicesStale = true;
}

/** Reset a stale answer set once, the next time the wizard shows the disk
 * page — whether that is a fresh mount or a reused instance (see the page's
 * effect). */
export function resetDiskChoicesIfStale(): void {
	if (!diskChoicesStale) return;
	diskChoicesStale = false;
	resetDiskChoices();
}

// ── Backend sync ───────────────────────────────────────────────────────────

// syncStorageToBackend pushes the committed LUKS/LVM state to the Go backend
// before the install starts.
//
// Every Wails binding call runs on its own goroutine in the backend, so these
// calls MUST be awaited: firing them and then immediately calling
// StartInstallation() lets the backend read a half-applied state. If
// SetLuks(true) lands but SetLuksPassphrase() has not, the CLI is spawned with
// --luks and no passphrase on stdin and aborts before its first step — the user
// sees a failure at 0% that has nothing to do with the disk. The reverse order
// (flag first) also opens a window where encryption is on with no passphrase;
// sending the passphrase first means that state is never observable.
export async function syncStorageToBackend(): Promise<void> {
	if (installState.luksEnabled && installState.luksPassphrase !== '') {
		await SetLuksPassphrase(installState.luksPassphrase);
		await SetLuks(true);
	} else {
		await SetLuks(false);
	}
	// LVM is independent of LUKS and carries no secret, so it is always pushed
	// (false included) — a stale backend flag must never survive a toggle off.
	await SetLvm(installState.lvmEnabled);
	// Partitioning mode, same rule: the manual page's Next saved the layout and
	// set the backend flag, but walking back to Automatic must clear it —
	// otherwise StartInstallation spawns the CLI with an abandoned
	// --manual-layout and installs a plan nobody chose.
	await SetManualMode(installState.manualMode);
}

// setFileSystem selects the root filesystem.
export function setFileSystem(fs: FileSystem): void {
	installState.fileSystem = fs;
}

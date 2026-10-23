import type { DiskInfo, FileSystem, ManualLayout } from '$lib/types';
import { DefaultFileSystem } from '$lib/constants';
import { t } from '$lib/i18n';
import { uiState } from '$lib/ui.svelte';
import { SetLuks, SetLuksPassphrase } from '$lib/backend';

// ── Installation State (shared across pages) ──

// Durable wizard data. Ephemeral view state (dialog flags, previews, inline
// errors) lives in uiState (ui.svelte.ts), not here.
export const installState = $state({
	// Language selection
	locale: '',

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

// ── Backend sync ───────────────────────────────────────────────────────────

// syncEncryptionToBackend pushes the committed LUKS state to the Go backend
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
export async function syncEncryptionToBackend(): Promise<void> {
	if (installState.luksEnabled && installState.luksPassphrase !== '') {
		await SetLuksPassphrase(installState.luksPassphrase);
		await SetLuks(true);
	} else {
		await SetLuks(false);
	}
}

// setFileSystem selects the root filesystem.
export function setFileSystem(fs: FileSystem): void {
	installState.fileSystem = fs;
}

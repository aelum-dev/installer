import { installState, navState } from '$lib/store.svelte';
import { uiState } from '$lib/ui.svelte';
import { PAGE_IDS } from '$lib/types';
import {
	RebootAndClose,
	SaveDiskSelection,
	SaveManualLayout,
	SaveUserAccount,
	setCurrentPage,
} from '$lib/backend';

// Wizard navigation controller.
//
// The frontend owns the "may we advance?" decision (see page-validation.ts);
// goNext/goBack only commit the current page's data and move the backend's
// page pointer. Backend bindings can still reject (I/O, serialization), so a
// transition returns an error message for the caller to surface.

/**
 * Navigation callbacks shared with pages through Svelte context. App.svelte
 * provides this; pages that need to drive navigation (the Confirmation dialog,
 * the auto-advance when installation completes) consume it instead of
 * dispatching DOM events.
 */
export interface WizardNavigation {
	next: () => void;
	back: () => void;
	finish: () => void;
}

export const WIZARD_NAV = Symbol('wizard-navigation');

// Whether the user confirmed the destructive-install dialog. The Confirmation
// page's Next shows that dialog once; the confirm action advances instead of
// re-prompting.
let _installationConfirmed = false;

/** Open the destructive-install confirmation dialog. */
export function requestConfirmation(): void {
	uiState.showConfirmDialog = true;
}

/** User accepted the destructive-install dialog. */
export function confirmInstallation(): void {
	_installationConfirmed = true;
}

/** User dismissed the destructive-install dialog. */
export function cancelConfirmation(): void {
	_installationConfirmed = false;
}

/** Advance one page. Returns an error message to surface, or null on success. */
export async function goNext(currentPage: number): Promise<string | null> {
	// The Confirmation page gates advancement behind a destructive-action dialog.
	if (currentPage === PAGE_IDS.CONFIRMATION && !_installationConfirmed) {
		requestConfirmation();
		return null; // Don't advance yet
	}
	_installationConfirmed = false; // Reset for the next visit

	try {
		await commitPageData(currentPage);

		const nextPage = currentPage + 1;
		await setCurrentPage(nextPage);

		navState.currentPage = nextPage;
	} catch (err) {
		return errorMessage(err);
	}

	return null;
}

/** Go back one page. Returns an error message to surface, or null on success. */
export async function goBack(currentPage: number): Promise<string | null> {
	const prevPage = currentPage - 1;
	try {
		await setCurrentPage(prevPage);
	} catch (err) {
		return errorMessage(err);
	}

	navState.currentPage = prevPage;

	return null;
}

/** Reboot into the installed system. Returns an error message, or null. */
export async function finish(): Promise<string | null> {
	try {
		await RebootAndClose();
		return null;
	} catch (err) {
		return err instanceof Error ? err.message : 'Failed to reboot';
	}
}

function errorMessage(err: unknown): string {
	return err instanceof Error ? err.message : String(err);
}

// Persist the current page's data to the backend app state before advancing.
// This is a save, not a validation: the rules that gate the wizard live in
// page-validation.ts and run before goNext is ever called.
async function commitPageData(page: number): Promise<void> {
	if (page === PAGE_IDS.USER_ACCOUNT) {
		await SaveUserAccount({
			username: installState.username,
			password: installState.password,
			confirmPassword: installState.confirmPassword,
		});
	}

	if (page === PAGE_IDS.DISK_SELECTION) {
		await SaveDiskSelection({
			diskId: installState.diskId,
			size: installState.disk?.size ?? 0,
			fileSystem: installState.fileSystem,
		});

		// Manual (dualboot) mode: persist the designed plan so the installer
		// only formats the Boot/Root partitions.
		if (installState.manualMode && installState.manualLayout) {
			await SaveManualLayout(installState.manualLayout);
		}
	}
}

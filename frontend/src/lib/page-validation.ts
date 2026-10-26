// Wizard page validity, shared by the Next button and the error modal.
//
// Both paths use the same rule engine (`validate.ts`) and the same manual-plan
// checks (`manual-layout.ts`), so "Next is enabled" and "Next explains why"
// can never disagree.

import { installState } from '$lib/store.svelte';
import { PAGE_IDS } from '$lib/types';
import { MinDiskSizeBytes, MinManualDiskSizeBytes, PartitionTableGPT } from '$lib/constants';
import { t } from '$lib/i18n';
import {
	confirmPasswordRule,
	passwordRule,
	usernameRule,
	type ConfirmPasswordRule,
	type UsernameRule,
} from '$lib/validate';
import { manualLayoutIssues, type ManualLayoutIssue } from '$lib/manual-layout';

/** Whether the wizard may advance from `page`. */
export function isPageValid(page: number): boolean {
	switch (page) {
		case PAGE_IDS.WELCOME:
			return !!installState.locale;
		case PAGE_IDS.USER_ACCOUNT:
			return userAccountValidationReasons().length === 0;
		case PAGE_IDS.DISK_SELECTION:
			return diskSelectionValidationReasons().length === 0;
		case PAGE_IDS.CONFIRMATION:
			return true; // Validated by previous pages
		case PAGE_IDS.INSTALLATION:
			return false; // Disable navigation during installation
		case PAGE_IDS.FINISH:
			return true;
		default:
			return false;
	}
}

/**
 * Human-readable reasons the user can't proceed. Used by the validation error
 * modal. Pages that are normally navigable (Confirmation, Installation) still
 * explain what the click did.
 */
export function getPageValidationReasons(page: number): string[] {
	switch (page) {
		case PAGE_IDS.WELCOME:
			return installState.locale ? [] : [t('validation.modal.selectLanguage')];
		case PAGE_IDS.USER_ACCOUNT:
			return userAccountValidationReasons();
		case PAGE_IDS.DISK_SELECTION:
			return diskSelectionValidationReasons();
		case PAGE_IDS.CONFIRMATION:
			return [t('validation.modal.reviewInstallation')];
		case PAGE_IDS.INSTALLATION:
			return [t('validation.modal.waitInstallation')];
		default:
			return [];
	}
}

function userAccountValidationReasons(): string[] {
	const reasons: string[] = [];
	const { username, password, confirmPassword } = installState;

	const nameRule = usernameRule(username);
	if (nameRule) reasons.push(usernameReason(nameRule, username));

	const passRule = passwordRule(password);
	if (passRule) reasons.push(t(`validation.modal.${passRule}`));

	// Only mention the confirmation when there is a password to compare with;
	// otherwise the password error already explains the problem.
	if (password) {
		const confirmRule = confirmPasswordRule(confirmPassword, password);
		if (confirmRule) reasons.push(confirmReason(confirmRule));
	}

	return reasons;
}

function usernameReason(rule: UsernameRule, username: string): string {
	return t(`validation.modal.${rule}`, { name: username });
}

function confirmReason(rule: ConfirmPasswordRule): string {
	// The modal copy is shared for "empty" and "mismatch": both mean the two
	// fields do not agree, and there is no dedicated modal message for an empty
	// confirmation field.
	const key = rule === 'passwordsMatch' ? 'passwordsNoMatch' : rule;
	return t(`validation.modal.${key}`);
}

function diskSelectionValidationReasons(): string[] {
	return installState.manualMode
		? manualLayoutValidationReasons()
		: autoDiskValidationReasons();
}

function autoDiskValidationReasons(): string[] {
	const reasons: string[] = [];
	if (!installState.diskId) reasons.push(t('validation.modal.selectDisk'));
	if (!installState.fileSystem) reasons.push(t('validation.modal.selectFileSystem'));
	if (installState.disk && installState.disk.size < MinDiskSizeBytes) {
		reasons.push(t('validation.modal.diskTooSmall'));
	}
	return reasons;
}

function manualLayoutValidationReasons(): string[] {
	const reasons: string[] = [];
	if (!installState.diskId) {
		reasons.push(t('validation.modal.selectDisk'));
		return reasons;
	}
	if (!installState.fileSystem) {
		reasons.push(t('validation.modal.selectFileSystem'));
		return reasons;
	}
	if (installState.disk && installState.disk.size < MinManualDiskSizeBytes) {
		reasons.push(
			t('validation.modal.diskTooSmallManual', {
				min: (MinManualDiskSizeBytes / 1024 ** 3).toFixed(1),
			}),
		);
	}

	// Manual mode applies the plan with sgdisk, which refuses to save any change
	// on a disk whose partition table is not a GPT. An empty value is not a
	// reason: it means the disk has no table at all (sgdisk creates one) or the
	// backend could not read it, and the backend re-checks before it writes.
	if (installState.disk?.partitionTable && installState.disk.partitionTable !== PartitionTableGPT) {
		reasons.push(t('validation.modal.notGPTDisk'));
		return reasons;
	}

	const layout = installState.manualLayout;
	if (!layout) {
		// Manual mode with no plan yet (still loading, or never fetched): the
		// page can't advance, and the modal must say why instead of opening empty.
		reasons.push(t('validation.selectDiskManual'));
		return reasons;
	}

	for (const issue of manualLayoutIssues(layout, {
		lvm: installState.lvmEnabled,
		luks: installState.luksEnabled,
	})) {
		reasons.push(manualIssueReason(issue));
	}
	return reasons;
}

function manualIssueReason(issue: ManualLayoutIssue): string {
	switch (issue.kind) {
		case 'espCount':
			return t('validation.modal.espRequired', { n: issue.count });
		case 'systemCount':
			return t('validation.modal.systemRequired', { n: issue.count });
		case 'newPartNoType':
			return t('validation.modal.newPartNoType');
		case 'espTooSmall':
			return t('validation.modal.espTooSmall', { size: issue.sizeMB });
		case 'systemTooSmall':
			return t('validation.modal.systemTooSmall', { size: issue.sizeMB, min: issue.minMB });
		case 'lvmSystemTooSmall':
			return t('validation.modal.lvmSystemTooSmall', {
				size: issue.sizeMB,
				min: issue.minMB,
			});
		case 'invalidType':
			return t('validation.modal.invalidType', { type: issue.type });
		case 'multipleRemaining':
			return t('validation.modal.multipleRemaining');
		case 'partitionAfterRemaining':
			return t('validation.modal.partitionAfterRemaining');
		case 'exceedsFreeSpace':
			return t('validation.modal.exceedsFreeSpace', {
				size: issue.sizeMB,
				free: issue.freeMB,
			});
		case 'exceedsDisk':
			return t('validation.modal.exceedsDisk', {
				size: issue.usedMB,
				disk: issue.diskMB,
			});
	}
}

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, unmount, flushSync, tick } from 'svelte';
import App from './App.svelte';
import { installState, navState } from '$lib/store.svelte';
import { PAGE_IDS } from '$lib/types';

// Mounting App pulls every page and binding in. Only the calls App makes on
// startup (and any page reached) need real values; the rest just have to exist.
vi.mock('@wails/go/gui/App.js', () => {
	const resolved = () => Promise.resolve();
	return {
		ClearStartPage: vi.fn(resolved),
		GetDiskPartitions: vi.fn(resolved),
		GetDisks: vi.fn(resolved),
		GetDryRun: vi.fn(() => Promise.resolve(false)),
		GetLanguages: vi.fn(() => Promise.resolve([])),
		GetManualLayout: vi.fn(resolved),
		GetSelectedLanguage: vi.fn(() => Promise.resolve('')),
		GetStartPage: vi.fn(() => Promise.resolve(-1)),
		RebootAndClose: vi.fn(resolved),
		SaveDiskSelection: vi.fn(resolved),
		SaveManualLayout: vi.fn(resolved),
		SaveUserAccount: vi.fn(resolved),
		SetCurrentPage: vi.fn(resolved),
		SetLuks: vi.fn(resolved),
		SetLuksPassphrase: vi.fn(resolved),
		SetSelectedLanguage: vi.fn(resolved),
		SetWallpaper: vi.fn(resolved),
		StartInstallation: vi.fn(resolved),
		ValidateDiskSelection: vi.fn(resolved),
		ValidateManualLayout: vi.fn(resolved),
		ValidateUserAccount: vi.fn(resolved),
	};
});

vi.mock('@wails/runtime/runtime', () => ({
	EventsOn: vi.fn(),
	LogInfo: vi.fn(),
}));

let app: ReturnType<typeof mount> | null = null;
let target: HTMLDivElement;

beforeEach(() => {
	installState.username = '';
	installState.password = '';
	installState.confirmPassword = '';
	navState.currentPage = PAGE_IDS.USER_ACCOUNT;

	target = document.createElement('div');
	document.body.appendChild(target);
	app = mount(App, { target });
	flushSync();
});

afterEach(() => {
	if (app) {
		unmount(app);
		app = null;
	}
	target?.remove();
	document.body.innerHTML = '';
});

async function pressEnterOn(el: Element): Promise<void> {
	el.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
	flushSync();
	await tick();
}

describe('App — Enter advances the wizard', () => {
	it('opens the validation modal when Enter advances an invalid page', async () => {
		await pressEnterOn(document.body);

		expect(document.querySelector('[role="dialog"]')).not.toBeNull();
	});

	it('leaves Enter to a focused control (the Next button handles it)', async () => {
		const next = target.querySelector<HTMLButtonElement>('button[aria-label="Next"]');
		expect(next).not.toBeNull();

		await pressEnterOn(next!);

		expect(document.querySelector('[role="dialog"]')).toBeNull();
	});
});

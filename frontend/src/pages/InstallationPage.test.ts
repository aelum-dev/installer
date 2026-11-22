import { describe, it, expect, vi, afterEach } from 'vitest';
import { mount, unmount, flushSync } from 'svelte';
import InstallationPage from './InstallationPage.svelte';
import { WIZARD_NAV, type WizardNavigation } from '$lib/navigation';

const mocks = vi.hoisted(() => {
	const released: ReturnType<typeof vi.fn>[] = [];
	return {
		released,
		// Every subscription the page makes gets its own teardown spy, so a test
		// can prove they are all called when the screen goes away.
		eventsOn: vi.fn((_name: string, _cb: (...args: unknown[]) => void) => {
			const off = vi.fn();
			released.push(off);
			return off;
		}),
		startInstallation: vi.fn(() => Promise.resolve()),
		logInfo: vi.fn(),
		syncStorageToBackend: vi.fn(() => Promise.resolve()),
	};
});

vi.mock('$lib/backend', () => ({
	StartInstallation: mocks.startInstallation,
	EventsOn: mocks.eventsOn,
	LogInfo: mocks.logInfo,
}));

vi.mock('$lib/store.svelte', async (importOriginal) => {
	const actual = await importOriginal<typeof import('$lib/store.svelte')>();
	return { ...actual, syncStorageToBackend: mocks.syncStorageToBackend };
});

let app: ReturnType<typeof mount> | null = null;
let target: HTMLDivElement;

function render(nav: Partial<WizardNavigation>): void {
	target = document.createElement('div');
	document.body.appendChild(target);
	app = mount(InstallationPage, {
		target,
		context: new Map<symbol, WizardNavigation>([
			[
				WIZARD_NAV,
				{ next: vi.fn(), back: vi.fn(), finish: vi.fn(), cancelled: vi.fn(), ...nav },
			],
		]),
	});
	// The subscriptions are made inside an effect, so they exist after this flush.
	flushSync();
}

function handlerFor(event: string): ((data?: unknown) => void) | undefined {
	const call = mocks.eventsOn.mock.calls.find(([name]) => name === event);
	return call?.[1] as ((data?: unknown) => void) | undefined;
}

afterEach(() => {
	if (app) {
		unmount(app);
		app = null;
	}
	target?.remove();
	mocks.eventsOn.mockClear();
	mocks.released.length = 0;
});

describe('InstallationPage cancellation', () => {
	it('hands the screen over to the cancelled page when the run is stopped', () => {
		const nav = { cancelled: vi.fn(), next: vi.fn() };
		render(nav);

		const handleCancelled = handlerFor('installation:cancelled');
		expect(handleCancelled, 'the page must subscribe to installation:cancelled').toBeTypeOf('function');

		handleCancelled?.();
		flushSync();

		expect(nav.cancelled).toHaveBeenCalledTimes(1);
		// Not a completion: a stopped run never advances to the finish page.
		expect(nav.next).not.toHaveBeenCalled();
		expect(mocks.logInfo).toHaveBeenCalledWith('[InstallationPage] Installation cancelled');
	});

	it('releases every subscription when the screen goes away', () => {
		// The cancelled screen replaces this one, so a subscription left behind
		// would keep firing against a page nobody is looking at.
		render({ cancelled: vi.fn() });
		expect(mocks.released.length).toBeGreaterThan(0);

		const mounted = app;
		if (!mounted) {
			throw new Error('the page did not mount');
		}
		unmount(mounted);
		app = null;
		flushSync();

		for (const off of mocks.released) {
			expect(off).toHaveBeenCalledTimes(1);
		}
	});
});

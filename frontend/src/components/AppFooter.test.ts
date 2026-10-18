import { describe, it, expect, afterEach, vi } from 'vitest';
import { mount, unmount, flushSync } from 'svelte';
import AppFooter from './AppFooter.svelte';
import { PAGE_IDS } from '$lib/types';

let app: ReturnType<typeof mount> | null = null;
let target: HTMLDivElement;

function render(currentPage: number, props: Record<string, unknown> = {}): HTMLDivElement {
	target = document.createElement('div');
	document.body.appendChild(target);
	app = mount(AppFooter, {
		target,
		props: { currentPage, onNext: vi.fn(), onBack: vi.fn(), onFinish: vi.fn(), ...props },
	});
	flushSync();
	return target;
}

function buttons(root: HTMLElement): HTMLButtonElement[] {
	return Array.from(root.querySelectorAll<HTMLButtonElement>('button'));
}

function teardown(): void {
	if (app) {
		unmount(app);
		app = null;
	}
	target?.remove();
}

afterEach(teardown);

describe('AppFooter', () => {
	it('puts Next before Back in the DOM so Tab reaches the primary action first', () => {
		const footer = render(PAGE_IDS.USER_ACCOUNT);

		const labels = buttons(footer).map((b) => b.getAttribute('aria-label'));

		expect(labels).toEqual(['Next', 'Back']);
	});

	it('calls onNext when Next is clicked', () => {
		const onNext = vi.fn();
		const footer = render(PAGE_IDS.USER_ACCOUNT, { onNext });

		buttons(footer)[0].click();

		expect(onNext).toHaveBeenCalledOnce();
	});

	it('calls onBack when Back is clicked', () => {
		const onBack = vi.fn();
		const footer = render(PAGE_IDS.USER_ACCOUNT, { onBack });

		footer.querySelector<HTMLButtonElement>('.nav-btn--back')?.click();

		expect(onBack).toHaveBeenCalledOnce();
	});

	it('shows only Next on Welcome, which has nothing to go back to', () => {
		const footer = render(PAGE_IDS.WELCOME);

		expect(buttons(footer).map((b) => b.getAttribute('aria-label'))).toEqual(['Next']);
	});

	it('shows neither navigation control on the terminal pages', () => {
		const installation = render(PAGE_IDS.INSTALLATION);
		expect(buttons(installation)).toHaveLength(0);

		teardown();
		const finish = render(PAGE_IDS.FINISH);
		expect(buttons(finish).map((b) => b.textContent?.trim())).toEqual(['Reboot']);
	});
});

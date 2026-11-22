import { describe, it, expect, afterEach, vi } from 'vitest';
import { mount, unmount, flushSync } from 'svelte';
import AppFooter from './AppFooter.svelte';
import { PAGE_IDS } from '$lib/types';

let app: ReturnType<typeof mount> | null = null;
let target: HTMLDivElement;

function render(currentPage: number, onExit = vi.fn()): HTMLDivElement {
	target = document.createElement('div');
	document.body.appendChild(target);
	app = mount(AppFooter, {
		target,
		props: { currentPage, onNext: () => {}, onBack: () => {}, onFinish: () => {}, onExit },
	});
	flushSync();
	return target;
}

afterEach(() => {
	if (app) {
		unmount(app);
		app = null;
	}
	target?.remove();
});

describe('AppFooter on the cancelled screen', () => {
	it('offers Exit in the trailing slot and no navigation', () => {
		const onExit = vi.fn();
		const footer = render(PAGE_IDS.CANCELLED, onExit);

		const exit = footer.querySelector<HTMLButtonElement>('.btn-exit');
		expect(exit?.textContent?.trim()).toBe('Exit');
		expect(footer.querySelectorAll('.nav-btn')).toHaveLength(0);
		expect(footer.querySelector('.btn-reboot')).toBeNull();

		exit?.click();
		flushSync();
		expect(onExit).toHaveBeenCalledTimes(1);
	});

	it('shows no step dots: none of them describes this screen', () => {
		const footer = render(PAGE_IDS.CANCELLED);

		// The run stopped partway through the wizard, and the dot row either
		// overstates or understates that. The screen states the fact in words
		// instead, so the row is simply absent.
		expect(footer.querySelectorAll('.step-dot')).toHaveLength(0);
	});

	it('leaves the Finish page alone', () => {
		const footer = render(PAGE_IDS.FINISH);

		expect(footer.querySelector<HTMLButtonElement>('.btn-reboot')).not.toBeNull();
		expect(footer.querySelector('.btn-exit')).toBeNull();
	});
});

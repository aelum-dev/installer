import { describe, it, expect, afterEach, vi } from 'vitest';
import { mount, unmount, flushSync } from 'svelte';
import UserAccountPage from './UserAccountPage.svelte';
import { installState } from '$lib/store.svelte';

let app: ReturnType<typeof mount> | null = null;
let target: HTMLDivElement;

function render(): HTMLDivElement {
	target = document.createElement('div');
	document.body.appendChild(target);
	app = mount(UserAccountPage, { target });
	flushSync();
	return target;
}

afterEach(() => {
	if (app) {
		unmount(app);
		app = null;
	}
	target?.remove();
	installState.username = '';
	installState.password = '';
	installState.confirmPassword = '';
});

describe('UserAccountPage', () => {
	it('starts quiet: no inline error is shown before a field is touched', () => {
		const page = render();

		for (const id of ['username', 'password', 'confirmPassword']) {
			const error = page.querySelector<HTMLElement>(`#${id} + .error`);
			expect(error).not.toBeNull();
			expect(error?.classList.contains('visible')).toBe(false);
			expect(error?.textContent).toBe('');
		}
	});

	// Regression: the username field used to be auto-selected on mount. The
	// page-change focus then moved to the heading, blurring the field, marking
	// it touched and showing "Username is required." before any input.
	it('does not auto-focus or select the username field on mount', () => {
		const selectSpy = vi.spyOn(HTMLInputElement.prototype, 'select');

		const page = render();

		expect(selectSpy).not.toHaveBeenCalled();
		expect(document.activeElement).not.toBe(page.querySelector('#username'));

		selectSpy.mockRestore();
	});
});

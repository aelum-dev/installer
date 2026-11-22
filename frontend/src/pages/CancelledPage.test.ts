import { describe, it, expect, afterEach } from 'vitest';
import { mount, unmount, flushSync } from 'svelte';
import CancelledPage from './CancelledPage.svelte';
import { installState } from '$lib/store.svelte';

let app: ReturnType<typeof mount> | null = null;
let target: HTMLDivElement;

function render(): HTMLDivElement {
	target = document.createElement('div');
	document.body.appendChild(target);
	app = mount(CancelledPage, { target });
	flushSync();
	return target;
}

afterEach(() => {
	if (app) {
		unmount(app);
		app = null;
	}
	target?.remove();
	installState.diskId = '';
});

describe('CancelledPage', () => {
	it('names the screen and the disk it stopped on', () => {
		installState.diskId = '/dev/vda';
		const page = render();

		// App.svelte moves focus to the first heading of a page, so the screen's
		// first focusable landmark has to be this title.
		expect(page.querySelector('h2')?.textContent?.trim()).toBe('Installation Cancelled');
		expect(page.textContent).toContain('/dev/vda');
	});

	it('states the limit of what ran without claiming the disk is unusable', () => {
		installState.diskId = '/dev/sdb';
		const body = render().querySelector('.message')?.textContent ?? '';

		expect(body).toContain('not finished');
		expect(body).toContain('remaining steps did not run');
	});
});

import { describe, it, expect, afterEach } from 'vitest';
import { mount, unmount, flushSync } from 'svelte';
import LuksPassphraseDialog from './LuksPassphraseDialog.svelte';

// The inline "required" hint is a once-left-the-field signal. Pressing Cancel
// blurs the passphrase on mousedown, before the button's click resets and
// closes the dialog — a blur-to-button must not paint that error at the very
// moment the person dismisses the dialog (uk: "Пароль не може бути порожнім.").

let app: ReturnType<typeof mount> | null = null;
let target: HTMLDivElement;

function render(): HTMLElement {
	target = document.createElement('div');
	document.body.appendChild(target);
	app = mount(LuksPassphraseDialog, { target, props: { show: true } });
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

/** Fires the blur jsdom would fire when focus moves to `relatedTarget`. */
function blur(el: Element, relatedTarget: Element | null): void {
	el.dispatchEvent(new FocusEvent('blur', { relatedTarget }));
	flushSync();
}

describe('LuksPassphraseDialog inline validation', () => {
	it('stays quiet when the passphrase blurs to the Cancel button', () => {
		const root = render();
		blur(root.querySelector('#luks-passphrase')!, root.querySelector('.btn-cancel'));
		expect(root.querySelector('.field-error')).toBeNull();
	});

	it('stays quiet when the passphrase blurs to a show/hide toggle', () => {
		const root = render();
		blur(root.querySelector('#luks-passphrase')!, root.querySelector('.toggle-visibility'));
		expect(root.querySelector('.field-error')).toBeNull();
	});

	it('stays quiet when the dialog is simply dismissed (no focus target)', () => {
		const root = render();
		blur(root.querySelector('#luks-passphrase')!, null);
		expect(root.querySelector('.field-error')).toBeNull();
	});

	it('shows the empty error once focus moves to the confirm field', () => {
		const root = render();
		blur(root.querySelector('#luks-passphrase')!, root.querySelector('#luks-confirm'));
		expect(root.querySelector('.field-error')?.textContent).toBe('Passphrase cannot be empty.');
	});
});

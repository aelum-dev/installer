import { describe, it, expect } from 'vitest';
import { shouldAdvanceOnEnter } from './enter-advance';

const READY = { canAdvance: true, blocked: false };

// A plain object stands in for the event: the predicate only reads these
// fields, and this avoids depending on how a dispatched event's target is
// retained after dispatch.
function keyEvent(target: EventTarget | null, init: Partial<KeyboardEvent> = {}): KeyboardEvent {
	return {
		key: 'Enter',
		repeat: false,
		defaultPrevented: false,
		target,
		...init,
	} as unknown as KeyboardEvent;
}

describe('shouldAdvanceOnEnter', () => {
	it('advances from ordinary content and from text fields', () => {
		expect(shouldAdvanceOnEnter(keyEvent(document.body), READY)).toBe(true);
		expect(shouldAdvanceOnEnter(keyEvent(document.createElement('input')), READY)).toBe(true);
	});

	it('ignores non-Enter keys, held keys and already-handled keys', () => {
		const input = document.createElement('input');

		expect(shouldAdvanceOnEnter(keyEvent(input, { key: 'a' }), READY)).toBe(false);
		expect(shouldAdvanceOnEnter(keyEvent(input, { repeat: true }), READY)).toBe(false);
		expect(shouldAdvanceOnEnter(keyEvent(input, { defaultPrevented: true }), READY)).toBe(false);
	});

	it('does nothing when the page has no Next control or a modal is open', () => {
		expect(shouldAdvanceOnEnter(keyEvent(document.body), { canAdvance: false, blocked: false })).toBe(false);
		expect(shouldAdvanceOnEnter(keyEvent(document.body), { canAdvance: true, blocked: true })).toBe(false);
	});

	it('leaves Enter to controls that use it themselves', () => {
		const anchor = document.createElement('a');
		anchor.setAttribute('href', '#');

		const controls: Element[] = [
			document.createElement('button'),
			anchor,
			document.createElement('select'),
			document.createElement('textarea'),
		];
		const row = document.createElement('tr');
		row.setAttribute('role', 'button');
		controls.push(row);

		for (const control of controls) {
			expect(shouldAdvanceOnEnter(keyEvent(control), READY)).toBe(false);
		}
	});

	it('never navigates from inside an open dialog or listbox', () => {
		const dialog = document.createElement('div');
		dialog.setAttribute('role', 'dialog');
		const field = document.createElement('input');
		dialog.appendChild(field);
		expect(shouldAdvanceOnEnter(keyEvent(field), READY)).toBe(false);

		const menu = document.createElement('div');
		menu.setAttribute('role', 'listbox');
		const option = document.createElement('button');
		option.setAttribute('role', 'option');
		menu.appendChild(option);
		expect(shouldAdvanceOnEnter(keyEvent(option), READY)).toBe(false);
	});
});

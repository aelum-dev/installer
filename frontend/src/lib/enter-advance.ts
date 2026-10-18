// Enter-to-advance key routing.
//
// Pressing Enter anywhere that isn't a control with its own Enter behaviour
// advances the wizard, matching the footer's Next button. Kept pure (no store
// or backend imports) so the rules can be unit tested without mounting the app.

// Controls that consume Enter themselves: buttons and links activate, textareas
// insert a newline, the ARIA listbox and its options move or pick, the manual
// partition row toggles itself. Anything inside an open dialog belongs to the
// dialog (confirm/cancel), never to navigation behind it.
const ENTER_OWNED_BY =
	'button, a[href], select, textarea, [role="button"], [role="option"], [role="listbox"], [role="dialog"], [contenteditable="true"]';

export interface EnterAdvanceOptions {
	/** The current page has a Next control (see canGoNext in navigation.ts). */
	canAdvance: boolean;
	/** A modal is open, so Enter belongs to the dialog. */
	blocked: boolean;
}

export function shouldAdvanceOnEnter(
	event: KeyboardEvent,
	{ canAdvance, blocked }: EnterAdvanceOptions,
): boolean {
	if (event.key !== 'Enter' || event.repeat || event.defaultPrevented) return false;
	if (!canAdvance || blocked) return false;
	const target = event.target;
	return !(target instanceof Element && target.closest(ENTER_OWNED_BY) !== null);
}

// Shared number/string formatting helpers.

// groupDigits renders an integer with a space as the thousands separator
// (e.g. 487875 -> "487 875"). Used wherever a raw MB/GB figure is shown next
// to a human-readable size.
export function groupDigits(value: number): string {
	return String(value).replace(/\B(?=(\d{3})+(?!\d))/g, ' ');
}

// escapeHtml neutralises text that is interpolated into a translation rendered
// with {@html}. Translation templates are trusted; their runtime arguments
// (e.g. a disk model reported by the hardware) are not.
export function escapeHtml(value: string): string {
	return value
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;')
		.replace(/'/g, '&#39;');
}

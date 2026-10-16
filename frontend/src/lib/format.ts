// Shared number formatting helpers.

// groupDigits renders an integer with a space as the thousands separator
// (e.g. 487875 -> "487 875"). Used wherever a raw MB/GB figure is shown next
// to a human-readable size.
export function groupDigits(value: number): string {
	return String(value).replace(/\B(?=(\d{3})+(?!\d))/g, ' ');
}

// Shared number/string formatting helpers.

// groupDigits renders an integer with a space as the thousands separator
// (e.g. 487875 -> "487 875"). Used wherever a raw MB/GB figure is shown next
// to a human-readable size.
export function groupDigits(value: number): string {
	return String(value).replace(/\B(?=(\d{3})+(?!\d))/g, ' ');
}

const BYTE_UNITS = ['B', 'KB', 'MB', 'GB', 'TB'] as const;

// formatBytes renders a byte count in the largest binary unit that fits, with
// one decimal for every unit above bytes (e.g. 8589934592 -> "8.0 GB"). Used
// where only a raw byte count is available (the backend ships its own
// sizeFormatted string for disks).
export function formatBytes(bytes: number): string {
	if (bytes <= 0) return `0 ${BYTE_UNITS[0]}`;
	const k = 1024;
	const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), BYTE_UNITS.length - 1);
	const size = bytes / Math.pow(k, i);
	return i === 0 ? `${Math.round(size)} ${BYTE_UNITS[0]}` : `${size.toFixed(1)} ${BYTE_UNITS[i]}`;
}

// mbToGB converts a MiB figure to GiB rounded to one decimal, for callers that
// print the unit separately (e.g. "1.5" + t('unit.gb')).
export function mbToGB(mb: number): number {
	return Math.round((mb / 1024) * 10) / 10;
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

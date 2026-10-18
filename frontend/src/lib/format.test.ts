import { describe, it, expect } from 'vitest';
import { escapeHtml, formatBytes, groupDigits, mbToGB } from './format';

describe('groupDigits', () => {
	it('groups thousands with spaces', () => {
		expect(groupDigits(487875)).toBe('487 875');
	});

	it('leaves short numbers unchanged', () => {
		expect(groupDigits(128)).toBe('128');
	});
});

describe('formatBytes', () => {
	it('renders whole bytes without a decimal', () => {
		expect(formatBytes(512)).toBe('512 B');
	});

	it('picks the largest binary unit with one decimal', () => {
		expect(formatBytes(8 * 1024 ** 3)).toBe('8.0 GB');
		expect(formatBytes(1536 * 1024 ** 2)).toBe('1.5 GB');
	});

	it('treats zero and negatives as zero bytes', () => {
		expect(formatBytes(0)).toBe('0 B');
		expect(formatBytes(-1)).toBe('0 B');
	});
});

describe('mbToGB', () => {
	it('converts MiB to GiB rounded to one decimal', () => {
		expect(mbToGB(1536)).toBe(1.5);
		expect(mbToGB(80000)).toBe(78.1);
	});
});

describe('escapeHtml', () => {
	it('neutralises markup so it cannot be injected via {@html}', () => {
		expect(escapeHtml('<img src=x onerror="alert(1)">')).toBe(
			'&lt;img src=x onerror=&quot;alert(1)&quot;&gt;',
		);
	});

	it('escapes ampersands and quotes', () => {
		expect(escapeHtml(`Tom & Jerry's "disk"`)).toBe(
			'Tom &amp; Jerry&#39;s &quot;disk&quot;',
		);
	});

	it('leaves plain text unchanged', () => {
		expect(escapeHtml('Samsung SSD 990 PRO')).toBe('Samsung SSD 990 PRO');
	});
});

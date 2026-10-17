import { describe, it, expect } from 'vitest';
import { escapeHtml, groupDigits } from './format';

describe('groupDigits', () => {
	it('groups thousands with spaces', () => {
		expect(groupDigits(487875)).toBe('487 875');
	});

	it('leaves short numbers unchanged', () => {
		expect(groupDigits(128)).toBe('128');
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

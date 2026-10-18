import { describe, it, expect } from 'vitest';
import { pageFade } from './transitions';

// jsdom's getComputedStyle returns '' for opacity, which pageFade treats as 1.
function css(config: ReturnType<typeof pageFade>): (t: number) => string {
	return config.css as (t: number) => string;
}

describe('pageFade', () => {
	it('fades from transparent to opaque', () => {
		const config = pageFade(document.createElement('div'), { duration: 300 });
		expect(config.duration).toBe(300);

		const style = css(config);
		expect(style(0)).toContain('opacity: 0');
		expect(style(1)).toContain('opacity: 1');
	});

	it('animates opacity only (no filter/transform, which WebKitGTK cannot ease)', () => {
		const style = css(pageFade(document.createElement('div')));

		expect(style(0)).not.toContain('blur');
		expect(style(0)).not.toContain('filter');
		expect(style(0)).not.toContain('translate');
	});

	it('forwards the delay used to fade the pages through', () => {
		const config = pageFade(document.createElement('div'), { delay: 140 });

		expect(config.delay).toBe(140);
	});

	it('stays fully opaque when fade is 1', () => {
		const style = css(pageFade(document.createElement('div'), { fade: 1 }));

		expect(style(0)).toContain('opacity: 1');
		expect(style(1)).toContain('opacity: 1');
	});

	it('applies a stacking order while transitioning', () => {
		const node = document.createElement('div');
		const config = pageFade(node, { zIndex: 2 });

		config.tick?.(1, 0);

		expect(node.style.zIndex).toBe('2');
	});

	it('marks an exiting page so its scrollbars can be hidden', () => {
		const node = document.createElement('div');
		const config = pageFade(node, { exiting: true });

		config.tick?.(1, 0);

		expect(node.hasAttribute('data-exiting')).toBe(true);
	});
});

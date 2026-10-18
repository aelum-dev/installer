import { describe, it, expect } from 'vitest';
import { pageSlide } from './transitions';

// jsdom's getComputedStyle returns '' for opacity, which pageSlide treats as 1.
function css(config: ReturnType<typeof pageSlide>): (t: number) => string {
	return config.css as (t: number) => string;
}

describe('pageSlide', () => {
	it('starts offset and transparent and ends in place and opaque', () => {
		const config = pageSlide(document.createElement('div'), {
			distance: 50,
			duration: 300,
		});
		expect(config.duration).toBe(300);

		const style = css(config);
		expect(style(0)).toContain('translateX(50px)');
		expect(style(0)).toContain('opacity: 0');

		expect(style(1)).toContain('translateX(0px)');
		expect(style(1)).toContain('opacity: 1');
	});

	it('stays fully opaque when fade is 1 (the incoming page)', () => {
		const style = css(pageSlide(document.createElement('div'), { distance: 50, fade: 1 }));

		expect(style(0)).toContain('translateX(50px)');
		expect(style(0)).toContain('opacity: 1');
		expect(style(1)).toContain('opacity: 1');
	});

	it('takes a negative distance for the outgoing direction', () => {
		const style = css(pageSlide(document.createElement('div'), { distance: -40 }));

		expect(style(0)).toContain('translateX(-40px)');
		expect(style(1)).toContain('translateX(0px)');
	});

	it('applies a stacking order while transitioning', () => {
		const node = document.createElement('div');
		const config = pageSlide(node, { zIndex: 2 });

		config.tick?.(1, 0);

		expect(node.style.zIndex).toBe('2');
	});

	it('marks an exiting page so its scrollbars can be hidden', () => {
		const node = document.createElement('div');
		const config = pageSlide(node, { exiting: true });

		config.tick?.(1, 0);

		expect(node.hasAttribute('data-exiting')).toBe(true);
	});
});

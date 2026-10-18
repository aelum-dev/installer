<script lang="ts">
	import { glyph } from '$lib/icons';
	import './Chevron.css';

	// A single Nerd Fonts glyph, maintained in the installer's self-hosted subset.
	// The base glyph (eab7) is an up-chevron; every direction is that glyph rotated
	// (rotate(180deg) swings the vertex to the bottom → points down), so there is
	// one shape to keep in the woff2 instead of one per arrow. The caller supplies
	// its own `class` so each site can colour it (the dropdown sets its own; the nav
	// buttons set color on the button). Color inherits from the caller.
	let {
		direction,
		size = 16,
		class: className = '',
	}: {
		direction: 'up' | 'down' | 'left' | 'right';
		size?: number;
		class?: string;
	} = $props();
</script>

<span
	class={['chevron', `chevron--${direction}`, className]}
	style="font-size: {size}px;"
	aria-hidden="true"
>{glyph('eab7')}</span>

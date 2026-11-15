<script lang="ts">
	/**
	 * The one renderer for every disk picture in the wizard (device rows, the
	 * destination preview, the manual-mode disk map). Sizes, kinds and tooltips
	 * come from `lib/disk-layout.ts`; this component only draws them, so the
	 * strip language — monochrome segments, the free-space track, the LUKS
	 * hatch, segment edges — lives exactly once.
	 *
	 * `tips` is the per-host switch on the segment hints: an empty text is the
	 * tooltip action's own "no tooltip" (see lib/tooltip.svelte), so turning
	 * them off costs no second markup path.
	 */
	import type { StripSegment } from '$lib/disk-layout';
	import { tooltip } from '$lib/tooltip.svelte';
	import './LayoutStrip.css';

	let {
		segments,
		tall = false,
		tag = '',
		caption = '',
		tips = true,
	}: {
		segments: StripSegment[];
		/** The manual-mode map, thick enough to label its segments. */
		tall?: boolean;
		/** Uppercase tag before the caption text (e.g. "Layout"). */
		tag?: string;
		/** The plain-words chain under the strip; the strip alone renders without it. */
		caption?: string;
		/** Offer each segment's own tooltip. Off where the host already carries one. */
		tips?: boolean;
	} = $props();

	// Tiny partitions have a 3px visual floor in CSS, so their widths cannot be
	// trusted to sum to 100%; let flex distribute the remainder to the largest
	// segment by leaving the widths at their honest shares.
	function widthOf(seg: StripSegment): string {
		return `${Math.max(seg.pct, 0)}%`;
	}

	// A label narrower than five shares of the bar would render as a clipped
	// stroke; below that share the tooltip carries the name instead.
	const LABEL_MIN_PCT = 5;
</script>

<div class={['strip', { 'strip--tall': tall }]} aria-hidden="true">
	{#each segments as seg (seg.key)}
		{#if tall}
			<span
				class={[
					'seg',
					'map-seg',
					{ 'map-seg--role': seg.role, 'map-seg--free': seg.free, 'seg--luks': seg.hatch },
				]}
				style:width={widthOf(seg)}
				use:tooltip={tips ? seg.tooltip : ''}
			>{seg.pct >= LABEL_MIN_PCT ? seg.label : ''}</span>
		{:else}
			<span
				class={['seg', { 'seg--free': seg.free, 'seg--luks': seg.hatch }]}
				style:width={widthOf(seg)}
				use:tooltip={tips ? seg.tooltip : ''}
			></span>
		{/if}
	{/each}
</div>

{#if caption}
	<div class="strip-caption">
		{#if tag}<span class="plan-tag">{tag}</span>{/if}
		<span>{caption}</span>
	</div>
{/if}

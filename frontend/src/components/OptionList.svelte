<script lang="ts">
	/**
	 * A vertical list of selectable rows — the wizard's list look.
	 *
	 * The Welcome page's language list and the location picker both render this, so
	 * the row design (selected fill, hover, height, RTL-safe padding) lives in one
	 * place instead of being copied per page. Callers pass anything with a `code`
	 * and a `name`; the component owns no state, so filtering and persistence stay
	 * with the page that needs them.
	 */
	let {
		items,
		selected,
		onSelect,
		label,
	}: {
		items: { code: string; name: string }[];
		selected: string;
		onSelect: (code: string) => void;
		/** Accessible name for the list ("Languages", "Locations", …). */
		label: string;
	} = $props();
</script>

<ul class="option-list" aria-label={label}>
	{#each items as item (item.code)}
		<li>
			<button
				type="button"
				class={['option-item', { 'option-item--selected': item.code === selected }]}
				onclick={() => onSelect(item.code)}
			>
				{item.name}
			</button>
		</li>
	{/each}
</ul>

<style>
	.option-list {
		list-style: none;
		display: flex;
		flex-direction: column;
		gap: var(--spacing-xs);
		width: 100%;
		padding: 0;
		margin: 0;
	}

	.option-item {
		width: 100%;
		padding: 10px 20px 10px 24px;
		border: none;
		border-radius: var(--radius-xl);
		background: transparent;
		font-size: var(--font-size-base);
		/* Arabic/Hebrew glyphs already resolve to the compact Noto variants via
		   the base --font-family stack in style.css (fixed app-wide), so no
		   per-font pin is needed here. line-height/min-height set the row height;
		   display:flex + align-items:center center the text. */
		line-height: 1.3;
		min-height: 2.35em;
		display: flex;
		align-items: center;
		color: var(--color-text);
		text-align: left;
		cursor: pointer;
		transition: background-color 0.15s ease;
	}

	.option-item:not(.option-item--selected):hover {
		background-color: var(--color-primary-light);
	}

	.option-item--selected {
		background-color: var(--color-primary);
		color: var(--color-on-primary);
	}
</style>

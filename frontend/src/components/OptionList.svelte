<script lang="ts">
	/**
	 * A vertical list of selectable rows — the wizard's list look.
	 *
	 * The Welcome page renders this for all three of its lists (languages,
	 * countries, clocks), so the row design (selected fill, hover, height, RTL-safe
	 * padding) lives in one place instead of being copied per mode. Callers pass
	 * anything with a `code` and a `name`, plus the optional trailing text each mode
	 * shows: a `sub` (a locale, a mirror answer, a zone name) and a `chip` (a UTC
	 * offset). The component owns no state, so filtering and persistence stay with
	 * the page that needs them.
	 *
	 * Each row carries its `code` in a data attribute: the code is the row's
	 * identity, and it is the one thing about a row that no translation, no
	 * ellipsis and no added trailing text can change — which is what the e2e suite
	 * selects on.
	 */
	let {
		items,
		selected,
		onSelect,
		label,
	}: {
		items: OptionItem[];
		selected: string;
		onSelect: (code: string) => void;
		/** Accessible name for the list ("Languages", "Locations", …). */
		label: string;
	} = $props();

	interface OptionItem {
		code: string;
		/** The row's primary text — what the row is a name of. */
		name: string;
		/** Quiet text at the trailing edge, beside the mark. */
		sub?: string;
		/** Renders `sub` in the mono face: it is a code, not a sentence. */
		subMono?: boolean;
		/** A pill after `sub`, for a fact that should read as a fact. */
		chip?: string;
	}
</script>

<ul class="option-list" aria-label={label}>
	{#each items as item (item.code)}
		<li>
			<button
				type="button"
				class={['option-item', { 'option-item--selected': item.code === selected }]}
				data-code={item.code}
				onclick={() => onSelect(item.code)}
			>
				<span class="option-main">{item.name}</span>
				{#if item.sub}
					<span class={['option-sub', { 'option-sub--mono': item.subMono }]}>{item.sub}</span>
				{/if}
				{#if item.chip}
					<span class="option-chip">{item.chip}</span>
				{/if}
				<span class="option-check" aria-hidden="true">
					<svg viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
						<path d="M3.5 8.5L6.5 11.5L12.5 4.5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
					</svg>
				</span>
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
		gap: var(--spacing-md);
		color: var(--color-text);
		text-align: left;
		cursor: pointer;
		transition: background-color 0.15s ease;
	}

	.option-item:not(.option-item--selected):hover {
		background-color: var(--color-primary-light);
	}

	/* Selection is a container tint plus a mark, never the primary fill: solid
	   primary belongs to the control that advances the wizard, and a row wearing it
	   reads as the page's main action. The mark sits at the trailing edge so it
	   stays on the reading side in RTL. */
	.option-item--selected {
		background-color: var(--color-selection);
		color: var(--color-on-selection);
	}

	/* The name is the row's one flexible part: it takes what is left and ellipsises,
	   so a long country name never pushes the trailing facts off the window. */
	.option-main {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	/* Trailing facts sit at the far edge, before the mark. Only the first of them
	   needs the auto margin — once the edge is claimed the rest just pack against
	   it, which keeps them on the same side when a row has both. */
	.option-sub {
		margin-inline-start: auto;
		flex-shrink: 0;
		font-size: var(--font-size-sm);
		color: var(--color-text-light);
		white-space: nowrap;
	}

	.option-sub--mono {
		font-family: var(--font-mono);
	}

	.option-chip {
		flex-shrink: 0;
		font-size: var(--font-size-xs);
		font-weight: var(--font-weight-600);
		color: var(--color-text-muted);
		background: var(--color-bg-neutral-hover);
		border-radius: var(--radius-pill);
		padding: 3px 10px;
	}

	/* A trailing fact is the same colour as the row's own text once the row is
	   selected, dimmed rather than recoloured: the tint already says "this one". */
	.option-item--selected .option-sub,
	.option-item--selected .option-chip {
		color: inherit;
		opacity: 0.75;
	}

	.option-check {
		display: none;
		flex-shrink: 0;
		color: var(--color-primary);
	}

	.option-item--selected .option-check {
		display: inline-flex;
	}

	.option-check svg {
		width: 1.1em;
		height: 1.1em;
	}
</style>

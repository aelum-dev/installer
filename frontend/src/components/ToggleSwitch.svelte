<script lang="ts">
	/**
	 * Compact on/off switch (LUKS). Styled to match the manual-mode
	 * "Use remaining" toggle. Accessible as an ARIA switch.
	 */
	let {
		checked,
		label,
		title = '',
		onToggle,
	}: {
		checked: boolean;
		label: string;
		title?: string;
		onToggle: () => void;
	} = $props();
</script>

<button
	type="button"
	class={['toggle-switch', { on: checked }]}
	role="switch"
	aria-checked={checked}
	aria-label={label}
	title={title}
	onclick={onToggle}
>
	<span class="toggle-slider"></span>
</button>

<style>
	.toggle-switch {
		position: relative;
		display: inline-block;
		width: 38px;
		height: 20px;
		border-radius: 999px;
		background: var(--color-border-light);
		border: 0;
		cursor: pointer;
		padding: 0;
		transition: background 0.2s ease;
		flex-shrink: 0;
	}

	.toggle-switch:focus-visible {
		outline: 2px solid var(--color-focus);
		outline-offset: 2px;
	}

	.toggle-switch.on {
		background: var(--color-primary);
	}

	.toggle-slider {
		position: absolute;
		top: 2px;
		left: 2px;
		width: 16px;
		height: 16px;
		border-radius: 50%;
		background: #fff;
		transition: transform 0.2s cubic-bezier(0.4, 0, 0.2, 1);
		box-shadow: 0 1px 2px color-mix(in srgb, var(--md-sys-color-shadow) 30%, transparent);
	}

	.toggle-switch.on .toggle-slider {
		transform: translateX(18px);
	}
</style>

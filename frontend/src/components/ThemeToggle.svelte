<script lang="ts">
	import { toggleTheme, getTheme } from '$lib/theme.svelte';
	import { t } from '$lib/i18n';

	// Reflect the active scheme on the control. The visible icon names the current
	// theme; the aria-label / title name the action the next click performs.
	let isDark = $derived(getTheme() === 'dark');

	const label = $derived(isDark ? t('app.themeToLight') : t('app.themeToDark'));
</script>

<button
	type="button"
	class="theme-toggle"
	aria-pressed={isDark}
	aria-label={label}
	title={label}
	onclick={toggleTheme}
>
	{#if isDark}
		<!-- Sun — clicking switches to light mode -->
		<svg
			class="theme-icon"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			stroke-width="2"
			stroke-linecap="round"
			stroke-linejoin="round"
			aria-hidden="true"
		>
			<circle cx="12" cy="12" r="4" />
			<path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" />
		</svg>
	{:else}
		<!-- Moon — clicking switches to dark mode -->
		<svg
			class="theme-icon"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			stroke-width="2"
			stroke-linecap="round"
			stroke-linejoin="round"
			aria-hidden="true"
		>
			<path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
		</svg>
	{/if}
</button>

<style>
	.theme-toggle {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 36px;
		height: 36px;
		padding: 0;
		border-radius: var(--radius-pill);
		border: 1px solid var(--color-border);
		background: var(--color-surface);
		color: var(--color-text-muted);
		cursor: pointer;
		transition:
			transform 0.3s var(--theme-ease),
			background 0.3s,
			color 0.3s,
			border-color 0.3s;
	}

	.theme-toggle:hover {
		transform: translateY(-2px);
		background: var(--color-bg-neutral-hover);
		color: var(--color-text);
	}

	.theme-toggle:focus-visible {
		outline: 2px solid var(--color-focus);
		outline-offset: 2px;
	}

	.theme-icon {
		width: 18px;
		height: 18px;
		line-height: 0;
	}
</style>

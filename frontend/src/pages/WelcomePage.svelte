<script lang="ts">
	import {
		GetLanguages,
		GetRegions,
		GetSelectedLanguage,
		GetSelectedRegion,
		SetSelectedLanguage,
		SetSelectedRegion,
	} from '$lib/backend';
	import { installState } from '$lib/store.svelte';
	import { t, setLocale, backendLocaleToFrontend } from '$lib/i18n';
	import PageLayout from '$components/PageLayout.svelte';
	import ScrollBox from '$components/ScrollBox.svelte';
	import OptionList from '$components/OptionList.svelte';
	import RegionPicker from '$components/RegionPicker.svelte';
	import { glyph, STEP_ICONS } from '$lib/icons';
	import { PAGE_IDS } from '$lib/types';
	import type { LanguageOption, RegionOption } from '$lib/types';

	const pageIcon = glyph(STEP_ICONS.welcome);

	let languages = $state<LanguageOption[]>([]);
	let regions = $state<RegionOption[]>([]);
	let loading = $state(true);
	let showRegionPicker = $state(false);

	const selectedRegion = $derived(regions.find((r) => r.code === installState.region));
	const selectedRegionName = $derived(selectedRegion?.name || '—');

	// Apply the UI translation that belongs to a backend language code.
	// Used for both the restored selection and a user click, so the UI language
	// always matches the selected install language.
	function applyUiLocale(code: string): void {
		const lang = languages.find((l) => l.code === code);
		const uiLocale = lang ? backendLocaleToFrontend(lang.locale) : undefined;
		if (uiLocale) setLocale(uiLocale);
	}

	async function loadChoices(): Promise<void> {
		try {
			[languages, regions] = await Promise.all([GetLanguages(), GetRegions()]);

			const saved = await GetSelectedLanguage();
			if (saved && languages.some((l) => l.code === saved)) {
				installState.locale = saved;
				applyUiLocale(saved);
			} else if (languages.length > 0) {
				installState.locale = languages[0].code;
				applyUiLocale(installState.locale);
				await SetSelectedLanguage(installState.locale);
			}

			// The location is whatever the backend holds: selecting a language
			// pre-fills its country, so nobody chooses their country twice.
			installState.region = await GetSelectedRegion();
		} catch (err) {
			console.error(`Failed to load languages: ${err}`);
		} finally {
			loading = false;
		}
	}

	async function selectLanguage(code: string): Promise<void> {
		installState.locale = code;
		applyUiLocale(code);

		try {
			await SetSelectedLanguage(code);
			// A language re-prefills the location only until the person has said
			// where they are; an explicit choice outlives a language change.
			if (installState.regionChosen && installState.region) {
				await SetSelectedRegion(installState.region);
			}
			installState.region = await GetSelectedRegion();
		} catch (err) {
			console.error(`Failed to set language: ${err}`);
		}
	}

	async function selectRegion(code: string): Promise<void> {
		try {
			await SetSelectedRegion(code);
			installState.region = code;
			installState.regionChosen = true;
		} catch (err) {
			console.error(`Failed to set region: ${err}`);
		}
	}

	loadChoices();
</script>

<PageLayout
	icon={pageIcon}
	title={t('page.welcome.title')}
	subtitle={t('page.welcome.subtitle')}
	variant="list"
	wideInset
>
	{#snippet info()}
		<div class="region-block">
			<div class="region-header">{t('page.welcome.region')}</div>
			<button type="button" class="region-select" onclick={() => (showRegionPicker = true)}>
				<span>{selectedRegionName}</span>
				<span class="region-caret" aria-hidden="true">
					<svg viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
						<path d="M6 3.5L10.5 8L6 12.5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
					</svg>
				</span>
			</button>
			<div class="region-hint">{t('page.welcome.regionHint')}</div>
		</div>
	{/snippet}

	{#if loading}
		<span class="loading-text">{t('page.welcome.loading')}</span>
	{:else}
		<ScrollBox persistKey={PAGE_IDS.WELCOME}>
			<OptionList
				items={languages}
				selected={installState.locale}
				onSelect={selectLanguage}
				label={t('page.welcome.title')}
			/>
		</ScrollBox>
	{/if}
</PageLayout>

<RegionPicker
	bind:show={showRegionPicker}
	{regions}
	selected={installState.region}
	onSelect={selectRegion}
/>

<style>
	.loading-text {
		font-size: var(--font-size-md-lg);
		color: var(--color-text-light);
		margin-top: 16px;
	}

	/* The location block is the info rail's one control: the header speaks in the
	   rail's quiet voice, and the button says plainly what it changes. Language is
	   the list on the right; this is the only place that says the clock, keyboard
	   and mirrors come from somewhere else. */
	.region-block {
		display: flex;
		flex-direction: column;
		gap: var(--spacing-xs);
		width: fit-content;
		margin-inline: auto;
	}

	.region-header {
		font-size: var(--font-size-sm);
		font-weight: var(--font-weight-600);
		color: var(--color-text-muted);
		letter-spacing: 0.02em;
		margin-bottom: var(--spacing-xs);
	}

	.region-select {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--spacing-md);
		min-width: 15rem;
		padding: 9px 14px 9px 18px;
		border: 1.5px solid var(--color-border);
		border-radius: var(--radius-xl);
		background: transparent;
		font-family: inherit;
		font-size: var(--font-size-base);
		color: var(--color-text);
		text-align: start;
		cursor: pointer;
		transition: background-color 0.15s ease, border-color 0.15s ease;
	}

	.region-select:hover {
		background-color: var(--color-primary-light);
		border-color: var(--color-primary);
	}

	.region-caret {
		flex-shrink: 0;
		color: var(--color-text-light);
		display: inline-flex;
	}

	.region-caret svg {
		width: 1em;
		height: 1em;
	}

	.region-hint {
		font-size: var(--font-size-sm);
		color: var(--color-text-muted);
		line-height: 1.4;
		text-align: left;
	}
</style>

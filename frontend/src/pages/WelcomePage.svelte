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
	let filter = $state('');

	const selectedRegion = $derived(regions.find((r) => r.code === installState.region));
	const selectedRegionName = $derived(selectedRegion?.name || '—');

	// 28 names in twenty scripts: reading them is the intended path, but someone
	// who cannot read the script they are after has to be able to type it. The
	// location picker has had a filter from the start for the same reason; the
	// language list is the choice the whole rest of the installer is written in,
	// and it was the one you could only scroll. The table's own order is kept —
	// it is deliberate (English first, then the additions after it), not sorted.
	const visibleLanguages = $derived.by(() => {
		const q = filter.trim().toLowerCase();
		if (q === '') return languages;
		return languages.filter(
			(l) => l.name.toLowerCase().includes(q) || l.code.toLowerCase().startsWith(q),
		);
	});

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
		<!-- The location is the one control on this page that decides the clock, the
		     keyboard, the mirrors and the weather, so it is a card that says what it
		     holds rather than a bare pill. It opens the picker, it does not advance
		     the wizard, so it discloses downward — the footer owns the only forward
		     chevron. -->
		<div class="region-card">
			<div class="field-label">{t('page.welcome.region')}</div>
			<div class="region-row">
				<span class="region-glyph" aria-hidden="true">
					<svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
						<path d="M12 21s-6.5-6.1-6.5-10.4A6.5 6.5 0 0 1 12 4a6.5 6.5 0 0 1 6.5 6.6C18.5 14.9 12 21 12 21z" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
						<circle cx="12" cy="10.4" r="2.3" stroke="currentColor" stroke-width="1.8"/>
					</svg>
				</span>
				<span class="region-place">
					<span class="region-value">{selectedRegionName}</span>
					{#if selectedRegion?.timezone}
						<span class="region-zone">{selectedRegion.timezone}</span>
					{/if}
				</span>
				<button type="button" class="region-change" onclick={() => (showRegionPicker = true)}>
					{t('page.welcome.regionChange')}
					<svg viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
						<path d="M3.5 6L8 10.5L12.5 6" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
					</svg>
				</button>
			</div>
			<!-- The pre-fill is the part a person can't see happening: picking a
			     language quietly rewrites this. Say so — and stop saying it once they
			     have chosen a location themselves, when it is no longer true. -->
			{#if !installState.regionChosen}
				<p class="region-note">{t('page.welcome.regionHint')}</p>
			{/if}
		</div>
	{/snippet}

	{#if loading}
		<span class="loading-text">{t('page.welcome.loading')}</span>
	{:else}
		<label class="field-label list-label" for="language-filter">{t('page.welcome.language')}</label>
		<input
			id="language-filter"
			type="text"
			class="line-edit list-filter"
			placeholder={t('page.welcome.filter')}
			bind:value={filter}
		/>
		<ScrollBox persistKey={PAGE_IDS.WELCOME}>
			{#if visibleLanguages.length > 0}
				<OptionList
					items={visibleLanguages}
					selected={installState.locale}
					onSelect={selectLanguage}
					label={t('page.welcome.language')}
				/>
			{:else}
				<p class="list-empty">{t('page.welcome.noMatch')}</p>
			{/if}
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

	/* Small uppercase label above a control — the location card's header and the
	   language list's label are the same kind of thing, so they are one style. */
	.field-label {
		font-size: var(--font-size-sm);
		font-weight: var(--font-weight-600);
		color: var(--color-text-muted);
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}

	/* Opens itself from the subtitle block above it (the rail's own gap is the
	   paragraph rhythm) and takes the rail's width: a control this consequential
	   is not a pill floating in the middle of an empty column. */
	.region-card {
		margin-top: var(--spacing-lg);
		display: flex;
		flex-direction: column;
		gap: var(--spacing-xs);
		background-color: var(--color-bg-white);
		border: 1px solid var(--color-border-light);
		border-radius: var(--radius-lg);
		box-shadow: var(--shadow-md);
		padding: var(--spacing-md) var(--spacing-lg);
		text-align: start;
	}

	.region-row {
		display: flex;
		align-items: center;
		gap: var(--spacing-md);
	}

	.region-glyph {
		flex-shrink: 0;
		display: inline-flex;
		color: var(--color-primary);
	}

	.region-glyph svg {
		width: 1.15em;
		height: 1.15em;
	}

	/* The place is one block: the name on top, its clock underneath. Side by side
	   they wrapped ("United / States") in a narrow window; a name of a country is
	   one line, so it gets one line and ellipsises instead of breaking. */
	.region-place {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.region-value {
		font-size: var(--font-size-base-lg);
		font-weight: var(--font-weight-600);
		color: var(--color-text);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	/* The zone is the part of the answer that is not obvious from a country name —
	   it is what the installed system's clock will say, so it is shown, not stored. */
	.region-zone {
		font-size: var(--font-size-sm);
		color: var(--color-text-light);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.region-change {
		display: inline-flex;
		align-items: center;
		gap: var(--spacing-xs);
		flex-shrink: 0;
		font-family: inherit;
		font-size: var(--font-size-md);
		font-weight: var(--font-weight-600);
		color: var(--color-primary);
		background: transparent;
		border: none;
		padding: 6px 10px;
		border-radius: var(--radius-pill);
		cursor: pointer;
		transition: background-color 0.15s ease;
	}

	.region-change:hover {
		background-color: var(--color-primary-light);
	}

	.region-change svg {
		width: 1em;
		height: 1em;
	}

	.region-note {
		margin: var(--spacing-xs) 0 0;
		font-size: var(--font-size-sm);
		color: var(--color-text-light);
	}

	.list-label {
		display: block;
		margin-bottom: var(--spacing-sm);
	}

	/* The filter sits outside the ScrollBox on purpose: it is the way into the list,
	   and it would scroll away if it were inside it. */
	.list-filter {
		width: 100%;
		box-sizing: border-box;
		margin-bottom: var(--spacing-md);
	}

	.list-empty {
		margin: var(--spacing-sm) 0;
		font-size: var(--font-size-md);
		color: var(--color-text-light);
		text-align: start;
	}
</style>

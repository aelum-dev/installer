<script lang="ts">
	import {
		GetLanguages,
		GetRegions,
		GetSelectedLanguage,
		GetSelectedRegion,
		GetSelectedTimezone,
		GetTimezones,
		SetSelectedLanguage,
		SetSelectedRegion,
		SetSelectedTimezone,
	} from '$lib/backend';
	import { installState } from '$lib/store.svelte';
	import { DefaultLanguageCode } from '$lib/constants';
	import { t, setLocale, backendLocaleToFrontend } from '$lib/i18n';
	import PageLayout from '$components/PageLayout.svelte';
	import ScrollBox from '$components/ScrollBox.svelte';
	import OptionList from '$components/OptionList.svelte';
	import { glyph, STEP_ICONS } from '$lib/icons';
	import { PAGE_IDS } from '$lib/types';
	import type { LanguageOption, RegionOption, TimezoneOption } from '$lib/types';
	import {
		filterLanguages,
		filterRegions,
		filterTimezones,
		findRegion,
		groupTimezones,
		selectLanguage,
		selectRegion,
		selectTimezone,
		utcOffset,
		zoneCity,
		zonesForRegion,
		type WelcomeSelection,
		type WelcomeTab,
	} from '$lib/welcome-cascade';

	const pageIcon = glyph(STEP_ICONS.welcome);

	let languages = $state<LanguageOption[]>([]);
	let regions = $state<RegionOption[]>([]);
	let zones = $state<TimezoneOption[]>([]);
	let loading = $state(true);

	// Which list the content column shows. It is a view state, not an answer: the
	// three answers are always all in play, and the summary card shows all three.
	let tab = $state<WelcomeTab>('language');
	// The row the visible list should lead with. It follows the tab, not the
	// clicks: picking a row must not yank the list around under the pointer, but
	// switching which list is on screen opens it on that list's answer.
	let scrollTarget = $state('');
	// Each list keeps its own filter text, so switching away from a filtered list
	// and back does not silently drop what was typed.
	let queries = $state<Record<WelcomeTab, string>>({ language: '', location: '', timezone: '' });
	// The clock list starts at the country's own clocks; the whole tzdb set is one
	// press away. A typed query overrides it — searching always searches everything.
	let tzOnlyCountry = $state(true);

	let searchEl = $state<HTMLInputElement | null>(null);

	const selectedLanguage = $derived(languages.find((l) => l.code === installState.locale));
	const selectedRegion = $derived(findRegion(regions, installState.region));

	// The summary card: the three answers, always visible, and the control that
	// switches which list the content column edits. Each row says what it holds and
	// what that answer brings with it, so the page can be read without opening
	// anything.
	const summaryRows = $derived.by(() => {
		const zone = installState.timezone;
		return [
			{
				tab: 'language' as const,
				key: 'page.welcome.language',
				value: selectedLanguage?.name ?? '',
				sub: selectedLanguage?.locale ?? '',
			},
			{
				tab: 'location' as const,
				key: 'page.welcome.region',
				value: selectedRegion?.name ?? '',
				// The two things a country decides that are not obvious from its name:
				// which keyboards it sells and whether the install comes from there.
				sub: selectedRegion
					? `${t('page.welcome.keyboard')} ${selectedRegion.xkb} · ` +
						(selectedRegion.mirrors ? t('page.welcome.mirrorsLocal') : t('page.welcome.mirrorsWorld'))
					: '',
			},
			{
				tab: 'timezone' as const,
				key: 'page.welcome.timezone',
				value: zone === '' ? '' : zoneCity(zone),
				sub: zone === '' ? '' : `${zone} · ${utcOffset(zone)}`,
			},
		];
	});

	// The i18n prefix for the current tab. The tab is a view id ("location"); the
	// key family for that answer is the page's long-standing `region` one
	// (page.welcome.region and its Placeholder/Empty siblings), so the tab id and
	// the key prefix are deliberately not the same word.
	const tabKey = $derived(tab === 'location' ? 'region' : tab);
	const tabLabel = $derived(t(summaryRows.find((r) => r.tab === tab)?.key ?? ''));
	const tabPlaceholder = $derived(t(`page.welcome.${tabKey}Placeholder`));
	const tabEmpty = $derived(t(`page.welcome.${tabKey}Empty`));
	const selectedValue = $derived(
		tab === 'language' ? installState.locale : tab === 'location' ? installState.region : installState.timezone,
	);

	const query = $derived(queries[tab]);

	// The clock list's two views. A query always searches every clock, so the
	// country-only view is only ever the unfiltered starting point.
	const tzCountryOnly = $derived(query.trim() === '' && tzOnlyCountry);

	const visibleZones = $derived(
		tab !== 'timezone'
			? []
			: tzCountryOnly
				? zonesForRegion(zones, selectedRegion)
				: filterTimezones(zones, query),
	);

	const listItems = $derived.by(() => {
		if (tab === 'language') {
			return filterLanguages(languages, query).map((l) => ({
				code: l.code,
				name: l.name,
				sub: l.locale,
				subMono: true,
			}));
		}
		if (tab === 'location') {
			return filterRegions(regions, query).map((r) => ({
				code: r.code,
				name: r.name,
				sub: r.mirrors ? t('page.welcome.mirrorsLocal') : t('page.welcome.mirrorsWorld'),
			}));
		}
		return visibleZones.map(zoneRow);
	});

	// The full set is grouped by continent so 320 clocks can be scanned, not only
	// searched. A country's own list is short enough to read straight through.
	const zoneGroups = $derived(tab === 'timezone' && !tzCountryOnly ? groupTimezones(visibleZones) : []);

	function zoneRow(z: TimezoneOption) {
		return { code: z.id, name: zoneCity(z.id), sub: z.id, subMono: true, chip: utcOffset(z.id) };
	}

	// The three answers travel to the backend in cascade order: the language
	// pre-fills the location there, and the location pre-fills the clock, so pushing
	// them in that order leaves the backend holding exactly what the person chose —
	// never a suggestion that overwrote a decision on the way.
	async function pushSelection(): Promise<void> {
		await SetSelectedLanguage(installState.locale);
		await SetSelectedRegion(installState.region);
		await SetSelectedTimezone(installState.timezone);
	}

	function applySelection(next: WelcomeSelection): void {
		installState.locale = next.locale;
		installState.region = next.region;
		installState.timezone = next.timezone;
		installState.regionChosen = next.regionChosen;
		installState.timezoneChosen = next.timezoneChosen;
	}

	// Apply the UI translation that belongs to a backend language code. Used for
	// both the restored selection and a user click, so the UI language always
	// matches the selected install language. A language with no UI translation keeps
	// the chrome as it is — the system it installs still reads that language.
	function applyUiLocale(code: string): void {
		const lang = languages.find((l) => l.code === code);
		const uiLocale = lang ? backendLocaleToFrontend(lang.locale) : undefined;
		if (uiLocale) setLocale(uiLocale);
	}

	async function loadChoices(): Promise<void> {
		try {
			[languages, regions, zones] = await Promise.all([GetLanguages(), GetRegions(), GetTimezones()]);

			const [savedLang, savedRegion, savedZone] = await Promise.all([
				GetSelectedLanguage(),
				GetSelectedRegion(),
				GetSelectedTimezone(),
			]);

			// The default is the backend's own fallback language, not the table's
			// first row — that row is merely the alphabetically first language.
			const fallback = languages.some((l) => l.code === DefaultLanguageCode)
				? DefaultLanguageCode
				: (languages[0]?.code ?? '');
			installState.locale = savedLang && languages.some((l) => l.code === savedLang) ? savedLang : fallback;
			scrollTarget = installState.locale;
			installState.region = findRegion(regions, savedRegion)?.code ?? '';
			installState.timezone = zones.some((z) => z.id === savedZone) ? savedZone : '';
			applyUiLocale(installState.locale);

			// A first visit adopts the backend's own pre-fill; a later one keeps what
			// the person already chose here. The pre-fill is the cascade itself — the
			// language's country and that country's clock — so the card holds all
			// three answers from the first paint, exactly as after a click.
			if (!savedLang && installState.locale) {
				applySelection(
					selectLanguage(
						{
							locale: installState.locale,
							region: '',
							timezone: '',
							regionChosen: false,
							timezoneChosen: false,
						},
						languages,
						regions,
						installState.locale,
					),
				);
				await pushSelection();
			}
		} catch (err) {
			console.error(`Failed to load the Welcome page's choices: ${err}`);
		} finally {
			loading = false;
		}
	}

	async function select(code: string): Promise<void> {
		const current: WelcomeSelection = {
			locale: installState.locale,
			region: installState.region,
			timezone: installState.timezone,
			regionChosen: installState.regionChosen,
			timezoneChosen: installState.timezoneChosen,
		};
		const next =
			tab === 'language'
				? selectLanguage(current, languages, regions, code)
				: tab === 'location'
					? selectRegion(current, regions, code)
					: selectTimezone(current, regions, zones, code);

		applySelection(next);
		if (tab === 'language') applyUiLocale(code);
		// A new location means new clocks; the view follows it, so the clock the
		// person is after is in the list in front of them.
		if (tab !== 'timezone') tzOnlyCountry = true;

		try {
			await pushSelection();
		} catch (err) {
			console.error(`Failed to record the choice: ${err}`);
		}
		// The list re-rendered and took the clicked row's focus with it; the search
		// field is where the next keystroke belongs.
		searchEl?.focus();
	}

	function pickTab(next: WelcomeTab): void {
		tab = next;
		// The new list opens on its own answer, brought to the top of the view.
		scrollTarget =
			next === 'language' ? installState.locale : next === 'location' ? installState.region : installState.timezone;
		searchEl?.focus();
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
		<!-- The three answers, and the only way to switch which list is being edited.
		     They are rows in one card rather than three buttons that each open a
		     dialog: the answers are the page's subject, so all three stay on screen
		     while one of them is being changed. -->
		<div class="summary" role="group" aria-label={t('page.welcome.summary')}>
			{#each summaryRows as row (row.tab)}
				<button
					type="button"
					class={['summary-row', { 'summary-row--active': tab === row.tab }]}
					aria-pressed={tab === row.tab}
					onclick={() => pickTab(row.tab)}
				>
					<span class="s-label">{t(row.key)}</span>
					<span class="s-value">{row.value}</span>
					{#if row.sub}<span class="s-sub">{row.sub}</span>{/if}
					<span class="s-check" aria-hidden="true">
						<svg viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
							<path d="M3.5 8.5L6.5 11.5L12.5 4.5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
						</svg>
					</span>
				</button>
			{/each}
		</div>
	{/snippet}

	{#if loading}
		<span class="loading-text">{t('page.welcome.loading')}</span>
	{:else}
		<label class="list-label" for="welcome-search">{tabLabel}</label>
		<input
			id="welcome-search"
			type="text"
			class="line-edit list-filter"
			placeholder={tabPlaceholder}
			autocomplete="off"
			spellcheck="false"
			bind:value={queries[tab]}
			bind:this={searchEl}
			onkeydown={(e) => {
				if (e.key === 'Escape') queries[tab] = '';
			}}
		/>

		{#if tab === 'timezone'}
			<!-- The clock list's two scopes. Both say how many clocks they hold, so
			     the difference is visible before anything is pressed. -->
			<div class="tz-toggle">
				<button
					type="button"
					class={['tz-pill', { 'tz-pill--active': tzCountryOnly }]}
					onclick={() => (tzOnlyCountry = true)}
				>
					{t('page.welcome.timezoneOnlyCountry', { x: selectedRegion?.name ?? '' })}
					· {zonesForRegion(zones, selectedRegion).length}
				</button>
				<button
					type="button"
					class={['tz-pill', { 'tz-pill--active': !tzOnlyCountry }]}
					onclick={() => (tzOnlyCountry = false)}
				>
					{t('page.welcome.timezoneAllZones')} · {zones.length}
				</button>
			</div>
		{/if}

		<ScrollBox persistKey={PAGE_IDS.WELCOME} scrollToCode={scrollTarget}>
			{#if listItems.length === 0}
				<p class="list-empty">{tabEmpty}</p>
			{:else if zoneGroups.length > 0}
				{#each zoneGroups as group (group.name)}
					<div class="group-head">{group.name}</div>
					<OptionList items={group.zones.map(zoneRow)} selected={selectedValue} onSelect={select} label={tabLabel} />
				{/each}
			{:else}
				<OptionList items={listItems} selected={selectedValue} onSelect={select} label={tabLabel} />
			{/if}
		</ScrollBox>
	{/if}
</PageLayout>

<style>
	.loading-text {
		font-size: var(--font-size-md-lg);
		color: var(--color-text-light);
		margin-top: 16px;
	}

	/* Small uppercase label above a control — the summary card's row labels and the
	   list's label are the same kind of thing, so they are one style. */
	.s-label,
	.list-label {
		font-size: var(--font-size-sm);
		font-weight: var(--font-weight-600);
		color: var(--color-text-muted);
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}

	.s-label {
		font-size: var(--font-size-xs);
	}

	/* The summary card takes the rail's width and wears the same chrome as every
	   other card in a rail: a light surface, a light border, a soft shadow.
	   Its margin is deliberately small: the rail centres this whole stack in a
	   fixed 900×600 window, and with all three rows filled the identity block,
	   the gaps and the card together are the rail's full height — a wider margin
	   here is what pushed the card's frame under the footer. */
	.summary {
		margin-top: var(--spacing-sm);
		width: 100%;
		box-sizing: border-box;
		text-align: start;
		display: flex;
		flex-direction: column;
		gap: 2px;
		background-color: var(--color-bg-white);
		border: 1px solid var(--color-border-light);
		border-radius: var(--radius-lg);
		box-shadow: var(--shadow-md);
		padding: var(--spacing-sm);
	}

	/* A row is the control that switches the list: it names an answer, shows its
	   value, and marks which one the content column is editing. The active one
	   carries a primary tint and a leading rule, so the current list is legible from
	   the rail alone. */
	.summary-row {
		display: grid;
		grid-template-columns: 1fr auto;
		grid-template-areas:
			'label check'
			'value check'
			'sub check';
		align-items: center;
		column-gap: var(--spacing-sm);
		text-align: start;
		font-family: inherit;
		background: transparent;
		border: none;
		cursor: pointer;
		border-radius: var(--radius-md);
		padding: 6px 10px;
		color: var(--color-text);
		transition: background-color 0.15s ease;
		border-inline-start: 2px solid transparent;
	}

	.summary-row:hover {
		background-color: var(--color-primary-lighter);
	}

	.summary-row--active {
		background-color: var(--color-primary-light);
		border-inline-start-color: var(--color-primary);
	}

	.s-label {
		grid-area: label;
	}

	/* A country name, a language name and a city are each one line, so they get one
	   line and ellipsise rather than wrapping — "United / States" reads as a bug. */
	.s-value {
		grid-area: value;
		font-size: var(--font-size-md-lg);
		font-weight: var(--font-weight-600);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		max-width: 210px;
	}

	.s-sub {
		grid-area: sub;
		font-size: var(--font-size-xs);
		color: var(--color-text-light);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		max-width: 210px;
	}

	.s-check {
		grid-area: check;
		color: var(--color-primary);
		display: none;
	}

	.summary-row--active .s-check {
		display: inline-flex;
	}

	.s-check svg {
		width: 15px;
		height: 15px;
	}

	/* The filter sits outside the ScrollBox on purpose: it is the way into the list,
	   and it would scroll away if it were inside it. */
	.list-filter {
		width: 100%;
		box-sizing: border-box;
	}

	/* The clock list's scope switch: quiet pills, the active one wearing the same
	   selection tint a selected row wears. */
	.tz-toggle {
		display: flex;
		gap: var(--spacing-xs);
		align-items: center;
	}

	.tz-pill {
		font-family: inherit;
		font-size: var(--font-size-sm);
		font-weight: var(--font-weight-600);
		padding: 5px 14px;
		border-radius: var(--radius-pill);
		cursor: pointer;
		border: 1px solid var(--color-border-light);
		background: transparent;
		color: var(--color-text-muted);
		transition:
			background-color 0.15s ease,
			color 0.15s ease,
			border-color 0.15s ease;
	}

	.tz-pill:hover {
		background-color: var(--color-primary-lighter);
	}

	.tz-pill--active {
		background-color: var(--color-selection);
		color: var(--color-on-selection);
		border-color: transparent;
	}

	/* A continent heading between two lists. It is a label, not a row, so it wears
	   the list's own indent without its height. */
	.group-head {
		font-size: var(--font-size-xs);
		font-weight: var(--font-weight-600);
		color: var(--color-text-light);
		letter-spacing: 0.06em;
		text-transform: uppercase;
		padding: 10px 20px 4px 24px;
	}

	.list-empty {
		margin: var(--spacing-sm) 0;
		font-size: var(--font-size-md);
		color: var(--color-text-light);
		text-align: start;
		padding-inline-start: 24px;
	}
</style>

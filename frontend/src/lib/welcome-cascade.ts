// The Welcome page's three answers and the rules that tie them together.
//
// Language, location and clock are three questions, and each one pre-fills the
// next: the language's country suggests the location, the location's country
// suggests the clock. A hand-picked answer outranks a suggestion below it, so the
// rules are about *when a change is allowed to move something else*, and that is
// exactly the part worth testing without a browser. The page owns the state; this
// module owns the decisions, and every function here is pure.
//
// The backend runs the same cascade for the CLI (gui.SetSelectedLanguage and its
// siblings). Both read the same generated tables — a language's territory, a
// region's default zone, a zone's countries all arrive on the bindings — so the
// two agree by construction rather than by keeping two lists in step.

import type { LanguageOption, RegionOption, TimezoneOption } from '$lib/types';

// Which list the content column is showing. The summary card's three rows are the
// control that switches it, so the tab is a view state, not an answer.
export type WelcomeTab = 'language' | 'location' | 'timezone';

// The three answers plus the two flags that say which of them the person chose by
// hand. A flag is what stops a suggestion from overwriting a decision.
export interface WelcomeSelection {
	locale: string;
	region: string;
	timezone: string;
	regionChosen: boolean;
	timezoneChosen: boolean;
}

export function findRegion(regions: RegionOption[], code: string): RegionOption | undefined {
	return regions.find((r) => r.code === code);
}

export function findZone(zones: TimezoneOption[], id: string): TimezoneOption | undefined {
	return zones.find((z) => z.id === id);
}

// zoneCity is the part of a zone name a person recognises — the city, with its
// underscores turned back into spaces. It is the row's primary text; the full ID
// sits beside it for anyone who needs the exact name. Mirrors shared.ZoneCity.
export function zoneCity(id: string): string {
	if (id === 'UTC') return 'UTC';
	const slash = id.lastIndexOf('/');
	const city = slash >= 0 ? id.slice(slash + 1) : id;
	return city.replaceAll('_', ' ');
}

// zoneGroup is the continent a zone is filed under when the whole set is listed —
// a 320-row list has to be scannable, not only searchable. UTC has no continent,
// and it is the one entry nobody has to look for, so it leads its own group.
export function zoneGroup(id: string): string {
	if (id === 'UTC') return 'UTC';
	const slash = id.indexOf('/');
	return slash >= 0 ? id.slice(0, slash) : 'Other';
}

const offsetCache = new Map<string, string>();

// utcOffset renders a zone's current offset as "UTC+02:00" — the one number that
// tells two cities on the same clock apart from two that merely share a name. The
// browser's own tzdata answers, which is the same database the installed system's
// tzdata is cut from, and the answer is cached because formatToParts is not cheap
// and a zone's offset is asked for once per list render.
export function utcOffset(id: string): string {
	const cached = offsetCache.get(id);
	if (cached !== undefined) return cached;

	let out = 'UTC';
	try {
		const parts = new Intl.DateTimeFormat('en-US', {
			timeZone: id,
			timeZoneName: 'longOffset',
		}).formatToParts(new Date());
		const name = parts.find((p) => p.type === 'timeZoneName')?.value ?? 'GMT';
		out = 'UTC' + (name === 'GMT' ? '+00:00' : name.slice(3));
	} catch {
		// A zone the browser's tzdata does not have still gets its name shown.
		out = 'UTC';
	}
	offsetCache.set(id, out);
	return out;
}

// ── The cascade ──────────────────────────────────────────────────────────────

// selectLanguage answers a language pick. The location follows the language's
// country only while the person has not named one themselves; the clock follows
// the location, and a clock the person named is dropped along with the location it
// came from — a new country means a new default clock.
export function selectLanguage(
	sel: WelcomeSelection,
	languages: LanguageOption[],
	regions: RegionOption[],
	code: string,
): WelcomeSelection {
	const next: WelcomeSelection = { ...sel, locale: code };
	if (next.regionChosen) return next;

	const territory = languages.find((l) => l.code === code)?.territory ?? '';
	const region = findRegion(regions, territory);
	if (!region) return next;

	next.region = region.code;
	next.timezone = region.timezone;
	next.timezoneChosen = false;
	return next;
}

// selectRegion answers a location pick. Choosing a location is a decision, so the
// flag goes up, and its clock becomes the answer again — the person changed which
// country's clock they want, which is the same act as changing the clock.
export function selectRegion(
	sel: WelcomeSelection,
	regions: RegionOption[],
	code: string,
): WelcomeSelection {
	const region = findRegion(regions, code);
	if (!region) return { ...sel, region: code, regionChosen: true };
	return {
		...sel,
		region: region.code,
		regionChosen: true,
		timezone: region.timezone,
		timezoneChosen: false,
	};
}

// selectTimezone answers a clock pick, the strongest location signal there is:
// someone who names a city has said where they are more precisely than the
// language ever implied. So while the location is still only the language's
// pre-fill, the zone moves it — and the zone stays, because it is the thing they
// just chose. A location the person picked by hand is never moved by a clock.
export function selectTimezone(
	sel: WelcomeSelection,
	regions: RegionOption[],
	zones: TimezoneOption[],
	id: string,
): WelcomeSelection {
	const next: WelcomeSelection = { ...sel, timezone: id, timezoneChosen: true };
	if (next.regionChosen) return next;

	const first = findZone(zones, id)?.countries?.[0] ?? '';
	const region = findRegion(regions, first);
	if (region) next.region = region.code;
	return next;
}

// ── The lists ────────────────────────────────────────────────────────────────

// filterLanguages matches a language on anything a person might type for it: the
// name in either language, the parenthetical tail, the code, or the locale.
// Someone hunting for German types "German" without being able to read "Deutsch";
// someone who knows the code types "de".
export function filterLanguages(languages: LanguageOption[], query: string): LanguageOption[] {
	const q = query.trim().toLowerCase();
	if (q === '') return languages;
	return languages.filter(
		(l) =>
			l.name.toLowerCase().includes(q) ||
			l.nameTail.toLowerCase().includes(q) ||
			l.native.toLowerCase().includes(q) ||
			l.en.toLowerCase().includes(q) ||
			l.code.toLowerCase().includes(q) ||
			l.locale.toLowerCase().includes(q),
	);
}

// filterRegions matches a country on its name or its ISO code.
export function filterRegions(regions: RegionOption[], query: string): RegionOption[] {
	const q = query.trim().toLowerCase();
	if (q === '') return regions;
	return regions.filter((r) => r.name.toLowerCase().includes(q) || r.code.startsWith(q));
}

// zonesForRegion is the country-only view: the clocks tzdata names that country in,
// plus its own default when that default is a local name (Europe/Copenhagen) that
// is not itself a tzdata row. Sorted by the city a person reads.
export function zonesForRegion(zones: TimezoneOption[], region: RegionOption | undefined): TimezoneOption[] {
	if (!region || region.code === 'worldwide') return [];
	const own = zones.filter((z) => z.countries?.includes(region.code));
	const seen = new Set(own.map((z) => z.id));
	const fallback = zones.find((z) => z.id === region.timezone);
	if (fallback && !seen.has(fallback.id)) own.push(fallback);
	return own.sort((a, b) => zoneCity(a.id).localeCompare(zoneCity(b.id)));
}

// filterTimezones searches the whole set. A typed query always searches every
// clock — finding Tokyo must not require switching the view first — so the
// country-only view is only ever the unfiltered starting point.
export function filterTimezones(zones: TimezoneOption[], query: string): TimezoneOption[] {
	const q = query.trim().toLowerCase();
	if (q === '') return zones;
	return zones.filter((z) => z.id.toLowerCase().includes(q) || zoneCity(z.id).toLowerCase().includes(q));
}

// A grouped view of the full set: every continent, in the order the sorted table
// presents them, each holding its own zones.
export interface ZoneGroup {
	name: string;
	zones: TimezoneOption[];
}

export function groupTimezones(zones: TimezoneOption[]): ZoneGroup[] {
	const groups: ZoneGroup[] = [];
	let last: ZoneGroup | null = null;
	for (const zone of zones) {
		const name = zoneGroup(zone.id);
		if (!last || last.name !== name) {
			last = { name, zones: [] };
			groups.push(last);
		}
		last.zones.push(zone);
	}
	return groups;
}

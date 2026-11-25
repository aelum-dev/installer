import { describe, expect, it } from 'vitest';
import type { LanguageOption, RegionOption, TimezoneOption } from './types';
import {
	filterLanguages,
	filterRegions,
	filterTimezones,
	groupTimezones,
	selectLanguage,
	selectRegion,
	selectTimezone,
	zoneCity,
	zoneGroup,
	zonesForRegion,
	type WelcomeSelection,
} from './welcome-cascade';

// Small fixtures that keep the shapes of the generated tables: a language with and
// without a territory, a single-zone country, a multi-zone one, a country whose
// default clock is a local name tzdata does not list as its own row, and the
// worldwide entry.
const LANGUAGES: LanguageOption[] = [
	{ code: 'en_US', name: 'English', nameTail: 'United States', locale: 'en_US.UTF-8', native: 'English', en: 'English', territory: 'us' },
	{ code: 'en_GB', name: 'English', nameTail: 'United Kingdom', locale: 'en_GB.UTF-8', native: 'English', en: 'English', territory: 'gb' },
	{ code: 'de_DE', name: 'Deutsch', nameTail: 'Germany', locale: 'de_DE.UTF-8', native: 'Deutsch', en: 'German', territory: 'de' },
	{ code: 'eo', name: 'Esperanto', nameTail: '', locale: 'eo.UTF-8', native: 'Esperanto', en: 'Esperanto', territory: '' },
];

const REGIONS: RegionOption[] = [
	{ code: 'us', name: 'United States', timezone: 'America/New_York', keymap: 'us', xkb: 'us', mirrors: true },
	{ code: 'gb', name: 'United Kingdom', timezone: 'Europe/London', keymap: 'uk', xkb: 'gb', mirrors: true },
	{ code: 'de', name: 'Germany', timezone: 'Europe/Berlin', keymap: 'de', xkb: 'de', mirrors: true },
	// Denmark's default is a tzdata link name, not a zone1970 row of its own.
	{ code: 'dk', name: 'Denmark', timezone: 'Europe/Copenhagen', keymap: 'dk', xkb: 'dk', mirrors: true },
	// A country Arch has no mirror section for: still a place, no mirrors of its own.
	{ code: 'ua', name: 'Ukraine', timezone: 'Europe/Kyiv', keymap: 'ua', xkb: 'ua', mirrors: false },
	{ code: 'worldwide', name: 'Worldwide', timezone: 'UTC', keymap: 'us', xkb: 'us', mirrors: true },
];

const ZONES: TimezoneOption[] = [
	{ id: 'America/New_York', countries: ['us'], comment: 'east coast' },
	{ id: 'America/Los_Angeles', countries: ['us'], comment: 'west coast' },
	{ id: 'Europe/London', countries: ['gb'], comment: '' },
	{ id: 'Europe/Berlin', countries: ['de'], comment: '' },
	{ id: 'Europe/Copenhagen', countries: ['dk'], comment: '' },
	{ id: 'Europe/Kyiv', countries: ['ua'], comment: '' },
	{ id: 'Asia/Tokyo', countries: ['jp'], comment: '' },
	{ id: 'UTC', countries: [], comment: '' },
];

const START: WelcomeSelection = {
	locale: 'en_US',
	region: 'us',
	timezone: 'America/New_York',
	regionChosen: false,
	timezoneChosen: false,
};

describe('zoneCity and zoneGroup', () => {
	it('reads the city a person recognises', () => {
		expect(zoneCity('Europe/Kyiv')).toBe('Kyiv');
		expect(zoneCity('America/Argentina/Buenos_Aires')).toBe('Buenos Aires');
		expect(zoneCity('UTC')).toBe('UTC');
	});

	it('files a zone under its continent', () => {
		expect(zoneGroup('Europe/Kyiv')).toBe('Europe');
		expect(zoneGroup('UTC')).toBe('UTC');
		expect(zoneGroup('Kiev')).toBe('Other');
	});
});

describe('the language list', () => {
	it('matches the English name someone cannot read in the script', () => {
		expect(filterLanguages(LANGUAGES, 'german').map((l) => l.code)).toEqual(['de_DE']);
	});

	it('matches the native name and the code', () => {
		expect(filterLanguages(LANGUAGES, 'deutsch').map((l) => l.code)).toEqual(['de_DE']);
		expect(filterLanguages(LANGUAGES, 'de_').map((l) => l.code)).toEqual(['de_DE']);
	});

	it('keeps every written form of one language when the query names the language', () => {
		expect(filterLanguages(LANGUAGES, 'english').map((l) => l.code)).toEqual(['en_US', 'en_GB']);
	});

	it('matches the parenthetical tail, so "germany" finds Deutsch (Germany)', () => {
		expect(filterLanguages(LANGUAGES, 'germany').map((l) => l.code)).toEqual(['de_DE']);
	});

	it('returns everything for an empty query', () => {
		expect(filterLanguages(LANGUAGES, '   ')).toBe(LANGUAGES);
	});
});

describe('the location list', () => {
	it('matches a country on its name or its ISO code', () => {
		expect(filterRegions(REGIONS, 'uni').map((r) => r.code)).toEqual(['us', 'gb']);
		expect(filterRegions(REGIONS, 'ua')).toEqual([REGIONS[4]]);
	});

	it('keeps a country without its own mirrors — where a machine is is not a mirror question', () => {
		expect(filterRegions(REGIONS, 'ukraine')).toHaveLength(1);
	});
});

describe('the clock list', () => {
	it('lists a country its own clocks, including a default that is a link name', () => {
		expect(zonesForRegion(ZONES, REGIONS[0]).map((z) => z.id)).toEqual(['America/Los_Angeles', 'America/New_York']);
		expect(zonesForRegion(ZONES, REGIONS[3]).map((z) => z.id)).toEqual(['Europe/Copenhagen']);
	});

	it('has no clocks of its own for the worldwide location', () => {
		expect(zonesForRegion(ZONES, REGIONS[5])).toEqual([]);
		expect(zonesForRegion(ZONES, undefined)).toEqual([]);
	});

	it('searches every clock by full name or by city', () => {
		expect(filterTimezones(ZONES, 'tokyo').map((z) => z.id)).toEqual(['Asia/Tokyo']);
		expect(filterTimezones(ZONES, 'america/').map((z) => z.id)).toEqual([
			'America/New_York',
			'America/Los_Angeles',
		]);
	});

	it('groups the full set by continent, in the order the table presents them', () => {
		const groups = groupTimezones(filterTimezones(ZONES, ''));
		expect(groups.map((g) => g.name)).toEqual(['America', 'Europe', 'Asia', 'UTC']);
		expect(groups[1].zones.map((z) => z.id)).toEqual([
			'Europe/London',
			'Europe/Berlin',
			'Europe/Copenhagen',
			'Europe/Kyiv',
		]);
	});
});

describe('the cascade', () => {
	it('a language pre-fills its country and that country’s clock', () => {
		const next = selectLanguage(START, LANGUAGES, REGIONS, 'de_DE');
		expect(next).toMatchObject({ locale: 'de_DE', region: 'de', timezone: 'Europe/Berlin' });
	});

	it('a language with no country leaves the location alone', () => {
		const next = selectLanguage(START, LANGUAGES, REGIONS, 'eo');
		expect(next).toMatchObject({ locale: 'eo', region: 'us', timezone: 'America/New_York' });
	});

	it('a language does not move a location the person chose', () => {
		const chosen = { ...START, regionChosen: true };
		expect(selectLanguage(chosen, LANGUAGES, REGIONS, 'de_DE')).toMatchObject({ region: 'us' });
	});

	it('a location re-defaults the clock, because choosing a country is choosing its clock', () => {
		const hand = { ...START, timezone: 'Asia/Tokyo', timezoneChosen: true };
		const next = selectRegion(hand, REGIONS, 'ua');
		expect(next).toMatchObject({ region: 'ua', regionChosen: true, timezone: 'Europe/Kyiv', timezoneChosen: false });
	});

	it('a hand-picked clock moves a location that is still only a suggestion', () => {
		const next = selectTimezone(START, REGIONS, ZONES, 'Europe/Kyiv');
		expect(next).toMatchObject({ region: 'ua', regionChosen: false, timezone: 'Europe/Kyiv', timezoneChosen: true });
	});

	it('a hand-picked clock never moves a location the person chose', () => {
		const chosen = { ...START, regionChosen: true };
		expect(selectTimezone(chosen, REGIONS, ZONES, 'Europe/Kyiv')).toMatchObject({ region: 'us' });
	});

	it('a clock keeps the location when it belongs to no country the installer offers', () => {
		expect(selectTimezone(START, REGIONS, ZONES, 'UTC')).toMatchObject({ region: 'us', timezone: 'UTC' });
	});

	it('the cascade never mutates the selection it was given', () => {
		const before = { ...START };
		selectLanguage(START, LANGUAGES, REGIONS, 'de_DE');
		selectRegion(START, REGIONS, 'ua');
		selectTimezone(START, REGIONS, ZONES, 'Europe/Kyiv');
		expect(START).toEqual(before);
	});
});

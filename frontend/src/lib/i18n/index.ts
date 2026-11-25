// Re-export everything from the .svelte.ts file for use in both .ts and .svelte files
export {
	t,
	setLocale,
	locale,
	getIsRTL,
	backendLocaleToFrontend,
	isDmsLocale,
	DEFAULT_LOCALE,
	type Locale,
} from './index.svelte';

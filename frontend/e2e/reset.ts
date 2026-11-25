import type { Page } from '@playwright/test';

/**
 * Make the shared warm backend agree with the English UI the test just clicked.
 *
 * `wails dev` serves the whole suite from one backend that remembers the last
 * test's language/region/clock. Clicking the English row pushes that choice to
 * the backend, but the push is asynchronous and the test's next navigation can
 * abort it mid-flight — so a later page that reads the backend (the Confirmation
 * summary does) can still see the previous test's language, cascading a location
 * and clock the test never chose. Awaiting the binding here settles the backend
 * on English before the test proceeds.
 *
 * Call it right after clicking the `en_US` row: the click is what keeps the UI
 * itself in English, this is what guarantees the backend follows.
 */
export async function settleEnglishBackend(page: Page): Promise<void> {
	await page.evaluate(() => (window as any).go.gui.App.SetSelectedLanguage('en_US'));
}

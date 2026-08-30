// NerdFontsSymbols helpers — the installer ships the same self-hosted icon
// glyph set as the website (frontend/src/assets/fonts/NerdFontsSymbols.woff2).
// Icons are referenced by their Nerd Fonts code point in hex (e.g. "f0ac").

// Render a NerdFontsSymbols glyph from a hex code point.
export function glyph(hex: string): string {
	return String.fromCodePoint(parseInt(hex, 16));
}

// Step icons for the wizard's left column (page-info). Kept in one place so the
// GUI icons stay in sync with the website's installSteps. Installation (05) and
// Finish (06) intentionally have no icon — they use their own centered layout.
export const STEP_ICONS: Record<string, string> = {
	welcome: 'f0ac', // globe
	userAccount: 'eb99', // account
	diskSelection: 'f02ca', // harddisk
	confirmation: 'f012c', // check
};

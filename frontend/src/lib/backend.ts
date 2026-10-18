// Typed facade over the Wails-generated bindings.
//
// The generated modules live in `wailsjs/` (outside `src/`) and marshal Go
// structs with a `convertValues` helper, so their types don't line up with the
// hand-written domain types in `types.ts`. Components and lib modules import
// the backend from here instead of reaching into `@wails/*` directly:
//
//   - one place to adapt generated shapes to the app's own types,
//   - one import site to change if the Wails output moves,
//   - no per-file `@ts-expect-error` to suppress resolution failures.
//
// Pass-through bindings keep the generated PascalCase names so call sites read
// the same as the Go methods they invoke.

import {
	ClearStartPage,
	GetDiskPartitions as rawGetDiskPartitions,
	GetDisks as rawGetDisks,
	GetDryRun,
	GetLanguages as rawGetLanguages,
	GetManualLayout as rawGetManualLayout,
	GetSelectedLanguage,
	GetStartPage as rawGetStartPage,
	RebootAndClose,
	SaveDiskSelection,
	SaveManualLayout as rawSaveManualLayout,
	SaveUserAccount,
	SetCurrentPage,
	SetLuks,
	SetLuksPassphrase,
	SetLvm,
	SetSelectedLanguage,
	SetStartPage,
	SetWallpaper,
	StartInstallation,
	ValidateDiskSelection,
	ValidateManualLayout as rawValidateManualLayout,
	ValidateUserAccount,
} from '@wails/go/gui/App.js';
import { EventsOn, LogInfo } from '@wails/runtime/runtime.js';
import { gui } from '@wails/go/models';
import type { shared } from '@wails/go/models';
import type { DiskInfo, LanguageOption, ManualLayout, PartitionEntry } from '$lib/types';

// ── Pass-through bindings (signatures already line up) ──────────────────────

export {
	ClearStartPage,
	GetDryRun,
	GetSelectedLanguage,
	RebootAndClose,
	SaveDiskSelection,
	SaveUserAccount,
	SetCurrentPage,
	SetLuks,
	SetLuksPassphrase,
	SetLvm,
	SetSelectedLanguage,
	SetStartPage,
	SetWallpaper,
	StartInstallation,
	ValidateDiskSelection,
	ValidateUserAccount,
	EventsOn,
	LogInfo,
};

// ── Adapted bindings (generated struct → domain type) ───────────────────────

// setCurrentPage wraps the generated gui.PageID struct so callers pass a plain
// wizard page number.
export async function setCurrentPage(page: number): Promise<void> {
	await SetCurrentPage(new gui.PageID({ Value: page }));
}

// GetManualLayout returns a generated shared.ManualLayout (a class carrying a
// `convertValues` method and a `type: string` field); the app works with the
// plain ManualLayout interface from types.ts.
export async function GetManualLayout(path: string): Promise<ManualLayout> {
	return (await rawGetManualLayout(path)) as unknown as ManualLayout;
}

export async function SaveManualLayout(layout: ManualLayout): Promise<void> {
	await rawSaveManualLayout(layout as unknown as shared.ManualLayout);
}

export async function ValidateManualLayout(layout: ManualLayout): Promise<void> {
	await rawValidateManualLayout(layout as unknown as shared.ManualLayout);
}

export async function GetDisks(): Promise<DiskInfo[]> {
	return (await rawGetDisks()) as unknown as DiskInfo[];
}

export async function GetDiskPartitions(path: string): Promise<PartitionEntry[]> {
	return (await rawGetDiskPartitions(path)) as unknown as PartitionEntry[];
}

export async function GetLanguages(): Promise<LanguageOption[]> {
	return (await rawGetLanguages()) as unknown as LanguageOption[];
}

export async function GetStartPage(): Promise<number> {
	return rawGetStartPage();
}

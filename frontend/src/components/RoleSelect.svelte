<script lang="ts">
	import type { PartitionType } from '$lib/types';
	import { TYPE_ESP, TYPE_SYSTEM } from '$lib/types';
	import { t } from '$lib/i18n';
	import './RoleSelect.css';

	// A fully custom role selector so the dropdown menu is themed to the installer's
	// light UI. The native <select> popup is drawn by the OS/browser and can't be
	// recoloured, so it would render a dark list over a light theme.
	//
	// Options follow the ARIA listbox pattern: a trigger button with aria-haspopup,
	// a menu with role="listbox", and option buttons with role="option".
	//
	// options keep their translation key so labels stay in sync with the rest of the
	// installer; role is used only for colour-coding the menu rows.
	const OPTIONS = [
		{ value: '' as PartitionType, labelKey: 'manual.type.preserve', role: '' as const },
		{ value: TYPE_ESP, labelKey: 'manual.type.esp', role: 'esp' as const },
		{ value: TYPE_SYSTEM, labelKey: 'manual.type.system', role: 'system' as const },
	] as const;

	let { value, onChange }: { value: PartitionType; onChange: (v: PartitionType) => void } = $props();

	// Unique id per instance so aria-controls points each trigger at its own menu.
	// ($props.id() is Svelte's per-instance id; a module- or instance-local counter
	// would hand every row the same id.)
	const menuId = $props.id();

	let open = $state(false);
	let wrap: HTMLElement | null = $state(null);

	function toggle(): void {
		open = !open;
	}

	function pick(v: PartitionType): void {
		// setType treats re-selecting the active role as a clear-back-to-Preserve, so
		// the toggle semantics stay identical to the old per-row pills.
		onChange(v);
		open = false;
	}

	// Close when the click lands outside the control.
	$effect(() => {
		if (!open) return;
		const onDoc = (e: MouseEvent): void => {
			if (wrap && !wrap.contains(e.target as Node)) open = false;
		};
		document.addEventListener('click', onDoc);
		return () => document.removeEventListener('click', onDoc);
	});

	function onKey(e: KeyboardEvent): void {
		if (e.key === 'Escape') open = false;
	}
</script>

<div class="role-select" bind:this={wrap}>
	<button
		type="button"
		class={['role-trigger', { esp: value === TYPE_ESP, system: value === TYPE_SYSTEM }]}
		aria-label={t('manual.type')}
		aria-haspopup="listbox"
		aria-expanded={open}
		aria-controls={menuId}
		onclick={toggle}
		onkeydown={onKey}
	>
		<span class="role-trigger-label">{t(value === '' ? 'manual.type.preserve' : 'manual.type.' + value)}</span>
		<span class="role-chevron" aria-hidden="true">
			<svg viewBox="0 0 16 16" width="12" height="12" class={['role-chevron-arrow', { open }]}><path d="M3 8l5 5 5-5" /></svg>
		</span>
	</button>

	{#if open}
		<div class="role-menu" id={menuId} role="listbox" tabindex="-1">
			{#each OPTIONS as opt (opt.value)}
				<button
					type="button"
					role="option"
					aria-selected={value === opt.value}
					class={['role-option', { esp: opt.role === 'esp', system: opt.role === 'system' }]}
					onclick={() => pick(opt.value)}
				>
					<span class="role-option-dot" aria-hidden="true"></span>
					{t(opt.labelKey)}
				</button>
			{/each}
		</div>
	{/if}
</div>

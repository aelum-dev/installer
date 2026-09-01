<script lang="ts">
	import type { Snippet } from 'svelte';
	import './StateMessage.css';

	let { error = false, children }: { error?: boolean; children?: Snippet } = $props();
</script>

<div class={['state-message', { error }]}>{#if children}{@render children()}{/if}</div>

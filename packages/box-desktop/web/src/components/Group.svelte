<script lang="ts">
	// A titled group of rows in a rounded card (settings.py's group): lead
	// says what it is under the title, note adds to it under the rows, icon
	// goes before the title.
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';

	let {
		title = '', note = '', lead = '', icon = '', attention = false, aside, children
	}: {
		title?: string; note?: string; lead?: string; icon?: string; attention?: boolean;
		aside?: Snippet; children?: Snippet;
	} = $props();
</script>

<div class="group">
	{#if title || aside}
		<div class="group-head" class:with-lead={!!lead}>
			<div class="named">
				{#if icon}<Icon name={icon} px={20} role="accent" />{/if}
				<span class="group-title">{title}</span>
				{#if aside && lead}<span class="grow"></span>{@render aside()}{/if}
			</div>
			{#if lead}<span class="group-note">{lead}</span>{/if}
			{#if aside && !lead}{@render aside()}{/if}
		</div>
	{/if}
	<div class="rows" class:attention>
		{@render children?.()}
	</div>
	{#if note}<span class="group-note">{note}</span>{/if}
</div>

<style>
	.grow { flex: 1; }
</style>

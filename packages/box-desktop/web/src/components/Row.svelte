<script lang="ts">
	// A row (settings.py's row): an icon maybe (a small coloured tile), the
	// title (and a tag, or another small thing such as a status chip, beside
	// it) and its explanation, then its controls; under it, more if given.
	import type { Snippet } from 'svelte';
	import Badge from './Badge.svelte';

	let {
		title = '', sub = '', icon = '', role = 'accent', big = false, iconPx = 0, tag = '',
		beside, ends, below, lead
	}: {
		title?: string; sub?: string; icon?: string; role?: string; big?: boolean; iconPx?: number; tag?: string;
		beside?: Snippet; ends?: Snippet; below?: Snippet; lead?: Snippet;
	} = $props();
	const px = $derived(iconPx || (big ? 28 : 20));
</script>

<div class="row" style:flex-wrap={below ? 'wrap' : null}>
	{@render lead?.()}
	{#if icon}<Badge name={icon} {px} {role} />{/if}
	<div class="words">
		{#if tag || beside}
			<div class="named">
				<span class="row-title">{title}</span>
				{#if tag}<span class="tag">{tag}</span>{/if}
				{@render beside?.()}
			</div>
		{:else if title}
			<span class="row-title">{title}</span>
		{/if}
		{#if sub}<span class="row-sub">{sub}</span>{/if}
	</div>
	{#if ends}<div class="row-ends">{@render ends()}</div>{/if}
	{#if below}<div style:flex-basis="100%">{@render below()}</div>{/if}
</div>

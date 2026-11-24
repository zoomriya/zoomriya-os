<script lang="ts">
	// A card over the desktop with a title and Close (support.py's sheet).
	import type { Snippet } from 'svelte';
	import { _ } from '../lib/i18n.svelte';

	let { title, width = 560, close, children }: { title: string; width?: number; close: () => void; children?: Snippet } = $props();
</script>

<div class="shade" role="presentation">
	<div class="card" style:width="{width}px" style:max-width="100%" role="dialog" aria-modal="true" aria-label={title}>
		<div class="row-between">
			<h2 class="title grow">{title}</h2>
			<button onclick={close}>{_('Close')}</button>
		</div>
		{@render children?.()}
	</div>
</div>

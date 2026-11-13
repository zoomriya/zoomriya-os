<script lang="ts">
	// An app's tile, the same size as a tool's: its letters on a colour of its own.
	import { appSvg, BODY_SHARE } from '../lib/tiles';

	let { id, name, size = 52 }: { id: string; name: string; size?: number } = $props();
	const px = $derived(Math.round(size / BODY_SHARE));
	const svg = $derived(appSvg(id, name, px));
</script>

<span class="icon tile-img" style:width="{px}px" style:height="{px}px">{@html svg}</span>

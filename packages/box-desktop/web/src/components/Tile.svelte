<script lang="ts">
	// A tool's tile, size pixels across; its shadow takes a little room around it.
	import { BODY_SHARE, tileSvg } from '../lib/tiles';

	let { kind, size = 52 }: { kind: string; size?: number } = $props();
	const px = $derived(Math.round(size / BODY_SHARE));
	const svg = $derived(tileSvg(kind, px, px >= 30));
</script>

<span class="icon tile-img" style:width="{px}px" style:height="{px}px">{@html svg}</span>

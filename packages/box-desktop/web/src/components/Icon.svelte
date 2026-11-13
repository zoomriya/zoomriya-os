<script lang="ts">
	// A line icon (icons.ts's glyph), in its role's colour: text, muted, ok,
	// warn, accent or white.
	import { glyphSvg } from '../lib/icons';

	let { name, px = 19, role = 'text', width = 1.8 }: { name: string; px?: number; role?: string; width?: number } = $props();
</script>

<span class="icon role-{role}" style:width="{px}px" style:height="{px}px">{@html glyphSvg(name, px, width)}</span>

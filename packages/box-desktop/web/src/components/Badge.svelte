<script lang="ts">
	// A line icon as a small tile: white on its colour, on the role's colour
	// when that says more (a warning, a done), or on shade if given.
	import { badgeShade, badgeSvg } from '../lib/icons';

	let { name, px = 28, role = 'accent', shade = '' }: { name: string; px?: number; role?: string; shade?: string } = $props();
	const svg = $derived(badgeSvg(name, px, badgeShade(name, role, shade)));
</script>

<span class="icon" style:width="{px}px" style:height="{px}px">{@html svg}</span>

<script lang="ts">
	// A row whose whole width is its content (a slider, a level, a form).
	import type { Snippet } from 'svelte';

	let { children }: { children?: Snippet } = $props();
</script>

<div class="wide">{@render children?.()}</div>

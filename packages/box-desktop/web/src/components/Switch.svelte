<script lang="ts">
	// A switch; fixed: one that only shows a state, since it isn't changed here.
	import { _ } from '../lib/i18n.svelte';

	let {
		on = $bindable(false), disabled = false, fixed = false, label = '', changed
	}: { on?: boolean; disabled?: boolean; fixed?: boolean; label?: string; changed?: (on: boolean) => void } = $props();
	const tip = $derived(fixed ? (on ? _('Always on') : _('Off')) : '');
</script>

<label class="switch" class:fixed title={tip}>
	<input type="checkbox" role="switch" bind:checked={on} disabled={disabled || fixed} aria-label={label || tip}
		onchange={() => changed?.(on)} />
	<span></span>
</label>

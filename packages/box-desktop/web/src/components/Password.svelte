<script lang="ts">
	// A password field with its eye, to show what's typed.
	import { _ } from '../lib/i18n.svelte';
	import Icon from './Icon.svelte';

	let {
		value = $bindable(''), placeholder = '', disabled = false, autofocus = false, enter, autocomplete = 'current-password'
	}: { value?: string; placeholder?: string; disabled?: boolean; autofocus?: boolean; enter?: () => void; autocomplete?: AutoFill } = $props();
	let shown = $state(false);
	let field: HTMLInputElement;
	$effect(() => {
		if (autofocus) field?.focus();
	});
</script>

<div class="password">
	<input bind:this={field} type={shown ? 'text' : 'password'} bind:value {placeholder} {disabled} {autocomplete}
		aria-label={placeholder} onkeydown={(e) => e.key === 'Enter' && enter?.()} />
	<button type="button" class="peek" tabindex="-1" aria-label={shown ? _('Hide password') : _('Show password')} onclick={() => (shown = !shown)}>
		<Icon name={shown ? 'eye-off' : 'eye'} px={16} role="muted" />
	</button>
</div>

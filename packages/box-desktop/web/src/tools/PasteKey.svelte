<script lang="ts">
	// Paste a key, from the computer the admin will use.
	import { _ } from '../lib/i18n.svelte';
	import Sheet from '../components/Sheet.svelte';
	import { addKey } from './keys';

	let { done, close }: { done: () => void; close: () => void } = $props();
	let name = $state('');
	let key = $state('');

	function go(): void {
		if (!key.trim()) return;
		close();
		addKey(key.trim(), name.trim(), done);
	}
</script>

<Sheet title={_('Paste a key')} {close}>
	<!-- svelte-ignore a11y_autofocus -->
	<input type="text" bind:value={name} placeholder={_("A name, like Anna's laptop")} aria-label={_("A name, like Anna's laptop")} autofocus />
	<span class="row-sub">{_('Public key (ssh-ed25519 …)')}</span>
	<textarea class="mono" bind:value={key} spellcheck="false" style:height="110px" aria-label={_('Public key (ssh-ed25519 …)')}></textarea>
	<div class="row-end"><button class="primary" onclick={go}>{_('Add')}</button></div>
</Sheet>

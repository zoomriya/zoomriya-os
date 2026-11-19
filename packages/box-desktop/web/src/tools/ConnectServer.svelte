<script lang="ts">
	// Your own server (ZOOMRIYA Connect, "Your own server"): one address, and
	// the box reads everything else from it. Phones pair again afterwards.
	import { desk } from '../lib/desk.svelte';
	import { _ } from '../lib/i18n.svelte';
	import Sheet from '../components/Sheet.svelte';
	import { connectWords, useServer } from './connect';

	let { done, close, ours = false }: { done: () => void; close: () => void; ours?: boolean } = $props();
	let url = $state('https://');

	function go(): void {
		const target = ours ? null : url.trim();
		close();
		desk.confirm({
			title: ours ? _("Use ZOOMRIYA's server again?") : _('Use your own server?'),
			text: _("The box gets a new name on that server, and every phone and laptop pairs again at home."),
			button: _('Switch'),
			action: async (password) => {
				try {
					return await useServer(target, password);
				} catch (e) {
					throw new Error(connectWords(e));
				}
			},
			done
		});
	}
</script>

<Sheet title={ours ? _("ZOOMRIYA's server") : _('Your own server')} {close} width={520}>
	{#if ours}
		<p class="lead">{_("Go back to ZOOMRIYA's server, relay and names.")}</p>
	{:else}
		<p class="lead">{_("Type your ZOOMRIYA Connect server's address. The box reads everything else from it.")}</p>
		<!-- svelte-ignore a11y_autofocus -->
		<input type="url" bind:value={url} placeholder="https://connect.example.com" aria-label={_("Your server's address")} autofocus spellcheck="false" />
	{/if}
	<div class="row-end"><button class="primary" onclick={go} disabled={!ours && url.trim().length < 12}>{_('Continue')}</button></div>
</Sheet>

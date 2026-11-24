<script lang="ts">
	// Pair a phone (zoomriya-connect, docs/direct-plus.md §6): the QR code
	// the ZOOMRIYA app scans, here at home. It works once, for ten
	// minutes; the sheet closes by itself when the phone is paired.
	import { onDestroy } from 'svelte';
	import { _, fill } from '../lib/i18n.svelte';
	import QR from '../components/QR.svelte';
	import Sheet from '../components/Sheet.svelte';
	import { deviceWords, devices, pairing, stopPairing } from './devices';

	let { done, close }: { done: () => void; close: () => void } = $props();
	let name = $state('');
	let url = $state('');
	let left = $state(0);
	let problem = $state('');
	let paired = $state('');
	let known = new Set<string>();
	let tick: ReturnType<typeof setInterval> | undefined;

	async function show(): Promise<void> {
		problem = '';
		try {
			known = new Set((await devices()).map((d) => d.id));
			const p = await pairing(name.trim());
			url = p.url;
			left = p.expires_in;
			clearInterval(tick);
			tick = setInterval(watch, 2000);
		} catch (e) {
			problem = deviceWords(e);
		}
	}

	async function watch(): Promise<void> {
		left = Math.max(0, left - 2);
		if (left === 0) {
			clearInterval(tick);
			url = '';
			return;
		}
		try {
			const added = (await devices()).find((d) => !known.has(d.id));
			if (added) {
				clearInterval(tick);
				url = '';
				paired = added.name;
				done();
			}
		} catch {
			/* try again in a moment */
		}
	}

	function finish(): void {
		clearInterval(tick);
		if (url) void stopPairing().catch(() => {});
		close();
	}

	onDestroy(() => clearInterval(tick));
	const minutes = $derived(Math.ceil(left / 60));
</script>

<Sheet title={_('Pair a phone')} close={finish} width={520}>
	{#if paired}
		<p class="lead">{fill(_('{name} is paired.'), { name: paired })}</p>
		<div class="row-end"><button class="primary" onclick={finish}>{_('Done')}</button></div>
	{:else if url}
		<p class="lead">{_('Open the ZOOMRIYA app on the phone, choose Pair with a box, and scan this code. The phone must be on your home network.')}</p>
		<div class="center"><QR text={url} size={260} /></div>
		<p class="row-sub center">{fill(_('This code works once, for {n} more minutes.'), { n: minutes })}</p>
		<div class="row-end"><button onclick={finish}>{_('Cancel')}</button></div>
	{:else}
		<p class="lead">{_('Pair each phone once, here at home.')}</p>
		<!-- svelte-ignore a11y_autofocus -->
		<input type="text" bind:value={name} placeholder={_("Whose phone, like Anna's phone")} aria-label={_("Whose phone, like Anna's phone")} autofocus />
		{#if problem}<p class="error" role="alert">{problem}</p>{/if}
		<div class="row-end"><button class="primary" onclick={show}>{_('Show the code')}</button></div>
	{/if}
</Sheet>

<style>
	.center { display: flex; justify-content: center; text-align: center; }
</style>

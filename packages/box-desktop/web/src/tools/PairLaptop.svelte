<script lang="ts">
	// Add a laptop (docs/laptops.md in zoomriya-connect): a short code to type
	// into the ZOOMRIYA browser extension, at home. Laptops often have no
	// camera, so it's a code, not a QR code. It works once, for ten minutes,
	// with three tries; the sheet closes by itself when the laptop is paired.
	import { onDestroy } from 'svelte';
	import { _, fill } from '../lib/i18n.svelte';
	import Sheet from '../components/Sheet.svelte';
	import { connectWords, devices, laptopPairing, stopPairing } from './connect';

	let { done, close }: { done: () => void; close: () => void } = $props();
	let name = $state('');
	let code = $state('');
	let fqdn = $state('');
	let left = $state(0);
	let problem = $state('');
	let paired = $state('');
	let known = new Set<string>();
	let tick: ReturnType<typeof setInterval> | undefined;

	async function show(): Promise<void> {
		problem = '';
		try {
			known = new Set((await devices()).map((d) => d.id));
			const p = await laptopPairing(name.trim());
			code = p.code;
			fqdn = p.fqdn;
			left = p.expires_in;
			clearInterval(tick);
			tick = setInterval(watch, 2000);
		} catch (e) {
			problem = connectWords(e);
		}
	}

	async function watch(): Promise<void> {
		left = Math.max(0, left - 2);
		if (left === 0) {
			clearInterval(tick);
			code = '';
			return;
		}
		try {
			const added = (await devices()).find((d) => !known.has(d.id));
			if (added) {
				clearInterval(tick);
				code = '';
				paired = added.name;
				done();
			}
		} catch {
			/* try again in a moment */
		}
	}

	function finish(): void {
		clearInterval(tick);
		if (code) void stopPairing().catch(() => {});
		close();
	}

	onDestroy(() => clearInterval(tick));
	const minutes = $derived(Math.ceil(left / 60));
</script>

<Sheet title={_('Add a laptop')} close={finish} width={520}>
	{#if paired}
		<p class="lead">{fill(_('{name} is paired. It can reach the box from anywhere now.'), { name: paired })}</p>
		<div class="row-end"><button class="primary" onclick={finish}>{_('Done')}</button></div>
	{:else if code}
		<p class="lead">{_('On the laptop, click the ZOOMRIYA button in the browser and type these. The laptop must be on your home network.')}</p>
		<div class="pair">
			<span class="row-sub">{_("Your box's name")}</span>
			<span class="value selectable">{fqdn}</span>
			<span class="row-sub">{_('Code')}</span>
			<span class="value code selectable">{code}</span>
		</div>
		<p class="row-sub">{fill(_('This code works once, for {n} more minutes.'), { n: minutes })}</p>
		<div class="row-end"><button onclick={finish}>{_('Cancel')}</button></div>
	{:else}
		<p class="lead">{_('Pair each laptop once, here at home. It needs the ZOOMRIYA browser extension, for Chrome, Edge, Brave or Opera.')}</p>
		<!-- svelte-ignore a11y_autofocus -->
		<input type="text" bind:value={name} placeholder={_("Whose laptop, like Anna's laptop")} aria-label={_("Whose laptop, like Anna's laptop")} autofocus />
		{#if problem}<p class="error" role="alert">{problem}</p>{/if}
		<div class="row-end"><button class="primary" onclick={show}>{_('Show the code')}</button></div>
	{/if}
</Sheet>

<style>
	.pair { display: grid; grid-template-columns: auto 1fr; gap: 8px 16px; align-items: baseline; }
	.value { font-size: 15px; font-weight: 600; }
	.code { font-family: ui-monospace, Menlo, monospace; font-size: 26px; letter-spacing: 0.12em; }
</style>

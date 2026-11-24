<script lang="ts">
	// Turn on sign-in codes by email for yourself (ZOOMRIYA ID's
	// /auth/second-step/setup): your password and an address; a code goes
	// there; entering it turns them on (or moves them to that address).
	import { untrack } from 'svelte';
	import { post } from '../lib/api';
	import { _, fill } from '../lib/i18n.svelte';
	import { isAddress } from '../lib/mail';
	import Password from '../components/Password.svelte';
	import Sheet from '../components/Sheet.svelte';
	import Spinner from '../components/Spinner.svelte';

	let { address: was = '', changed, close }: { address?: string; changed: () => void; close: () => void } = $props();
	let address = $state(untrack(() => was));
	let password = $state('');
	let code = $state('');
	let sent = $state(false);
	let problem = $state('');
	let busy = $state(false);

	async function send(): Promise<void> {
		if (!isAddress(address)) {
			problem = _("That doesn't look like an email address.");
			return;
		}
		if (!password) {
			problem = _('Type your password.');
			return;
		}
		busy = true;
		problem = '';
		try {
			await post('/api/me/second-step/setup', { password, email: address.trim() });
			sent = true;
			password = '';
		} catch (e) {
			problem = (e as Error).message;
			password = '';
		} finally {
			busy = false;
		}
	}

	async function finish(): Promise<void> {
		if (!code.trim()) return;
		busy = true;
		problem = '';
		try {
			await post('/api/me/second-step/finish', { code });
			close();
			changed();
		} catch (e) {
			problem = (e as Error).message;
			code = '';
		} finally {
			busy = false;
		}
	}
</script>

<Sheet title={_('Sign-in codes by email')} {close}>
	{#if !sent}
		<p class="lead">{_('After your password, Box Desktop asks for a code we email you. A device you sign in on is remembered for 30 days.')}</p>
		<!-- svelte-ignore a11y_autofocus -->
		<input type="email" bind:value={address} placeholder={_('Your email address')} aria-label={_('Your email address')}
			autocapitalize="none" spellcheck="false" autofocus />
		<Password bind:value={password} placeholder={_('Your password')} enter={send} />
		{#if problem}<p class="error" role="alert">{problem}</p>{/if}
		<div class="row-end">
			{#if busy}<Spinner />{/if}
			<button class="primary" onclick={send} disabled={busy}>{_('Send a code')}</button>
		</div>
	{:else}
		<p class="lead">{fill(_('We sent a 6-digit code to {address}. Enter it to turn sign-in codes on.'), { address: address.trim() })}</p>
		<!-- svelte-ignore a11y_autofocus -->
		<input type="text" inputmode="numeric" bind:value={code} placeholder={_('6-digit code')} aria-label={_('6-digit code')}
			autocomplete="one-time-code" autofocus onkeydown={(e) => e.key === 'Enter' && finish()} />
		{#if problem}<p class="error" role="alert">{problem}</p>{/if}
		<div class="row-end">
			<button onclick={() => { sent = false; problem = ''; code = ''; }}>{_('Back')}</button>
			{#if busy}<Spinner />{/if}
			<button class="primary" onclick={finish} disabled={busy}>{_('Turn on')}</button>
		</div>
	{/if}
</Sheet>

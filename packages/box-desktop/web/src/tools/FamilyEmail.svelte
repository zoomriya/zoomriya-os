<script lang="ts">
	// The family's email account (ZOOMRIYA ID's /mail): the address and its
	// password; the server is guessed, and More settings shows it. ZOOMRIYA
	// ID signs in to it first and keeps it only when that works.
	import { untrack } from 'svelte';
	import { put, type Answer } from '../lib/api';
	import { _ } from '../lib/i18n.svelte';
	import { guessServer, isAddress } from '../lib/mail';
	import Password from '../components/Password.svelte';
	import Segmented from '../components/Segmented.svelte';
	import Sheet from '../components/Sheet.svelte';
	import Spinner from '../components/Spinner.svelte';

	let { current, changed, close }: { current: Answer; changed: () => void; close: () => void } = $props();
	let address = $state(untrack(() => String(current.username ?? '')));
	let password = $state('');
	let more = $state(false);
	let host = $state(untrack(() => String(current.host ?? '')));
	let port = $state(untrack(() => String(current.port ?? '')));
	let security = $state(untrack(() => String(current.security ?? 'starttls')));
	let problem = $state('');
	let busy = $state(false);

	/** Guessed again while the server isn't shown and the address changes. */
	function guess(): void {
		if (more) return;
		const found = guessServer(address);
		host = found.host;
		port = String(found.port);
		security = found.security;
	}

	async function save(): Promise<void> {
		if (!isAddress(address)) {
			problem = _("That doesn't look like an email address.");
			return;
		}
		if (!current.set && !password) {
			problem = _("Type the email account's password.");
			return;
		}
		if (!host.trim() || !Number(port)) {
			more = true;
			problem = _('Type the server and port your email provider gives.');
			return;
		}
		busy = true;
		problem = '';
		try {
			await put('/api/mail', {
				host: host.trim(), port: Number(port), security, username: address.trim(),
				...(password ? { password } : {})
			});
			close();
			changed();
		} catch (e) {
			problem = (e as Error).message;
			password = '';
		} finally {
			busy = false;
		}
	}

	untrack(() => {
		if (!host) guess();
	});
</script>

<Sheet title={_("The family's email")} {close}>
	<p class="lead">{_('Sign-in codes are sent from this email account. Use one the family owns; nothing goes through ZOOMRIYA.')}</p>
	<!-- svelte-ignore a11y_autofocus -->
	<input type="email" bind:value={address} oninput={guess} placeholder={_('Email address')} aria-label={_('Email address')}
		autocapitalize="none" spellcheck="false" autofocus />
	<Password bind:value={password} placeholder={current.set ? _('Password (leave empty to keep it)') : _('Password')} enter={save} />
	<p class="note">{_("Many providers want an app password here, made in the email account's security settings.")}</p>
	{#if more}
		<input type="text" bind:value={host} placeholder={_('Server, like smtp.example.com')} aria-label={_('Server')} autocapitalize="none" spellcheck="false" />
		<div class="row-between">
			<input type="text" inputmode="numeric" bind:value={port} placeholder={_('Port')} aria-label={_('Port')} style:width="96px" />
			<Segmented choices={[['ssl', 'SSL/TLS'], ['starttls', 'STARTTLS']]} bind:chosen={security} />
		</div>
	{:else}
		<div><button class="flat" onclick={() => (more = true)}>{_('More settings')}</button></div>
	{/if}
	{#if problem}<p class="error" role="alert">{problem}</p>{/if}
	<div class="row-end">
		{#if busy}<Spinner />{/if}
		<button class="primary" onclick={save} disabled={busy}>{_('Save')}</button>
	</div>
</Sheet>

<script lang="ts">
	// Remote access › Your own name: the family's domain, and the key that
	// lets the box change its records (RFC 2136 with a TSIG key; or the
	// certificate's check through acme-dns). The box then keeps
	// home.<domain> pointing at itself, with a real padlock
	// (zoomriya-connect docs/direct-plus.md §11). The admin's password again.
	import { desk } from '../lib/desk.svelte';
	import { _, fill } from '../lib/i18n.svelte';
	import Sheet from '../components/Sheet.svelte';
	import { deviceWords } from './devices';
	import { setNames } from './remote';

	let { done, close }: { done: () => void; close: () => void } = $props();
	let domain = $state('');
	let server = $state('');
	let zone = $state('');
	let keyName = $state('');
	let algorithm = $state('hmac-sha256');
	let secret = $state('');
	let withAcmeDns = $state(false);
	let acmeUrl = $state('');
	let acmeSub = $state('');
	let acmeUser = $state('');
	let acmePass = $state('');

	const ready = $derived(
		domain.trim().includes('.') && server.trim() && keyName.trim() && secret.trim() &&
			(!withAcmeDns || (acmeUrl.trim() && acmeSub.trim() && acmeUser.trim() && acmePass.trim()))
	);

	function save(): void {
		const d = domain.trim().toLowerCase().replace(/\.$/, '');
		desk.confirm({
			title: _('Use your own name?'),
			text: fill(_('The box keeps home.{domain} pointing at itself and gets its certificate.'), { domain: d }),
			button: _('Use it'),
			action: async (password) => {
				try {
					return await setNames({
						domain: d,
						dns: { server: server.trim(), zone: (zone.trim() || d).toLowerCase(), key_name: keyName.trim(), algorithm, secret: secret.trim() },
						acme_dns: withAcmeDns
							? { url: acmeUrl.trim(), subdomain: acmeSub.trim(), username: acmeUser.trim(), password: acmePass.trim() }
							: null
					}, password);
				} catch (e) {
					throw new Error(deviceWords(e));
				}
			},
			done: () => {
				done();
				close();
			}
		});
	}
</script>

<Sheet title={_('Your own name')} {close} width={560}>
	<p class="lead">{_('With a domain of your own, the box answers at home.your-domain with a real padlock, and guests get share.home.your-domain. Your DNS provider gives the key; ask for one limited to these names.')}</p>
	<label>{_('Your domain')}<input type="text" bind:value={domain} placeholder="smith.ca" autocomplete="off" /></label>
	<label>{_('DNS server that takes updates')}<input type="text" bind:value={server} placeholder="ns1.example.net" autocomplete="off" /></label>
	<label>{_('Zone (if not the domain)')}<input type="text" bind:value={zone} placeholder={domain || 'smith.ca'} autocomplete="off" /></label>
	<label>{_('Key name')}<input type="text" bind:value={keyName} autocomplete="off" /></label>
	<label>{_('Key type')}
		<select bind:value={algorithm}>
			<option value="hmac-sha256">HMAC-SHA256</option>
			<option value="hmac-sha384">HMAC-SHA384</option>
			<option value="hmac-sha512">HMAC-SHA512</option>
		</select>
	</label>
	<label>{_('Key')}<input type="password" bind:value={secret} autocomplete="off" /></label>
	<label class="check"><input type="checkbox" bind:checked={withAcmeDns} /> {_('The certificate check goes through acme-dns')}</label>
	{#if withAcmeDns}
		<label>{_('acme-dns server')}<input type="text" bind:value={acmeUrl} placeholder="https://" autocomplete="off" /></label>
		<label>{_('Subdomain')}<input type="text" bind:value={acmeSub} autocomplete="off" /></label>
		<label>{_('Username')}<input type="text" bind:value={acmeUser} autocomplete="off" /></label>
		<label>{_('Password')}<input type="password" bind:value={acmePass} autocomplete="off" /></label>
	{/if}
	<div class="row-end">
		<button onclick={close}>{_('Cancel')}</button>
		<button class="primary" disabled={!ready} onclick={save}>{_('Use this name')}</button>
	</div>
</Sheet>

<style>
	label { display: flex; flex-direction: column; gap: 4px; margin-top: 10px; }
	label.check { flex-direction: row; align-items: center; gap: 8px; }
</style>

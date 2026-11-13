<script lang="ts">
	// Control Panel › Advanced access (access.py): SSH for the box's admins, by
	// key only, from the home network only; off unless an admin turns it on.
	import { ask, type Answer } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { day } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';
	import KeysFromUsb from './KeysFromUsb.svelte';
	import PasteKey from './PasteKey.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	let access = $state<Answer | null>(null);
	let problem = $state('');
	let busy = $state(false);
	const keys = $derived((access?.keys ?? []) as Answer[]);

	async function load(): Promise<void> {
		try {
			access = await ask('advanced-access');
		} catch (e) {
			problem = (e as Error).message;
		}
	}

	function changed(): void {
		void load();
		desk.refreshWidgets();
	}

	async function turnOff(): Promise<void> {
		busy = true;
		try {
			await ask('advanced-access', { action: 'off' });
			changed();
		} catch (e) {
			problem = (e as Error).message;
		} finally {
			busy = false;
		}
	}

	function turnOn(): void {
		desk.confirm({
			title: _('Turn on advanced access?'), text: _('Keys you add get full control of the box, from your home network.'),
			button: _('Turn on'), action: (password) => ask('advanced-access', { action: 'on' }, password), done: changed
		});
	}

	async function removeKey(key: Answer): Promise<void> {
		try {
			await ask('advanced-access', { action: 'key-remove', fingerprint: key.fingerprint });
			changed();
		} catch (e) {
			problem = (e as Error).message;
		}
	}

	void load();
</script>

<div class="settings">
	{#if access}
		{#if access.on}
			<Group attention>
				<Row big icon="terminal" role="warn" title={_('Advanced access is on')} sub={_('The keys below can sign in over SSH at home.')}>
					{#snippet ends()}<button class="primary" onclick={turnOff} disabled={busy}>{_('Turn off')}</button>{/snippet}
				</Row>
			</Group>
		{:else}
			<Group title={_('Advanced access')} icon="terminal" lead={_('SSH, for admins who want a command line.')}>
				<Row big icon="lock" title={_('Off')} sub={keys.length ? _('No one can sign in over SSH.') : _('Add a key first.')}>
					{#snippet ends()}<button class="primary" disabled={!keys.length} onclick={turnOn}>{_('Turn on')}</button>{/snippet}
				</Row>
			</Group>
		{/if}
		<Group title={_('Keys')} icon="key" note={_('Public keys only (.pub). Never share a private key.')}>
			{#each keys as key (key.fingerprint)}
				<Row icon="key" title={key.name ?? ''}
					sub={`${key.type ?? ''} · ${key.fingerprint ?? ''}\n${fill(_('Added {date}'), { date: day(key.added ?? '') })} · ${key.last_used ? fill(_('Last used {date}'), { date: day(key.last_used, true) }) : _('Not used yet')}`}>
					{#snippet ends()}<button onclick={() => removeKey(key)}>{_('Remove')}</button>{/snippet}
				</Row>
			{/each}
			<Row icon="disk" title={_('Add from a USB stick')} sub={_('Put the .pub file in zoomriya/ssh on the stick.')}>
				{#snippet ends()}<button onclick={() => desk.sheet(KeysFromUsb, { done: changed })}>{_('Look on USB stick')}</button>{/snippet}
			</Row>
			<Row icon="keyboard" title={_('Paste a key')} sub={_("From the computer you'll use.")}>
				{#snippet ends()}<button onclick={() => desk.sheet(PasteKey, { done: changed })}>{_('Paste a key')}</button>{/snippet}
			</Row>
		</Group>
		{#if access.host_key || access.address}
			<Group title={_('Connecting')} icon="info">
				{#if access.address}
					<Row icon="terminal" title={_('Command')}>
						{#snippet ends()}<span class="row-value selectable">ssh {access?.user ?? 'admin'}@{access?.address}</span>{/snippet}
					</Row>
				{/if}
				{#if access.host_key}<Row icon="shield" title={_("The box's key")} sub={access.host_key + '\n' + _('Check it the first time you connect.')} />{/if}
			</Group>
		{/if}
		<Group title={_('Good to know')} icon="alert">
			<Row icon="maintenance" title={_('Changes by hand are yours')} sub={_('Updates and restores may undo them.')} />
			<Row icon="router" title={_('Home network only')} sub={_('Never from the internet or ZOOMRIYA Connect.')} />
			<Row icon="reset" title={_('Reset removes the keys')} sub={_('Resetting the box turns this off and removes the keys.')} />
		</Group>
	{/if}
	{#if problem}<p class="error">{problem}</p>{/if}
</div>

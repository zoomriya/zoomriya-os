<script lang="ts">
	// Control Panel › Sign-in codes (ZOOMRIYA ID's app.second_step and
	// app.mail): a code by email after the password, a choice for each person,
	// admins too. The family's own email account sends the codes; nothing goes
	// through ZOOMRIYA, and there are no text messages.
	import { del, get, post, type Answer } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { _, fill } from '../lib/i18n.svelte';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';
	import FamilyEmail from './FamilyEmail.svelte';
	import MyCodes from './MyCodes.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	let mail = $state<Answer | null>(null);
	let mine = $state<Answer | null>(null);
	let problem = $state('');
	let note = $state('');
	let busy = $state(false);

	async function load(): Promise<void> {
		try {
			[mail, mine] = await Promise.all([get<Answer>('/api/mail'), get<Answer>('/api/me/second-step')]);
		} catch (e) {
			problem = (e as Error).message;
		}
	}

	async function test(): Promise<void> {
		const to = String(mine?.email || mail?.username || '');
		busy = true;
		problem = note = '';
		try {
			await post('/api/mail/test', { to });
			note = fill(_('A test email is on its way to {address}.'), { address: to });
		} catch (e) {
			problem = (e as Error).message;
		} finally {
			busy = false;
		}
	}

	function removeEmail(): void {
		const using = Number(mail?.in_use ?? 0);
		desk.confirm({
			title: _("Remove the family's email?"),
			text: using
				? fill(_('Sign-in codes by email turn off for everyone who uses them ({count}).'), { count: using })
				: _("Codes can't be sent until an email account is set again."),
			button: _('Remove'),
			action: () => del('/api/mail'),
			done: () => void load()
		});
	}

	function turnOff(): void {
		desk.confirm({
			title: _('Turn off sign-in codes?'),
			text: _('You sign in with your password only.'),
			button: _('Turn off'),
			action: (password) => post('/api/me/second-step/off', { password }),
			done: () => void load()
		});
	}

	void load();
</script>

<div class="settings">
	{#if mail && mine}
		<Group title={_('Your sign-in')} icon="key" lead={_("A code by email after your password. Each person chooses; it's never required.")}>
			{#if mine.on}
				<Row big icon="check" role="ok" title={_('Codes by email are on')} sub={fill(_('Sent to {address}'), { address: mine.email })}>
					{#snippet ends()}
						<button onclick={() => desk.sheet(MyCodes, { address: mine!.email, changed: load })}>{_('Change address')}</button>
						<button onclick={turnOff}>{_('Turn off')}</button>
					{/snippet}
				</Row>
			{:else}
				<Row big icon="lock" role="muted" title={_('Codes by email are off')}
					sub={mail.set ? _('You sign in with your password only.') : _("Set the family's email below first.")}>
					{#snippet ends()}
						<button class="primary" disabled={!mail!.set} onclick={() => desk.sheet(MyCodes, { changed: load })}>{_('Turn on')}</button>
					{/snippet}
				</Row>
			{/if}
		</Group>
		<Group title={_("The family's email")} icon="send"
			note={_('Codes go out from an email account the family owns. Nothing goes through ZOOMRIYA.')}>
			{#if mail.set}
				<Row icon="send" title={String(mail.username)} sub={fill(_('Server {host}, port {port}'), { host: mail.host, port: mail.port })}>
					{#snippet ends()}
						<button onclick={test} disabled={busy}>{_('Send a test')}</button>
						<button onclick={() => desk.sheet(FamilyEmail, { current: mail, changed: load })}>{_('Change')}</button>
						<button onclick={removeEmail}>{_('Remove')}</button>
					{/snippet}
				</Row>
				<Row icon="user" title={_('People using codes by email')}>
					{#snippet ends()}<span class="row-value">{mail!.in_use}</span>{/snippet}
				</Row>
			{:else}
				<Row big icon="send" role="muted" title={_('Not set')} sub={_('Needed before anyone can turn on sign-in codes.')}>
					{#snippet ends()}
						<button class="primary" onclick={() => desk.sheet(FamilyEmail, { current: mail, changed: load })}>{_('Set up')}</button>
					{/snippet}
				</Row>
			{/if}
		</Group>
	{/if}
	{#if note}<p class="lead">{note}</p>{/if}
	{#if problem}<p class="error">{problem}</p>{/if}
</div>

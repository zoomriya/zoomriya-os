<script lang="ts">
	// Edit someone (people.py's edit_person): their name (and a child's year
	// and guardians), a new password, making or removing an admin, and turning
	// their account off or on. The risky ones ask for the password again.
	import { untrack } from 'svelte';
	import { patch } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { _, fill } from '../lib/i18n.svelte';
	import { shownName, type Person } from '../lib/people';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';
	import Sheet from '../components/Sheet.svelte';
	import Wide from '../components/Wide.svelte';
	import ChildFields from './ChildFields.svelte';
	import NewPassword from './NewPassword.svelte';

	let { person, everyone, changed, close }: { person: Person; everyone: Person[]; changed: () => void; close: () => void } = $props();
	const who = $derived(shownName(person));
	const isMe = $derived(person.id === desk.member?.id);
	const isChild = $derived(person.kind === 'child');
	const active = $derived(person.is_active !== false);
	const admin = $derived(person.role === 'admin');
	// What's typed starts from what they have now.
	let name = $state(untrack(() => person.display_name ?? ''));
	let year = $state(untrack(() => person.birth_year ?? new Date().getFullYear() - 10));
	let guardians = $state<number[]>(untrack(() => (person.guardians ?? []).map((g: Person) => g.id)));
	let problem = $state('');
	let busy = $state(false);

	const change = (fields: Record<string, unknown>, password?: string) =>
		patch(`/api/people/${person.id}`, password === undefined ? { fields } : { fields, password });

	async function saveDetails(): Promise<void> {
		const fields: Record<string, unknown> = { display_name: name.trim() || null };
		if (isChild) {
			fields.birth_year = Number(year);
			fields.guardian_ids = guardians;
			if (!guardians.length) {
				problem = _('Choose a parent or guardian.');
				return;
			}
		}
		busy = true;
		try {
			await change(fields);
			close();
			changed();
		} catch (e) {
			problem = (e as Error).message;
		} finally {
			busy = false;
		}
	}

	function risky(title: string, text: string, button: string, fields: Record<string, unknown>): void {
		close();
		desk.confirm({ title, text, button, action: (password) => change(fields, password), done: changed });
	}

	async function turnOn(): Promise<void> {
		busy = true;
		try {
			await change({ is_active: true });
			close();
			changed();
		} catch (e) {
			problem = (e as Error).message;
			busy = false;
		}
	}
</script>

<Sheet title={who} {close}>
	<Group>
		<Row icon="user" title={_('Name')} sub={'@' + (person.username ?? '')}>
			{#snippet ends()}<input type="text" style:width="240px" bind:value={name} aria-label={_('Name')} onkeydown={(e) => e.key === 'Enter' && saveDetails()} />{/snippet}
		</Row>
		{#if isChild}<Wide><ChildFields {everyone} {person} bind:year bind:guardians /></Wide>{/if}
		<Wide>
			{#if problem}<p class="error">{problem}</p>{/if}
			<div class="row-end"><button class="primary" onclick={saveDetails} disabled={busy}>{_('Save')}</button></div>
		</Wide>
	</Group>
	<Group>
		<Row icon="key" title={_('Password')} sub={isMe ? _('Change your own password.') : fill(_('Forgotten? {name} types a new one here.'), { name: who })}>
			{#snippet ends()}<button onclick={() => { close(); desk.sheet(NewPassword, { person, changed }); }}>{_('New password')}</button>{/snippet}
		</Row>
		{#if !isMe && !isChild}
			{#if admin}
				<Row icon="shield" title={_('Admin')} sub={_("Can manage the box and everyone's accounts.")}>
					{#snippet ends()}
						<button onclick={() => risky(fill(_('Remove {name} as admin?'), { name: who }),
							_("They keep their account but can no longer manage the box. They're signed out everywhere."),
							_('Remove admin'), { role: 'family' })}>{_('Remove admin')}</button>
					{/snippet}
				</Row>
			{:else if active}
				<Row icon="shield" title={_('Admin')} sub={_("Admins manage the box and everyone's accounts.")}>
					{#snippet ends()}
						<button onclick={() => risky(fill(_('Make {name} an admin?'), { name: who }),
							_("They can manage the box and everyone's accounts, here and in Family Hub."),
							_('Make admin'), { role: 'admin' })}>{_('Make admin')}</button>
					{/snippet}
				</Row>
			{/if}
		{/if}
		{#if !isMe}
			{#if active}
				<Row icon="power" title={_('Account')} sub={_('On. They can sign in.')}>
					{#snippet ends()}
						<button onclick={() => risky(fill(_("Turn off {name}'s account?"), { name: who }),
							_("They're signed out everywhere and can't sign in. Their photos and files stay. You can turn it on again."),
							_('Turn off'), { is_active: false })}>{_('Turn off')}</button>
					{/snippet}
				</Row>
			{:else}
				<Row icon="power" title={_('Account')} sub={_("Off. They can't sign in.")}>
					{#snippet ends()}<button class="primary" onclick={turnOn} disabled={busy}>{_('Turn on')}</button>{/snippet}
				</Row>
			{/if}
		{/if}
	</Group>
</Sheet>

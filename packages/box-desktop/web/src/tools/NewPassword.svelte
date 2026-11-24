<script lang="ts">
	// A new password (people.py's new_password): they type it themselves; your
	// own needs your current one first (box-desktopd checks it).
	import { patch } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { _, fill } from '../lib/i18n.svelte';
	import { passwordProblem, shownName, type Person } from '../lib/people';
	import Password from '../components/Password.svelte';
	import Sheet from '../components/Sheet.svelte';

	let { person, changed, close }: { person: Person; changed: () => void; close: () => void } = $props();
	const who = $derived(shownName(person));
	const isMe = $derived(person.id === desk.member?.id);
	let current = $state('');
	let first = $state('');
	let again = $state('');
	let problem = $state('');
	let busy = $state(false);

	async function go(): Promise<void> {
		problem = passwordProblem(first, again);
		if (problem) {
			current = first = again = '';
			return;
		}
		busy = true;
		try {
			const fields = { password: first, temporary: false };
			await patch(`/api/people/${person.id}`, isMe ? { fields, password: current } : { fields });
			close();
			changed();
		} catch (e) {
			problem = (e as Error).message;
			current = first = again = '';
		} finally {
			busy = false;
		}
	}
</script>

<Sheet title={fill(_('New password for {name}'), { name: who })} {close}>
	<p class="lead">{isMe ? _('Type a new password. At least 8 characters.')
		: fill(_("Let {name} type a new one. At least 8 characters. They're signed out everywhere."), { name: who })}</p>
	{#if isMe}<Password bind:value={current} placeholder={_('Your current password')} autofocus />{/if}
	<Password bind:value={first} placeholder={_('Password')} autofocus={!isMe} autocomplete="new-password" />
	<Password bind:value={again} placeholder={_('Password again')} enter={go} autocomplete="new-password" />
	{#if problem}<p class="error">{problem}</p>{/if}
	<div class="row-end"><button class="primary" onclick={go} disabled={busy}>{_('Save password')}</button></div>
</Sheet>

<script lang="ts">
	// Add a person (people.py's add_person): their name and kind first; then
	// they type their own password, so it's theirs from the start.
	import { post } from '../lib/api';
	import { _, fill } from '../lib/i18n.svelte';
	import { detailsProblem, passwordProblem, type Person } from '../lib/people';
	import Password from '../components/Password.svelte';
	import Segmented from '../components/Segmented.svelte';
	import Sheet from '../components/Sheet.svelte';
	import ChildFields from './ChildFields.svelte';

	let { everyone, changed, close }: { everyone: Person[]; changed: () => void; close: () => void } = $props();
	let name = $state('');
	let username = $state('');
	let kind = $state('adult');
	let year = $state(new Date().getFullYear() - 10);
	let guardians = $state<number[]>([]);
	let problem = $state('');
	let secret = $state(false);
	let first = $state('');
	let again = $state('');
	let busy = $state(false);

	function toPassword(): void {
		problem = detailsProblem(name, username, kind === 'child', guardians);
		if (problem) return;
		secret = true;
	}

	async function create(): Promise<void> {
		problem = passwordProblem(first, again);
		if (problem) return;
		const fields: Record<string, unknown> = {
			username: username.trim(), display_name: name.trim(), password: first, kind,
			// They typed it themselves: it's theirs, not temporary.
			temporary: false
		};
		if (kind === 'child') {
			fields.birth_year = Number(year);
			fields.guardian_ids = guardians;
		}
		busy = true;
		try {
			await post('/api/people', fields);
			close();
			changed();
		} catch (e) {
			first = again = '';
			problem = (e as Error).message;
		} finally {
			busy = false;
		}
	}
</script>

<Sheet title={_('Add a person')} {close}>
	{#if !secret}
		<!-- svelte-ignore a11y_autofocus -->
		<input type="text" bind:value={name} placeholder={_('Name, like Priya')} aria-label={_('Name, like Priya')} autofocus />
		<input type="text" bind:value={username} placeholder={_('Username, like priya')} aria-label={_('Username, like priya')} autocapitalize="none"
			onkeydown={(e) => e.key === 'Enter' && toPassword()} />
		<div class="row-between">
			<span class="row-title grow">{_('Adult or child')}</span>
			<Segmented choices={[['adult', _('Adult')], ['child', _('Under 18')]]} bind:chosen={kind} />
		</div>
		{#if kind === 'child'}<ChildFields {everyone} bind:year bind:guardians />{/if}
		{#if problem}<p class="error">{problem}</p>{/if}
		<div class="row-end"><button class="primary" onclick={toPassword}>{_('Next')}</button></div>
	{:else}
		<p class="lead">{fill(_('Now let {name} type their own password. At least 8 characters.'), { name: name.trim() })}</p>
		<Password bind:value={first} placeholder={_('Password')} autofocus autocomplete="new-password" />
		<Password bind:value={again} placeholder={_('Password again')} enter={create} autocomplete="new-password" />
		{#if problem}<p class="error">{problem}</p>{/if}
		<div class="row-end">
			<button onclick={() => { secret = false; problem = ''; }}>{_('Back')}</button>
			<button class="primary" onclick={create} disabled={busy}>{_('Add')}</button>
		</div>
	{/if}
</Sheet>

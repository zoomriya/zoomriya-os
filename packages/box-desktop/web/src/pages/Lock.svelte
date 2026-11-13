<script lang="ts">
	// The locked screen (app.py's lock_page): the box's background, the time,
	// and the admin's password to come back. They stay signed in, their
	// windows as they were, and the box goes on as before.
	import { onDestroy } from 'svelte';
	import { desk } from '../lib/desk.svelte';
	import { clockTime, initials, longDate } from '../lib/format';
	import { _ } from '../lib/i18n.svelte';
	import Password from '../components/Password.svelte';
	import Spinner from '../components/Spinner.svelte';
	import Backdrop from './Backdrop.svelte';

	let now = $state(new Date());
	const timer = setInterval(() => (now = new Date()), 1000);
	onDestroy(() => clearInterval(timer));
	let password = $state('');
	let busy = $state(false);
	let problem = $state('');

	async function unlock(): Promise<void> {
		if (!password || busy) return;
		busy = true;
		problem = '';
		try {
			await desk.unlock(password);
		} catch (e) {
			problem = (e as Error).message;
		} finally {
			password = '';
			busy = false;
		}
	}
</script>

<Backdrop />
<div class="lock-shade"></div>
<div class="lock-column">
	<div class="lock-time">{clockTime(now)}</div>
	<div class="lock-date">{longDate(now)}</div>
	<form class="glass-card lock-card" onsubmit={(e) => { e.preventDefault(); unlock(); }}>
		<span class="avatar lock-avatar">{initials(desk.member?.name ?? '')}</span>
		<div class="lock-who">{desk.member?.name}</div>
		<Password bind:value={password} placeholder={_('Password')} disabled={busy} autofocus enter={unlock} />
		{#if problem}<p class="error" role="alert">{problem}</p>{/if}
		<div class="row-between">
			<!-- Switch account: signs this admin out. -->
			<button type="button" class="flat" onclick={() => desk.signOut()}>{_('Switch account')}</button>
			<span class="grow"></span>
			{#if busy}<Spinner />{/if}
			<button type="submit" class="primary" disabled={busy}>{_('Unlock')}</button>
		</div>
	</form>
</div>

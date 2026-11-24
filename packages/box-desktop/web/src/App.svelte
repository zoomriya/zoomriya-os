<script lang="ts">
	// Box Desktop: the sign-in, the desktop, or the lock screen, in the
	// admin's theme and accent (Midnight and the apps' blue before anyone
	// signs in).
	import { onMount } from 'svelte';
	import { desk } from './lib/desk.svelte';
	import { applyLook } from './lib/look';
	import Desktop from './pages/Desktop.svelte';
	import Lock from './pages/Lock.svelte';
	import Setup from './pages/Setup.svelte';
	import SetupElsewhere from './pages/SetupElsewhere.svelte';
	import SignIn from './pages/SignIn.svelte';

	onMount(() => {
		void desk.start();
	});
	$effect(() => applyLook(desk.theme, desk.accent));
</script>

{#if desk.page === 'setup'}
	<Setup />
{:else if desk.page === 'setup-elsewhere'}
	<SetupElsewhere />
{:else if desk.page === 'sign-in'}
	<SignIn />
{:else if desk.page === 'desktop' || desk.page === 'lock'}
	<!-- Kept while locked: the windows stay as they were. -->
	<div hidden={desk.page === 'lock'}><Desktop /></div>
	{#if desk.page === 'lock'}<Lock />{/if}
{/if}

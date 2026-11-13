<script lang="ts">
	// "Your password again" (tools.py's confirm): a sheet over everything, the
	// admin's password (and, with extra, one more field, such as the setup
	// code), then the action. box-desktopd checks the password, not the page.
	import { desk, type Confirming } from '../lib/desk.svelte';
	import { _ } from '../lib/i18n.svelte';
	import Password from './Password.svelte';

	let shown = $state<Confirming | null>(null);
	let password = $state('');
	let extra = $state('');
	let problem = $state('');
	let busy = $state(false);

	desk.confirming = (c) => {
		shown = c;
		password = extra = problem = '';
		busy = false;
	};

	export function close(): boolean {
		if (!shown) return false;
		shown = null;
		return true;
	}

	async function run(): Promise<void> {
		if (!shown || busy) return;
		busy = true;
		problem = '';
		const c = shown;
		try {
			const answer = await c.action(password, extra.trim());
			shown = null;
			c.done?.(answer);
		} catch (e) {
			problem = (e as Error).message;
			password = '';
		} finally {
			busy = false;
		}
	}
</script>

{#if shown}
	<div class="shade" role="dialog" aria-modal="true" tabindex="-1">
		<form class="card" style:width="440px" style:max-width="100%" aria-label={shown.title}
			onsubmit={(e) => { e.preventDefault(); run(); }}>
			<h2 class="title">{shown.title}</h2>
			<p class="lead">{shown.text}</p>
			{#if shown.extra}
				<!-- svelte-ignore a11y_autofocus -->
				<input type="text" bind:value={extra} placeholder={shown.extra} aria-label={shown.extra} autofocus />
			{/if}
			<Password bind:value={password} placeholder={_('Your password')} autofocus={!shown.extra} enter={run} />
			{#if problem}<p class="error" role="alert">{problem}</p>{/if}
			<div class="row-end">
				<button type="button" onclick={() => (shown = null)}>{_('Cancel')}</button>
				<button type="submit" class="danger" disabled={busy}>{shown.button}</button>
			</div>
		</form>
	</div>
{/if}

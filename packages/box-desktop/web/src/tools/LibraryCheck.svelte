<script lang="ts">
	// The library's check (tools.py's library_check, a Btrfs scrub): start,
	// pause, resume. With two drives it repairs one copy from the other; with
	// one it finds damage only.
	import { onDestroy } from 'svelte';
	import { ask, type Answer } from '../lib/api';
	import { when } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import Meter from '../components/Meter.svelte';

	let { check, mirror }: { check: Answer; mirror: boolean } = $props();
	let now = $state<Answer>({});
	$effect.pre(() => {
		now = check;
	});
	let busy = $state(false);
	let problem = $state('');
	const [doIt, doing] = $derived(mirror ? [_('Check that both copies match'), _('Checking that both copies match')]
		: [_('Check the library for damage'), _('Checking the library for damage')]);
	const running = $derived(!!now.running && !now.paused);
	const action = $derived(running ? 'pause' : now.paused ? 'resume' : 'start');
	const percent = $derived(Number(now.percent ?? 0));
	const last = $derived(
		now.never_run ? _("It hasn't been checked yet.")
			: now.errors && mirror ? fill(_('Last check found {n} problems and repaired what it could.'), { n: now.errors })
			: now.errors ? fill(_("Last check found {n} damaged files. With one drive they can't be repaired: restore them from a backup."), { n: now.errors })
			: _('The last check found nothing needing repair.')
	);

	async function refresh(): Promise<void> {
		try {
			now = (await ask('storage')).library?.check ?? {};
		} catch {
			// As it was.
		}
	}

	async function act(): Promise<void> {
		busy = true;
		try {
			await ask('storage-check', { action });
			await refresh();
		} catch (e) {
			problem = (e as Error).message;
		} finally {
			busy = false;
		}
	}

	const timer = setInterval(refresh, 10000);
	onDestroy(() => clearInterval(timer));
</script>

<div class="panel-box">
	<div class="grow" style:flex="1" style:display="flex" style:flex-direction="column" style:gap="4px">
		{#if running}
			<span class="fact">{doing} · {percent.toFixed(0)}%</span>
			<span class="fact-name">{fill(_('Started {time}.'), { time: when(now.started ?? '') })}</span>
			<Meter fraction={percent / 100} kind="library" />
		{:else if now.paused}
			<span class="fact">{fill(_('The check is paused at {percent}%'), { percent: percent.toFixed(0) })}</span>
			<span class="fact-name">{_('It carries on from there when you resume it.')}</span>
		{:else}
			<span class="fact">{doIt}</span>
			<span class="fact-name">{last + ' ' + _('The box also checks once a month by itself.')}</span>
		{/if}
		{#if problem}<p class="error">{problem}</p>{/if}
	</div>
	<button onclick={act} disabled={busy}>{running ? _('Pause') : now.paused ? _('Resume') : _('Check now')}</button>
</div>

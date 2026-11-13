<script lang="ts">
	// Storage Manager › Backups (backups.py): when the last one ran and how it
	// went, the next nightly one, where they go, Back up now, and the nightly
	// backup on or off. Setting backups up stays in Family Hub's settings.
	import { onDestroy } from 'svelte';
	import { ask, type Answer } from '../lib/api';
	import { when } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';
	import Spinner from '../components/Spinner.svelte';
	import StatusTile from '../components/StatusTile.svelte';
	import Switch from '../components/Switch.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	/** How often an open page asks again while a backup runs. */
	const RUNNING_REFRESH_SECONDS = 5;
	let answer = $state<Answer | null>(null);
	let problem = $state('');
	let timer: ReturnType<typeof setInterval> | null = null;
	onDestroy(() => timer && clearInterval(timer));

	function got(a: Answer): void {
		answer = a;
		if (a.running && !timer) {
			timer = setInterval(() => ask('backups').then(got).catch(() => {}), RUNNING_REFRESH_SECONDS * 1000);
		} else if (!a.running && timer) {
			clearInterval(timer);
			timer = null;
		}
	}

	const act = (fields: Answer) => ask('backups', fields).then(got).catch((e) => (problem = e.message));
	ask('backups').then(got).catch((e) => (problem = e.message));
	const ok = $derived(answer?.last_ok);
</script>

<div class="settings">
	{#if answer && !answer.set_up}
		<Group title={_('Backups')} icon="backup">
			<Row big icon="alert" role="warn" title={_("Backups aren't set up yet")} sub={_("Set them up in Family Hub's settings: where they go, and which apps.")} />
		</Group>
	{:else if answer}
		<div class="tiles">
			{#if answer.running}
				<StatusTile icon="backup" title={_('Backing up')} words={fill(_('Started {time}'), { time: when(answer.last_start ?? '') })} state="plain" />
			{:else if ok == null}
				<StatusTile icon="backup" title={_('No backup yet')} words={_('Back up now to make the first.')} state="warn" />
			{:else if ok}
				<StatusTile icon="check" title={_('Backed up')} words={when(answer.last_end ?? '')} state="ok" />
			{:else}
				<StatusTile icon="alert" title={_('The last backup failed')} words={when(answer.last_end ?? '')} state="warn" />
			{/if}
		</div>
		<Group title={_('Backups')} icon="backup">
			<Row big icon="backup" title={_('Back up now')} sub={_("Every app's data and the accounts, as they are now.")}>
				{#snippet ends()}
					{#if answer?.running}<Spinner />{/if}
					<button class="primary" disabled={!!answer?.running} onclick={() => act({ action: 'now' })}>{_('Back up now')}</button>
				{/snippet}
			</Row>
			<Row big icon="calendar-clock" title={_('Nightly backup')}
				sub={answer.next ? fill(_('Next: {time}'), { time: when(answer.next) }) : _('Every night at 4:00, while the family sleeps.')}>
				{#snippet ends()}<Switch on={!!answer?.nightly} changed={(on) => act({ action: 'nightly', on })} label={_('Nightly backup')} />{/snippet}
			</Row>
			<Row big icon="disk" title={_('Where they go')} sub={_("Keep them on another drive or computer than the box's.")}>
				{#snippet ends()}<span class="row-value">{answer?.where || _('Not known')}</span>{/snippet}
			</Row>
		</Group>
		{#if (answer.problems ?? []).length}
			<Group title={_('What went wrong last time')} icon="alert" note={_('The other apps were backed up. Older backups are kept.')}>
				{#each answer.problems as p, i (i)}<Row icon="alert" role="warn" title={p} />{/each}
			</Group>
		{/if}
	{/if}
	{#if problem}<p class="error">{problem}</p>{/if}
</div>

<script lang="ts">
	// Log Center's logs (tools.py's logs): box events, admin actions,
	// sign-ins or the system log (with its level), searched, newest first,
	// with Show older.
	import { ask, type Answer } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { when } from '../lib/format';
	import { _ } from '../lib/i18n.svelte';
	import { LOG_LEVELS } from '../lib/words';
	import Dropdown from '../components/Dropdown.svelte';

	let { kind, shown = true }: { kind: string; shown?: boolean } = $props();
	let level = $state(1);
	let search = $state('');
	let entries = $state<Answer[] | null>(null);
	let next = $state('');
	let problem = $state('');
	let asked = 0;

	// From the top bar's search: the words to look for.
	$effect(() => {
		if (kind === 'system' && desk.logSearch) {
			search = desk.logSearch;
			desk.logSearch = '';
		}
	});

	async function load(more = false): Promise<void> {
		const fields: Record<string, unknown> = { kind, search: search || undefined };
		if (kind === 'system') fields.priority = LOG_LEVELS[level][0];
		if (more) fields.cursor = next;
		const mine = ++asked;
		try {
			const answer = await ask('logs', fields);
			if (mine !== asked) return;
			const shownNow = answer.entries ?? [];
			entries = more ? [...(entries ?? []), ...shownNow] : shownNow;
			next = answer.more ? answer.next ?? '' : '';
			problem = '';
		} catch (e) {
			problem = (e as Error).message;
		}
	}

	$effect(() => {
		void level;
		void search;
		void load();
	});
</script>

<div style:display="flex" style:flex-direction="column" style:gap="10px">
	<div class="row-between">
		{#if kind === 'system'}<Dropdown names={LOG_LEVELS.map(([, n]) => _(n))} bind:selected={level} label={_('Everything')} />{/if}
		<input type="search" class="grow" bind:value={search} placeholder={_('Search')} aria-label={_('Search')} />
	</div>
	{#if problem}<p class="error">{problem}</p>{/if}
	{#if entries}
		<div class="list">
			{#if !entries.length}<p class="lead">{_('Nothing here.')}</p>{/if}
			{#each entries as entry, i (i)}
				<div>
					<span class="fact-name {entry.level ?? ''}">{when(entry.time ?? '')}  ·  {entry.source ?? ''}{entry.by ? `  ·  ${entry.by}` : ''}</span>
					<span class="log-line selectable">{entry.message ?? ''}</span>
				</div>
			{/each}
		</div>
		{#if next}<div style:text-align="center"><button onclick={() => load(true)}>{_('Show older')}</button></div>{/if}
	{/if}
</div>

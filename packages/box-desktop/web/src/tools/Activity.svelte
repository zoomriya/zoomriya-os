<script lang="ts">
	// Log Center › Family activity (tools.py's FamilyActivity): ZOOMRIYA ID's
	// list, the same as Family Hub's Activity, 90 days, newest first.
	import { get, type Answer } from '../lib/api';
	import { when } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import { ACTIVITY_KINDS } from '../lib/words';

	let { shown = true }: { shown?: boolean } = $props();
	let lines = $state<Answer[] | null>(null);
	let more = $state(false);
	let problem = $state('');
	let lastId: number | null = null;

	async function load(): Promise<void> {
		try {
			const answer = await get<Answer>('/api/people/activity' + (lastId ? `?before=${lastId}` : ''));
			const got = answer.activity ?? [];
			lines = [...(lines ?? []), ...got];
			if (got.length) lastId = got[got.length - 1].id ?? lastId;
			more = !!answer.more;
		} catch (e) {
			problem = (e as Error).message;
		}
	}

	void load();
</script>

<div style:display="flex" style:flex-direction="column" style:gap="10px">
	<p class="lead">{_("Who signed in to Family Hub and the apps, new devices, share links and changes to the family's accounts, for the last 90 days. The box's own sign-ins and admin actions are in their own sections.")}</p>
	{#if problem}<p class="error">{problem}</p>{/if}
	{#if lines}
		<div class="list">
			{#if !lines.length}<p class="lead">{_('Nothing yet.')}</p>{/if}
			{#each lines as line, i (i)}
				{@const extra = [line.source ?? '', line.by ? fill(_('by {name}'), { name: line.by }) : '', line.address ? fill(_('from {address}'), { address: line.address }) : ''].filter(Boolean).join('  ·  ')}
				<div>
					<span class="fact-name" class:warning={line.kind === 'sign-in-failed'}>{when(line.at ?? '')}  ·  {_(ACTIVITY_KINDS[line.kind ?? ''] ?? line.kind ?? '')}</span>
					<span class="fact">{line.member || _('Someone')}: {line.what ?? ''}</span>
					{#if extra}<span class="dim">{extra}</span>{/if}
				</div>
			{/each}
		</div>
		{#if more}<div><button class="flat" onclick={load}>{_('Show older')}</button></div>{/if}
	{/if}
</div>

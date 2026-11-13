<script lang="ts">
	// Log Center › Notifications (tools.py's log_notifications): the last
	// month's, newest first, each with its kind, the turned-off ones included.
	import { ask, type Answer } from '../lib/api';
	import { when } from '../lib/format';
	import { _ } from '../lib/i18n.svelte';
	import { KINDS } from '../lib/words';

	let { shown = true }: { shown?: boolean } = $props();
	let listed = $state<Answer[] | null>(null);
	let problem = $state('');
	ask('notifications').then((a) => (listed = [...(a.notifications ?? [])].sort((x, y) => (y.time ?? '').localeCompare(x.time ?? ''))))
		.catch((e) => (problem = e.message));
	const cap = (k: string) => k.charAt(0).toUpperCase() + k.slice(1);
</script>

<div style:display="flex" style:flex-direction="column" style:gap="10px">
	<p class="lead">{_('What the box reported in the last month. Choose what appears on the bell in Control Panel › Notifications.')}</p>
	{#if problem}<p class="error">{problem}</p>{/if}
	{#if listed}
		<div class="list">
			{#if !listed.length}<p class="lead">{_('Nothing in the last month.')}</p>{/if}
			{#each listed as n, i (i)}
				<div>
					<span class="fact-name {n.level ?? ''}">{when(n.time ?? '')}  ·  {_((KINDS[n.kind ?? ''] ?? [cap(n.kind ?? '')])[0])}</span>
					<span class="fact">{n.message ?? ''}</span>
				</div>
			{/each}
		</div>
	{/if}
</div>

<script lang="ts">
	// Support Center › Help (support.py's support_help): common questions, each with the tool that answers it.
	import { desk, TOOLS } from '../lib/desk.svelte';
	import { _, fill } from '../lib/i18n.svelte';
	import { SUPPORT_URL } from '../lib/support';
	import { HELP } from '../lib/words';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';

	let { shown = true }: { shown?: boolean } = $props();
</script>

<div class="settings">
	<Group title={_('Common questions')} icon="help" lead={fill(_('More at {url}.'), { url: SUPPORT_URL })}>
		{#each HELP as [question, answer, kind, section] (question)}
			<Row icon="help" title={_(question)} sub={_(answer)}>
				{#snippet ends()}<button onclick={() => desk.openTool(kind, section)}>{_(TOOLS[kind][0])}</button>{/snippet}
			</Row>
		{/each}
	</Group>
</div>

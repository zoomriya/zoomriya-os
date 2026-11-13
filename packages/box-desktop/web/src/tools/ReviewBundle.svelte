<script lang="ts">
	// What's in a bundle (support.py's review_bundle): the admin's note, the
	// checks, and each part, which can be read, before it leaves the box.
	import { onMount } from 'svelte';
	import { ask, type Answer } from '../lib/api';
	import { size } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';
	import Sheet from '../components/Sheet.svelte';

	let { name, close }: { name: string; close: () => void } = $props();
	let manifest = $state<Answer | null>(null);
	let reading = $state<{ part: string; text: string; cut: boolean } | null>(null);
	let problem = $state('');

	onMount(() => {
		ask('diagnostics', { action: 'show', name }).then((a) => (manifest = a.manifest ?? {})).catch((e) => (problem = e.message));
	});

	async function read(part: string): Promise<void> {
		try {
			const a = await ask('diagnostics', { action: 'read', name, part });
			reading = { part, text: a.text ?? '', cut: !!a.cut };
		} catch (e) {
			problem = (e as Error).message;
		}
	}
</script>

<Sheet title={_("What's in this bundle")} width={680} {close}>
	<div style:max-height="560px" style:overflow-y="auto" style:display="flex" style:flex-direction="column" style:gap="16px">
		{#if problem}<p class="error">{problem}</p>{/if}
		{#if reading}
			<div><button onclick={() => (reading = null)}>{_('Back')}</button></div>
			<span class="row-title">{reading.part}</span>
			<pre class="bundle-text">{reading.text}</pre>
			{#if reading.cut}<span class="group-note">{_('Only the start is shown.')}</span>{/if}
		{:else if manifest}
			{#if manifest.note}<p class="lead">{fill(_('Your note: {note}'), { note: manifest.note })}</p>{/if}
			<Group title={_('What the box found')}>
				{#each manifest.checks ?? [] as check, i (i)}
					{@const bad = check.severity === 'error' || check.severity === 'warning'}
					<Row icon={bad ? 'alert' : 'info'} role={bad ? 'warn' : 'accent'} title={check.summary ?? ''} />
				{/each}
				{#if !(manifest.checks ?? []).length}<Row icon="check" title={_('No problems found')} />{/if}
			</Group>
			<Group title={_('Files')} note={_('Passwords and keys are taken out.')}>
				{#each manifest.parts ?? [] as part (part.file)}
					<Row title={part.file ?? ''} sub={`${part.what ?? ''} · ${size(part.bytes ?? 0)}`} tag={part.sensitive ? _('about your home') : ''}>
						{#snippet ends()}<button onclick={() => read(part.file)}>{_('View')}</button>{/snippet}
					</Row>
				{/each}
			</Group>
		{/if}
	</div>
</Sheet>

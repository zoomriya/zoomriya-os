<script lang="ts">
	// Support Center › Contact us (support.py's support_contact).
	import { ask, type Answer } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { _ } from '../lib/i18n.svelte';
	import { SUPPORT_URL, updatesUntil } from '../lib/support';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';
	import Wide from '../components/Wide.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	let status = $state<Answer | null>(null);
	let problem = $state('');
	ask('status').then((s) => (status = s)).catch((e) => (problem = e.message));
	const facts = $derived(status ? ([
		[_('Model'), status.model ?? '', 'box'], [_('Serial number'), status.serial ?? '', 'barcode'],
		[_('ZOOMRIYA OS'), [status.os ?? '', status.image_version ?? ''].filter(Boolean).join(' '), 'package'],
		[_("The box's address"), desk.boxName, 'address']
	] as [string, string, string][]).filter(([, v]) => v) : []);
</script>

<div class="settings">
	<Group title={_('Contact ZOOMRIYA support')} icon="lifebuoy">
		<Row big icon="web" title={_('Help and support')} sub={_('On a phone or computer.')}>
			{#snippet ends()}<span class="row-value">{SUPPORT_URL}</span>{/snippet}
		</Row>
		{#each [_("Say what's wrong, and since when."), _('Save a diagnostics bundle to a USB stick.'), _('Upload it, with the serial number below.')] as step, n (n)}
			<Row title={step}>{#snippet lead()}<span class="step-number">{n + 1}</span>{/snippet}</Row>
		{/each}
		<Wide><div class="row-end"><button onclick={() => desk.openTool('support', 'diagnostics')}>{_('Make a bundle')}</button></div></Wide>
	</Group>
	<Group title={_('About this box')} icon="box" note={_('Support asks for these.')}>
		{#each facts as [name, value, icon] (name)}
			<Row {icon} title={name}>{#snippet ends()}<span class="row-value selectable">{value}</span>{/snippet}</Row>
		{/each}
		{#if status && updatesUntil(status)}
			<Row icon="updates" title={_('Security updates')} sub={_('The box moves to the next version before then.')}>
				{#snippet ends()}<span class="row-value">{updatesUntil(status!)}</span>{/snippet}
			</Row>
		{/if}
	</Group>
	{#if problem}<p class="error">{problem}</p>{/if}
</div>

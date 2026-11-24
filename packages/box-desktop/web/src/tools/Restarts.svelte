<script lang="ts">
	// Control Panel › Restarts (tools.py's restarts): a banner while a restart
	// waits, the maintenance hour, and shutting down.
	import { ask, type Answer } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { _, fill } from '../lib/i18n.svelte';
	import Dropdown from '../components/Dropdown.svelte';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';
	import Switch from '../components/Switch.svelte';
	import Wide from '../components/Wide.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	const hours = Array.from({ length: 24 }, (_x, h) => `${String(h).padStart(2, '0')}:30`);
	let status = $state<Answer | null>(null);
	let picked = $state(-1);
	let problem = $state('');
	const hour = $derived(status?.maintenance_hour ?? '03:30');

	const banner = $derived.by(() => {
		const reasons: string[] = status?.restart_reasons ?? [];
		if (reasons.some((r) => r.startsWith('update: linux-image'))) return [_('Awaiting Kernel Update Completion'), _('A new kernel is ready.')];
		if (reasons.some((r) => r.startsWith('update:'))) return [_('Awaiting Update Completion'), _('Security fixes are installed.')];
		if (reasons.some((r) => r.startsWith('restore point'))) return [_('Awaiting Restore Completion'), _('The OS is restored.')];
		if (reasons.some((r) => r === 'reset the system' || r === 'erase everything')) return [_('Awaiting Reset'), _('The reset is ready.')];
		return [_('Restart Needed'), _('Something waits for a restart.')];
	});

	async function load(): Promise<void> {
		try {
			status = await ask('status');
			picked = hours.indexOf(status.maintenance_hour ?? '03:30');
		} catch (e) {
			problem = (e as Error).message;
		}
	}

	async function adjust(): Promise<void> {
		try {
			await ask('maintenance-hour', { hour: hours[picked] });
		} finally {
			await load();
		}
	}

	void load();
</script>

<div class="settings">
	{#if status}
		{#if status.restart_needed}
			<Group attention>
				<Row icon="alert-gear" role="warn" iconPx={30} title={banner[0]} />
				<Row big icon="info" title={banner[1] + ' ' + fill(_('The system will restart at {hour}, or you can restart manually now.'), { hour })}>
					{#snippet ends()}<button class="primary" onclick={() => desk.restart()}>{_('Finish & Restart')}</button>{/snippet}
				</Row>
			</Group>
		{/if}
		<Group title={_('Maintenance')} icon="maintenance">
			<Row big icon="maintenance" title={_('Maintenance Hour')} sub={_('If an update needs a restart, the box restarts this hour.')}>
				{#snippet ends()}
					<Dropdown names={hours} bind:selected={picked} label={_('Maintenance Hour')} />
					<button onclick={adjust}>{_('Adjust Time')}</button>
				{/snippet}
			</Row>
		</Group>
		<Group title={_('Power')} icon="power">
			<!-- Always on (design §8): shown, not changed here. -->
			<Row big icon="bolt" title={_('Always On')} sub={_('The box runs 24/7 with power-saving mode disabled.')}>
				{#snippet ends()}<Switch fixed on={status?.always_on !== false} />{/snippet}
			</Row>
			<Row big icon="power" title={_('Shutdown')} sub={_('If powered off, it must be turned back on in person.')} />
			<Wide><div class="row-end"><button onclick={() => desk.shutDown()}>{_('Shut down system')}</button></div></Wide>
		</Group>
	{/if}
	{#if problem}<p class="error">{problem}</p>{/if}
</div>

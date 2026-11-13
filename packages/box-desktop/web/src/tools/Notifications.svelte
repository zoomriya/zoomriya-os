<script lang="ts">
	// Control Panel › Notifications (control.py's cp_notifications): which
	// kinds this admin sees on the bell; the notifications are in Log Center.
	import { ask } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { _ } from '../lib/i18n.svelte';
	import { KINDS } from '../lib/words';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';
	import Switch from '../components/Switch.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	let kinds = $state<string[]>([]);
	let problem = $state('');
	const off = $derived(new Set(desk.prefs.notifications?.off ?? []));
	ask('notifications').then((a) => (kinds = a.kinds ?? [])).catch((e) => (problem = e.message));
	const cap = (k: string) => k.charAt(0).toUpperCase() + k.slice(1);
</script>

<div class="settings">
	<Group title={_('Show Notifications For')} icon="bell">
		{#snippet aside()}<button class="flat-link" onclick={() => desk.openTool('logs', 'notifications')}>{_('See past notifications')}</button>{/snippet}
		{#each kinds as kind (kind)}
			{@const [name, about, icon] = KINDS[kind] ?? [cap(kind), '', 'bell']}
			<Row big {icon} title={_(name)} sub={about ? _(about) : ''}>
				{#snippet ends()}<Switch on={!off.has(kind)} changed={(on) => desk.turnNotifications(kind, on)} label={_(name)} />{/snippet}
			</Row>
		{/each}
	</Group>
	{#if problem}<p class="error">{problem}</p>{/if}
</div>

<script lang="ts">
	// Control Panel › Restore & Reset (tools.py's restore_points): from the
	// mildest fix to the most drastic: the OS as it was before an update,
	// updates a restore holds back, the system as it left the factory, then
	// erasing everything.
	import { ask, type Answer } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { when } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import Group from '../components/Group.svelte';
	import Icon from '../components/Icon.svelte';
	import Row from '../components/Row.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	let points = $state<Answer[]>([]);
	let held = $state<Answer[]>([]);
	let problem = $state('');
	// How each kind of restore point is worded: its tag and what it's from.
	const kinds = $derived<Record<string, [string, string]>>({
		'before-update': [_('before update'), _('Update')], 'after-update': [_('after update'), _('Update')],
		'before-restore': [_('before restore'), _('Restore')], factory: [_('factory'), _('As it left the factory')]
	});

	ask('restore-point-list').then((a) => (points = [...(a.restore_points ?? [])].reverse())).catch((e) => (problem = e.message));
	ask('held').then((a) => (held = a.held ?? [])).catch(() => {});

	function restore(point: Answer): void {
		desk.confirm({
			title: _('Restore the OS?'),
			text: fill(_('The OS returns to {time} ({what}) on the next restart. Family files and app data stay.'),
				{ time: when(point.date ?? ''), what: point.description ?? '' }),
			button: _('Restore'),
			action: (password) => ask('restore-point-restore', { number: point.number }, password),
			done: () => desk.restartNow()
		});
	}

	function letUpdate(item: Answer): void {
		desk.confirm({
			title: _('Update now?'),
			text: fill(_("{package} installs its latest fix now, even if that's {taken}, the version the restore removed."),
				{ package: item.package ?? '', taken: item.taken_back ?? '' }),
			button: _('Update now'),
			action: async (password) => {
				// Lifts the hold, then installs the security fixes at once.
				const answer = await ask('held-release', { package: item.package }, password);
				await ask('security-check-now').catch(() => {});
				return answer;
			},
			done: () => (held = held.filter((h) => h !== item))
		});
	}

	function resetSystem(): void {
		desk.confirm({
			title: _('Reset the system?'), text: _('The system returns to its factory state on restart. Family apps and data stay.'),
			button: _('Reset'), action: (password) => ask('factory-reset', { level: 'system' }, password), done: () => desk.restartNow()
		});
	}

	function eraseEverything(): void {
		desk.confirm({
			title: _('Erase everything?'),
			text: _("Deletes every app, account, photo and message on restart. This can't be undone. Enter the setup code from the sticker and your password."),
			button: _('Erase everything'), extra: _('Setup code, like ABCD-EFGH'),
			action: (password, code) => ask('factory-reset', { level: 'everything', setup_code: code }, password),
			done: () => desk.restartNow()
		});
	}
</script>

<div class="settings">
	<Group title={_('Restore points')} lead={_('Reverts the OS to an earlier state. Family files and app data stay. Try this first.')} icon="calendar-clock">
		{#each points as point (point.number)}
			{@const [tag, made] = kinds[point.kind ?? ''] ?? ['', point.description ?? '']}
			<Row big icon={point.kind === 'factory' ? 'factory' : 'restore-point'} title={when(point.date ?? '')} sub={made} {tag}>
				{#snippet ends()}{#if point.kind !== 'current'}<button onclick={() => restore(point)}>{_('Restore')}</button>{/if}{/snippet}
			</Row>
		{/each}
	</Group>
	{#if problem}<p class="error">{problem}</p>{/if}
	{#if held.length}
		<Group title={_('Updates on Hold')} icon="held" lead={_('A restore rolled back these updates. They will update automatically when a newer fix is released.')}>
			{#each held as item (item.package)}
				<Row big icon="package" title={item.package ?? ''}
					sub={fill(_('On {version}. Skipping {taken}.'), { version: item.version ?? '', taken: item.taken_back ?? '' })}>
					{#snippet ends()}<button onclick={() => letUpdate(item)}>{_('Update now')}</button>{/snippet}
				</Row>
			{/each}
		</Group>
	{/if}
	<Group title={_('Reset')} icon="reset">
		<Row big icon="reset" title={_('System Reset')}
			sub={_('Returns the OS to factory state, then updates it. Your data stays. Use if restoring fails.')}>
			{#snippet ends()}<button onclick={resetSystem}>{_('Reset the system')}</button>{/snippet}
		</Row>
	</Group>
	<Group title={_('Erase everything')} icon="erase">
		<Row big icon="erase" title={_('Factory Reset')} sub={_('Erases everything, leaving the box as new. Needs the setup code from the sticker.')}>
			{#snippet ends()}<button class="danger with-icon" onclick={eraseEverything}><Icon name="trash" px={18} role="white" />{_('Erase everything')}</button>{/snippet}
		</Row>
	</Group>
</div>

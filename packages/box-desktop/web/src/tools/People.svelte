<script lang="ts">
	// Control Panel › People (people.py): the family's accounts in the box's
	// ZOOMRIYA ID, so a box with no Family Hub can still add someone.
	import { get } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { _, fill } from '../lib/i18n.svelte';
	import { inOrder, shownName, type Person } from '../lib/people';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';
	import AddPerson from './AddPerson.svelte';
	import EditPerson from './EditPerson.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	let people = $state<Person[] | null>(null);
	let problem = $state('');

	async function load(): Promise<void> {
		try {
			people = await get<Person[]>('/api/people');
			problem = '';
		} catch (e) {
			problem = (e as Error).message;
		}
	}

	function tags(person: Person): string {
		const listed = [];
		if (person.role === 'admin') listed.push(_('Admin'));
		if (person.level === 'child') listed.push(_('Child'));
		else if (person.level === 'teen') listed.push(_('Teen'));
		if (person.is_active === false) listed.push(_('Off'));
		return listed.join(' · ');
	}

	function lines(person: Person): string {
		const listed = ['@' + (person.username ?? '')];
		if (person.id === desk.member?.id) listed[0] += ' · ' + _('You');
		if (person.kind === 'child') {
			const names = (person.guardians ?? []).filter((g: Person) => g.is_active).map((g: Person) => g.display_name ?? '');
			if (names.length) listed.push(fill(_('Parents or guardians: {names}'), { names: names.join(', ') }));
		}
		if (person.needs_guardian) listed.push(_('Needs a parent or guardian.'));
		return listed.join('\n');
	}

	void load();
</script>

<div class="settings">
	{#if problem}
		<p class="error">{problem}</p>
	{:else if people}
		<Group title={_('People')} icon="user" lead={_('Everyone who signs in to this box.')}>
			{#each inOrder(people) as person (person.id)}
				<Row icon="user" role={person.is_active === false ? 'muted' : 'accent'} title={shownName(person)} sub={lines(person)} tag={tags(person)}>
					{#snippet ends()}<button onclick={() => desk.sheet(EditPerson, { person, everyone: people, changed: load })}>{_('Edit')}</button>{/snippet}
				</Row>
			{/each}
			<Row big icon="user" title={_('Add a person')} sub={_('They choose their own password, here at the box.')}>
				{#snippet ends()}<button class="primary" onclick={() => desk.sheet(AddPerson, { everyone: people, changed: load })}>{_('Add')}</button>{/snippet}
			</Row>
		</Group>
		<Group title={_('Good to know')} icon="info">
			<Row icon="globe" title={_('Same in Family Hub')} sub={_("Family Hub's Family page shows these people too.")} />
			<Row icon="shield" title={_("Children's settings")} sub={_('Their parents set apps and quiet hours in Family Hub.')} />
			<Row icon="lock" title={_('Always an admin')} sub={_("You can't turn off your own account or remove your own admin role.")} />
		</Group>
	{/if}
</div>

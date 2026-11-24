<script lang="ts">
	// A child's birth year and their one or two parents or guardians (people.py's child_fields).
	import { _ } from '../lib/i18n.svelte';
	import { guardianChoices, MAX_GUARDIANS, shownName, type Person } from '../lib/people';

	let {
		everyone, person = null, year = $bindable(), guardians = $bindable()
	}: { everyone: Person[]; person?: Person | null; year: number; guardians: number[] } = $props();
	const thisYear = new Date().getFullYear();
	const choices = $derived(guardianChoices(everyone, person?.id));
	const full = $derived(guardians.length >= MAX_GUARDIANS);

	function toggle(id: number, on: boolean): void {
		guardians = on ? [...guardians, id] : guardians.filter((g) => g !== id);
	}
</script>

<div style:display="flex" style:flex-direction="column" style:gap="10px">
	<div class="row-between">
		<span class="row-title grow">{_('Birth year')}</span>
		<input type="number" style:width="110px" min={thisYear - 17} max={thisYear} bind:value={year} aria-label={_('Birth year')} />
	</div>
	<span class="row-title">{_('Parents or guardians (one or two)')}</span>
	{#each choices as adult (adult.id)}
		<label class="check">
			<input type="checkbox" checked={guardians.includes(adult.id)} disabled={full && !guardians.includes(adult.id)}
				onchange={(e) => toggle(adult.id, (e.currentTarget as HTMLInputElement).checked)} />
			{shownName(adult)}
		</label>
	{/each}
	{#if !choices.length}<span class="row-sub">{_('Add an adult first.')}</span>{/if}
</div>

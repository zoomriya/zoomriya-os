<script lang="ts">
	// Two or three buttons joined into one, one of them pressed (Left | Right).
	import Icon from './Icon.svelte';

	let {
		choices, chosen = $bindable(''), changed
	}: { choices: [string, string, string?][]; chosen?: string; changed?: (key: string) => void } = $props();
</script>

<div class="segmented" role="radiogroup">
	{#each choices as [key, name, icon] (key)}
		<button class:on={key === chosen} role="radio" aria-checked={key === chosen}
			onclick={() => { if (chosen !== key) { chosen = key; changed?.(key); } }}>
			{#if icon}<span class="with-icon"><Icon name={icon} px={16} />{name}</span>{:else}{name}{/if}
		</button>
	{/each}
</div>

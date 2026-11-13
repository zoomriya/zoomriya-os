<script lang="ts">
	// Control Panel › Sound (control.py's cp_sound): the box's speakers and
	// microphone, their volume, a test sound, and the microphone's level as it
	// hears, while this page is in front.
	import { onDestroy } from 'svelte';
	import { get, post } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { _ } from '../lib/i18n.svelte';
	import Dropdown from '../components/Dropdown.svelte';
	import Group from '../components/Group.svelte';
	import Icon from '../components/Icon.svelte';
	import Level from '../components/Level.svelte';
	import Meter from '../components/Meter.svelte';
	import Row from '../components/Row.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	interface Device { id: number; name: string; on: boolean }
	interface Side { devices: Device[]; volume: number; muted: boolean }
	let output = $state<Side>({ devices: [], volume: 0, muted: false });
	let input = $state<Side>({ devices: [], volume: 0, muted: false });
	let level = $state(0);
	let listening: EventSource | null = null;

	async function load(): Promise<void> {
		try {
			const sound = await get<{ output: Side; input: Side }>('/api/sound');
			if (sound.output) output = sound.output;
			if (sound.input) input = sound.input;
		} catch {
			// Shown as it was.
		}
	}

	const send = (body: Record<string, unknown>) => post('/api/sound', body).catch(() => {});

	// The microphone's level only while Sound is in front.
	$effect(() => {
		const front = shown && desk.active === 'control';
		if (front && !listening) {
			listening = new EventSource('/api/sound/level');
			listening.onmessage = (e) => (level = Number(e.data));
		} else if (!front && listening) {
			listening.close();
			listening = null;
			level = 0;
		}
	});
	onDestroy(() => listening?.close());
	void load();
</script>

{#snippet devicePick(side: Side, which: string, label: string)}
	{#if side.devices.length}
		<Dropdown names={side.devices.map((d) => d.name)} {label} selected={Math.max(0, side.devices.findIndex((d) => d.on))}
			changed={(i) => send({ action: 'use', device: which, id: side.devices[i].id })} />
	{:else}
		<span class="row-value">{_('None found')}</span>
	{/if}
{/snippet}

{#snippet volume(side: Side, which: string, on: string, off: string)}
	<button class="bar-button" class:on={side.muted} title={_('Mute')} aria-label={_('Mute')} aria-pressed={side.muted}
		onclick={() => { side.muted = !side.muted; send({ action: 'mute', device: which, muted: side.muted }); }}>
		<Icon name={side.muted ? off : on} px={20} />
	</button>
	<Level bind:value={side.volume} changed={(v) => send({ action: 'volume', device: which, level: v })} />
{/snippet}

<div class="settings">
	<Group title={_('Output')} icon="volume">
		<Row big icon="speaker" title={_('Output Device')}>
			{#snippet ends()}
				{@render devicePick(output, 'output', _('Output Device'))}
				<button class="with-icon" title={_('Play a test sound')} onclick={() => send({ action: 'test' })}><Icon name="play" px={14} />{_('Test')}</button>
			{/snippet}
		</Row>
		<Row big icon="sliders" title={_('Output Volume')}>
			{#snippet ends()}{@render volume(output, 'output', 'volume', 'muted')}{/snippet}
		</Row>
	</Group>
	<Group title={_('Input')} icon="mic">
		<Row big icon="mic" title={_('Input Device')}>
			{#snippet ends()}{@render devicePick(input, 'input', _('Input Device'))}{/snippet}
		</Row>
		<Row big icon="sliders" title={_('Input Volume')}>
			{#snippet ends()}{@render volume(input, 'input', 'mic', 'mic-off')}{/snippet}
		</Row>
		<Row big icon="wave" title={_('Input Level')} sub={_('Speak to see it move.')}>
			{#snippet ends()}<Meter fraction={level} kind="mic" width="260px" />{/snippet}
		</Row>
	</Group>
</div>

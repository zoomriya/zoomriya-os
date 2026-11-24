<script lang="ts">
	// Resource Monitor › Overview (tools.py's resources): the processor, the
	// memory, the network's last three minutes and each port's, the disks
	// and the hottest sensor, every three seconds.
	import { onDestroy } from 'svelte';
	import { ask, type Answer } from '../lib/api';
	import { rate, size } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import Meter from '../components/Meter.svelte';
	import Rates from '../components/Rates.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	let answer = $state<Answer>({});
	let history = $state<[number, number][]>([]);

	function got(a: Answer): void {
		answer = a;
		const net = a.network ?? {};
		history = [...history, [Number(net.in_bps ?? 0), Number(net.out_bps ?? 0)] as [number, number]].slice(-60);
	}

	const tick = () => ask('resources').then(got).catch(() => {});
	void tick();
	const timer = setInterval(tick, 3000);
	onDestroy(() => clearInterval(timer));

	const percent = $derived(Number(answer.cpu_percent ?? 0));
	const total = $derived(answer.memory_total_bytes ?? 0);
	const used = $derived(answer.memory_used_bytes ?? 0);
	const links = $derived((answer.network?.links ?? []) as Answer[]);
	const disks = $derived(answer.disks ?? {});
	const temps = $derived((answer.temperatures ?? []).map((t: Answer) => t.celsius).filter(Boolean) as number[]);
	const last = $derived(history[history.length - 1]);
</script>

<div style:display="flex" style:flex-direction="column" style:gap="12px">
	<span class="field-label">{answer.cpu_percent === undefined ? _('Processor') : fill(_('Processor: {percent}%'), { percent: percent.toFixed(0) })}</span>
	<Meter fraction={percent / 100} />
	<span class="field-label">{total ? fill(_('Memory: {used} of {total}'), { used: size(used), total: size(total) }) : _('Memory')}</span>
	<Meter fraction={total ? used / total : 0} kind="memory" />
	<div class="graph-card">
		<div class="row-between" style:gap="18px">
			<span class="field-label grow">{_('Network')}</span>
			{#each [['↓', last?.[0], _('Download'), 'down'], ['↑', last?.[1], _('Upload'), 'up']] as [arrow, value, words, which] (which)}
				<span style:display="flex" style:gap="4px" style:align-items="baseline">
					<span class="rate-arrow {which}">{arrow}</span>
					<span class="row-title">{value === undefined ? '–' : rate(Number(value))}</span>
					<span class="row-sub">{words}</span>
				</span>
			{/each}
		</div>
		<Rates {history} height={120} />
		{#if links.length > 1}
			<div class="facts">
				{#each links as link (link.name)}
					<span class="fact-name">{link.name}</span>
					<span class="fact">{fill(_('in {received}, out {sent}'), { received: rate(link.in_bps ?? 0), sent: rate(link.out_bps ?? 0) })}</span>
				{/each}
			</div>
		{/if}
	</div>
	{#if answer.cpu_percent !== undefined}
		<div class="facts">
			<span class="fact-name">{_('Disks')}</span>
			<span class="fact">{fill(_('reading {read}, writing {written}'), { read: rate(disks.read_bps ?? 0), written: rate(disks.write_bps ?? 0) })}</span>
			<span class="fact-name">{_('Hottest sensor')}</span>
			<span class="fact">{temps.length ? `${Math.max(...temps).toFixed(0)} °C` : _('No sensor')}</span>
		</div>
	{/if}
</div>

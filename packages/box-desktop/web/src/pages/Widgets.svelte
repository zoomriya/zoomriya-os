<script lang="ts">
	// The widgets down the right (app.py's widgets): Box health, CPU and
	// memory, Network and Space, each card tinted in its own colour.
	import { desk } from '../lib/desk.svelte';
	import { duration, rate, size, when } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import Badge from '../components/Badge.svelte';
	import Meter from '../components/Meter.svelte';
	import Rates from '../components/Rates.svelte';
	import Sparkline from '../components/Sparkline.svelte';

	const drives = $derived((desk.health.drives ?? []) as Record<string, unknown>[]);
	const sick = $derived(drives.filter((d) => d.health !== 'healthy' && d.health != null));
	const needs = $derived(!!sick.length || !!desk.status.restart_needed);
	const rows = $derived.by(() => {
		const listed: [string, string, boolean][] = [
			[_('Drives'), sick.length ? fill(_('{n} need checking'), { n: sick.length }) : _('All healthy'), !!sick.length],
			[_('Security fixes'), when(desk.status.last_security_update ?? ''), false]
		];
		if (desk.status.restart_needed) listed.push([_('Restart'), _('Needed'), true]);
		if (desk.status.advanced_access) listed.push([_('Advanced access'), _('On'), true]);
		return listed;
	});
	const library = $derived(desk.storage.library ?? {});
	const disk = $derived(library.path ? library : (desk.storage.system ?? {}));
	const total = $derived(Number(disk.size_bytes ?? 0));
	const used = $derived(Number(disk.used_bytes ?? 0));
	const res = $derived(desk.resources);
	const memTotal = $derived(Number(res.memory_total_bytes ?? 0));
	const memUsed = $derived(Number(res.memory_used_bytes ?? 0));
	const links = $derived(((desk.network.links ?? []) as Record<string, unknown>[]).filter((x) => x.kind === 'ethernet'));
	const link = $derived(links.find((x) => x.connected));
	const speed = $derived(Number(link?.speed_mbps ?? 0));
	const last = $derived(desk.netHistory[desk.netHistory.length - 1]);
</script>

<aside class="widget-column" aria-label={_('Widgets')}>
	<section class="widget widget-blue">
		<div class="widget-head">
			<Badge name="box" px={20} shade="blue" />
			<span class="widget-title">{_('Box health')}</span>
			<span class="widget-aside">{desk.boxName.replace(/\.local$/, '')}</span>
		</div>
		<div style:display="flex" style:gap="12px" style:align-items="center">
			<Badge name={needs ? 'alert' : 'check'} px={40} role={needs ? 'warn' : 'ok'} />
			<div style:display="flex" style:flex-direction="column">
				<span class="widget-big">{desk.status.os ? (needs ? _('Needs a look') : _("Everything's fine")) : _('Checking the box…')}</span>
				{#if desk.status.os}
					<span class="widget-small">{fill(_('{os} · up {time}'), { os: desk.osName, time: duration(Number(desk.status.uptime_seconds ?? 0)) })}</span>
				{/if}
			</div>
		</div>
		{#if desk.status.os}
			<div class="widget-rows">
				{#each rows as [name, value, warn] (name)}
					<span class="widget-small">{name}</span>
					<span class="widget-value" class:warn class:ok={!warn}>{value}</span>
				{/each}
			</div>
		{/if}
	</section>

	<section class="widget widget-indigo">
		<div class="widget-head">
			<Badge name="cpu" px={20} shade="indigo" />
			<span class="widget-title">{_('CPU and memory')}</span>
		</div>
		<div style:display="flex" style:gap="10px" style:align-items="center">
			<div style:display="flex" style:flex-direction="column">
				<span class="widget-figure">{res.cpu_percent !== undefined ? `${Number(res.cpu_percent).toFixed(0)}%` : '–'}</span>
				<span class="widget-small">{_('CPU')}</span>
			</div>
			<div style:flex="1"><Sparkline points={desk.cpuHistory} /></div>
		</div>
		<div class="row-between">
			<span class="widget-small grow">{_('Memory')}</span>
			<span class="widget-small">{memTotal ? fill(_('{used} of {total}'), { used: size(memUsed), total: size(memTotal) }) : ''}</span>
		</div>
		<Meter fraction={memTotal ? memUsed / memTotal : 0} kind="bar-memory" />
	</section>

	<!-- Network: download and upload now, their last minutes, and the cable. -->
	<section class="widget widget-green widget-link" title={_('Open Resource Monitor')} role="button" tabindex="0"
		onclick={() => desk.openTool('resources')} onkeydown={(e) => e.key === 'Enter' && desk.openTool('resources')}>
		<div class="widget-head">
			<Badge name="ethernet" px={20} shade="green" />
			<span class="widget-title">{_('Network')}</span>
			<span class="widget-aside">{link ? String((link.ipv4 as string[] | undefined)?.[0] ?? '').split('/')[0] : ''}</span>
		</div>
		<div style:display="grid" style:grid-template-columns="1fr 1fr">
			{#each [['↓', last?.[0], _('Download'), 'down'], ['↑', last?.[1], _('Upload'), 'up']] as [arrow, value, words, which] (which)}
				<div style:display="flex" style:flex-direction="column">
					<span style:display="flex" style:gap="4px" style:align-items="baseline">
						<span class="rate-arrow {which}">{arrow}</span>
						<span class="widget-figure-line">{value === undefined ? '–' : rate(Number(value))}</span>
					</span>
					<span class="widget-small">{words}</span>
				</div>
			{/each}
		</div>
		<Rates history={desk.netHistory.slice(-40)} height={40} />
		<span class="widget-value" style:text-align="left" class:ok={!!link} class:warn={!link}>
			{link ? (speed ? fill(_('Connected · {speed}'), { speed: speed >= 1000 ? `${speed / 1000} Gb/s` : `${speed} Mb/s` }) : _('Connected')) : _('Cable unplugged')}
		</span>
	</section>

	<section class="widget widget-cyan">
		<div class="widget-head">
			<Badge name="disk" px={20} shade="cyan" />
			<span class="widget-title">{_('Space')}</span>
			<span class="widget-aside">{total ? `${((used / total) * 100).toFixed(0)}%` : ''}</span>
		</div>
		<span class="widget-figure-line">
			{#if total}{@html fill(_('<b>{used}</b> <small>of {total} used</small>'), { used: size(used), total: size(total) })}{:else}–{/if}
		</span>
		<Meter fraction={total ? used / total : 0} kind="bar-space" />
		<span class="widget-small">
			{!library.path ? _("The box's own disk") : library.mirror ? _('Library drives mirrored') : _('Library')}
		</span>
	</section>
</aside>

<script lang="ts">
	// The USB installer (design §10; the native installer's app.py), full
	// screen on the computer it's installing: Language picks the installer's
	// language, Welcome checks the computer can take ZOOMRIYA OS, Drive picks
	// where it goes and shows it before and after, Summary asks once more
	// before anything is erased, Install copies it while a slideshow tells what
	// the box does, and Done restarts into the box's first setup.
	import { onDestroy } from 'svelte';
	import { _, fill, N_, use } from '../lib/i18n.svelte';
	import Logo from '../components/Logo.svelte';
	import Spinner from '../components/Spinner.svelte';

	const STEPS = [N_('Language'), N_('Welcome'), N_('Drive'), N_('Summary'), N_('Install'), N_('Done')];
	const [LANGUAGE, WELCOME, DRIVE, SUMMARY, INSTALL, DONE] = [0, 1, 2, 3, 4, 5];
	/** The installer's languages, each named in itself. */
	const LANGUAGES: [string, string][] = [['en', 'English'], ['fr', 'Français'], ['de', 'Deutsch'], ['es', 'Español'],
		['ta', 'தமிழ்'], ['ja', '日本語'], ['zh_CN', '简体中文']];
	const SLIDE_SECONDS = 7;
	const SLIDES: [string, string, string, [string, string]][] = [
		['home', N_('Your files stay at home'), N_('Everything stays on the box, at home, not in someone else\'s cloud.'), ['#1765c1', '#3b8ce0']],
		['shield', N_('Security fixes install by themselves'), N_('OS security updates install automatically.'), ['#0f766e', '#14a597']],
		['restore', N_('Restore points'), N_('A restore point before every update. One click in Box Desktop goes back; your files stay.'), ['#5b3fb8', '#8462e0']],
		['fallback', N_("A kernel that won't start falls back"), N_("If a new kernel won't start, the box falls back to the previous one."), ['#b4531c', '#e07a3b']],
		['account', N_('One account: ZOOMRIYA ID'), N_('One sign-in for the box and every ZOOMRIYA app.'), ['#1d6f42', '#2f9a5e']],
		['lock', N_('Closed unless you open it'), N_('A firewall blocks anything uninvited; remote access stays off until an admin enables it.'), ['#24364b', '#3d5874']]
	];
	/** The installer's line icons, drawn for ZOOMRIYA OS (zoomriya_installer/icons.py). */
	const GLYPHS: Record<string, string> = {
		home: '<path d="M4 11l8-6.5 8 6.5"/><path d="M6 9.5V19.5h12V9.5"/><path d="M10 19.5v-5h4v5"/>',
		shield: '<path d="M12 3l7.5 3v5.5c0 4.5-3.2 8-7.5 9.5-4.3-1.5-7.5-5-7.5-9.5V6z"/><path d="M8.5 12l2.5 2.5 4.5-5"/>',
		restore: '<path d="M4 12a8 8 0 1 0 2.3-5.7L4 8.5M4 4.5v4h4"/><path d="M12 8v4.5l3 2"/>',
		fallback: '<path d="M20 11a8 8 0 0 0-14.3-4.9L4 8M4 4v4h4M4 13a8 8 0 0 0 14.3 4.9L20 16M20 20v-4h-4"/>',
		account: '<circle cx="12" cy="8.5" r="4"/><path d="M4.5 20.5a7.5 7.5 0 0 1 15 0"/>',
		lock: '<rect x="5" y="10.5" width="14" height="10" rx="2"/><path d="M8 10.5V7.5a4 4 0 0 1 8 0v3"/>',
		disk: '<rect x="3" y="6" width="18" height="12" rx="2"/><path d="M3 13.5h18M7 16h.01M15 16h2"/>',
		usb: '<rect x="7" y="9" width="10" height="12" rx="2"/><path d="M9 9V3h6v6M11 5.5h.01M13 5.5h.01"/>',
		'sd-card': '<path d="M7 3h7.5L18 6.5V21H7z"/><path d="M10 3v3.5M12.5 3v3.5"/>',
		alert: '<path d="M12 3.5l9.5 16.5h-19z"/><path d="M12 10v4.5M12 17.5v.01"/>'
	};
	const glyph = (name: string, px: number, colour: string) =>
		`<svg xmlns="http://www.w3.org/2000/svg" width="${px}" height="${px}" viewBox="0 0 24 24" fill="none" stroke="${colour}" ` +
		`stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${GLYPHS[name]}</svg>`;

	interface Segment { name: string; size: number; size_text: string; colour: string }
	interface Disk { path: string; title: string; kind: string; bus: string; size_text: string; usable: boolean; reason: string;
		parts: string[]; now: Segment[]; after: Segment[] }
	interface State {
		language: string; os: string; looking: boolean; online: boolean | null; log: string;
		requirements: { words: string; ok: boolean }[]; disks: Disk[];
		install: { running: boolean; done: boolean; problem: string; overall: number; stage_name: string };
	}

	let step = $state(LANGUAGE);
	let box = $state<State | null>(null);
	let chosen = $state('');
	let eraseOk = $state(false);
	let slide = $state(0);
	let busy = $state(false);
	let problem = $state('');
	let language = $state('en');

	async function call(path: string, body: unknown = {}): Promise<Record<string, unknown>> {
		const answer = await fetch(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
		const data = await answer.json().catch(() => ({}));
		if (!answer.ok) throw new Error((data as { error?: string }).error || _('Something went wrong.'));
		return data;
	}

	async function refresh(): Promise<void> {
		try {
			box = await (await fetch('/api/state', { cache: 'no-store' })).json();
		} catch {
			return;
		}
		const disks = box!.disks;
		// The chosen drive, or else the first that can be chosen.
		if (!disks.some((d) => d.path === chosen && d.usable)) chosen = disks.find((d) => d.usable)?.path ?? '';
		if (step === INSTALL && box!.install.done) step = DONE;
	}

	const timer = setInterval(refresh, 1000);
	const slides = setInterval(() => { if (step === INSTALL) slide = (slide + 1) % SLIDES.length; }, SLIDE_SECONDS * 1000);
	onDestroy(() => { clearInterval(timer); clearInterval(slides); });
	void refresh();

	async function speak(code: string): Promise<void> {
		language = code;
		await use(code);
		// The drives' and checks' words in it too: the server says them.
		await call('/api/language', { code }).catch(() => {});
		await refresh();
	}

	const disk = $derived(box?.disks.find((d) => d.path === chosen) ?? null);
	const failed = $derived(box?.install.problem ?? '');
	const ready = $derived(
		step === WELCOME ? !!box && box.requirements.every((r) => r.ok)
			: step === DRIVE ? !!disk
			: step === SUMMARY ? !!disk && eraseOk
			: true
	);

	async function go(next: number): Promise<void> {
		problem = '';
		if (next === SUMMARY) eraseOk = false;
		if (next === INSTALL) {
			busy = true;
			try {
				await call('/api/install', { disk: chosen });
			} catch (e) {
				problem = (e as Error).message;
				busy = false;
				return;
			}
			busy = false;
			slide = 0;
		}
		step = next;
	}

	async function confirm(): Promise<void> {
		if (step === DONE) {
			if (failed) step = DRIVE;
			else await call('/api/restart').catch(() => {});
		} else {
			await go(step + 1);
		}
	}

	const nextWords = $derived(step === SUMMARY ? _('Erase and install') : step === DONE ? (failed ? _('Try again') : _('Restart now')) : _('Next'));

	/** Small partitions still show: a few pixels' worth each. */
	function widths(segments: Segment[]): number[] {
		const total = segments.reduce((sum, s) => sum + s.size, 0) || 1;
		return segments.map((s) => Math.max((s.size / total) * 100, 1.2));
	}
</script>

{#snippet bar(caption: string, segments: Segment[])}
	<div class="bar-block">
		<span class="field-label">{caption}</span>
		<div class="bar">{#each segments as s, i (i)}<span style:flex-grow={widths(segments)[i]} style:background={s.colour}></span>{/each}</div>
		<div class="legend">
			{#each segments as s, i (i)}<span><i style:background={s.colour}></i>{s.name} <span class="dim">{s.size_text}</span></span>{/each}
		</div>
	</div>
{/snippet}

<div class="backdrop" style="background-image: url(/backgrounds/wallpaper.jpg)"></div>
<div class="place">
	<div class="installer">
		<aside class="side">
			<div class="brand"><Logo height={22} /><span>{_('OS Installer')}</span></div>
			<ol>
				{#each STEPS as name, i (name)}
					<li class:current={i === step} class:done={i < step}>
						<span class="marker">{i < step ? '✓' : i + 1}</span><span>{_(name)}</span>
					</li>
				{/each}
			</ol>
		</aside>
		<section class="main">
			<div class="page">
				{#if step === LANGUAGE}
					<h1 class="title">{_('Choose your language')}</h1>
					<p class="lead">{_("For the installer. The box's language is chosen at first setup.")}</p>
					<div class="choices" role="listbox" aria-label={_('Language')}>
						{#each LANGUAGES as [code, name] (code)}
							<button class:on={language === code} role="option" aria-selected={language === code} onclick={() => speak(code)}>{name}</button>
						{/each}
					</div>
				{:else if step === WELCOME}
					<div class="hero" aria-hidden="true">
						<svg viewBox="0 0 600 220" preserveAspectRatio="xMidYMid slice">
							<rect width="600" height="220" fill="#e3eefb" />
							<circle cx="72" cy="44" r="90" fill="#b9d4f3" fill-opacity=".35" /><circle cx="516" cy="187" r="120" fill="#b9d4f3" fill-opacity=".3" />
							<circle cx="420" cy="11" r="60" fill="#b9d4f3" fill-opacity=".25" />
							<rect x="190" y="64" width="220" height="120" rx="20" fill="#fff" style="filter: drop-shadow(0 10px 14px rgba(15, 51, 102, .18))" />
							<rect x="208" y="82" width="184" height="10" rx="5" fill="#eef2f6" />
							{#each [0, 1, 2, 3, 4] as i (i)}<rect x={214 + i * 16} y="150" width="8" height="14" rx="3" fill="#dfe5ec" />{/each}
							<circle cx="376" cy="157" r="12" fill="#1765c1" fill-opacity=".18" /><circle cx="376" cy="157" r="6" fill="#1765c1" />
						</svg>
					</div>
					<h1 class="title">{_('Install ZOOMRIYA OS')}</h1>
					<p class="lead">{_('Turns this computer into a ZOOMRIYA Box in about five minutes. The drive you choose next is erased.')}</p>
					<div class="checks">
						{#each box?.requirements ?? [] as r (r.words)}
							<div><span class="mark" class:bad={!r.ok}>{r.ok ? '✓' : '!'}</span>{r.words}</div>
						{/each}
						{#if box?.online === null}<div><span class="mark wait">…</span>{_('Checking the internet…')}</div>
						{:else if box?.online}<div><span class="mark">✓</span>{_('Internet: connected')}</div>
						{:else if box}<div><span class="mark warn">!</span>{_('No internet. Installing anyway; updates arrive once the box is online. Plug in a network cable to connect.')}</div>{/if}
					</div>
				{:else if step === DRIVE}
					<div class="row-between">
						<h1 class="title grow">{_('Where should ZOOMRIYA OS go?')}</h1>
						{#if box?.looking}<Spinner />{/if}
						<button onclick={() => call('/api/look').then(refresh)}>{_('Look again')}</button>
					</div>
					<p class="lead">{_('The chosen drive is erased; others aren\'t touched. Plugged one in? Press Look again.')}</p>
					<div class="disks" role="listbox" aria-label={_('Drive')}>
						{#each box?.disks ?? [] as d (d.path)}
							<button class="disk" class:on={chosen === d.path} disabled={!d.usable} role="option" aria-selected={chosen === d.path}
								onclick={() => (chosen = d.path)}>
								<span class="disk-icon">{@html glyph(d.bus === 'USB' ? 'usb' : d.bus === 'SD card' ? 'sd-card' : 'disk', 32, d.usable ? 'var(--accent)' : 'var(--muted)')}</span>
								<span class="disk-words">
									<span class="disk-name">{d.title}</span>
									<span class="dim">{d.kind} · {d.path}</span>
									{#if d.reason}<span class="disk-reason">{d.reason}</span>{/if}
								</span>
								<span class="disk-size">{d.size_text}</span>
							</button>
						{/each}
					</div>
					{#if disk}
						{@render bar(_('Now'), disk.now)}
						{@render bar(_('After installing'), disk.after)}
					{/if}
				{:else if step === SUMMARY && disk}
					<h1 class="title">{_('Ready to install')}</h1>
					<p class="lead">{_('Check the drive. Nothing has changed yet.')}</p>
					<div class="facts">
						<span class="fact-name">{_('Drive')}</span><span class="fact">{disk.title} ({disk.kind}, {disk.path})</span>
						<span class="fact-name">{_('Size')}</span><span class="fact">{disk.size_text}</span>
						<span class="fact-name">{_('Erased')}</span><span class="fact">{disk.parts.join(', ') || _('Nothing: the drive is empty')}</span>
						<span class="fact-name">{_('Installed')}</span><span class="fact">{box?.os}</span>
					</div>
					{@render bar(_('The drive after installing'), disk.after)}
					<label class="warning">
						<span class="icon">{@html glyph('alert', 24, '#7a4510')}</span>
						<input type="checkbox" bind:checked={eraseOk} />
						{fill(_('Erase everything on {drive} ({size})'), { drive: disk.title, size: disk.size_text })}
					</label>
				{:else if step === INSTALL}
					<h1 class="title">{_('Installing ZOOMRIYA OS')}</h1>
					<div class="slides">
						{#each SLIDES as [icon, title, text, [from, to]], i (icon)}
							<div class="slide" class:shown={i === slide} style:background="linear-gradient(135deg, {from}, {to})">
								<span class="slide-icon">{@html glyph(icon, 72, '#ffffff')}</span>
								<span class="slide-words"><span class="slide-title">{_(title)}</span><span class="slide-text">{_(text)}</span></span>
							</div>
						{/each}
					</div>
					<div class="dots">{#each SLIDES as s, i (s[0])}<span class:current={i === slide}></span>{/each}</div>
					<span class="stage">{box?.install.stage_name ?? ''}…  {Math.round((box?.install.overall ?? 0) * 100)}%</span>
					<div class="meter"><div style:width="{(box?.install.overall ?? 0) * 100}%"></div></div>
					<p class="dim">{_('Keep the computer on and the USB stick in until done.')}</p>
				{:else if step === DONE}
					<svg viewBox="0 0 96 96" width="96" height="96" aria-hidden="true">
						<circle cx="48" cy="48" r="48" fill={failed ? '#c0392b' : '#1f9d55'} fill-opacity=".15" />
						<circle cx="48" cy="48" r="36" fill={failed ? '#c0392b' : '#1f9d55'} />
						{#if failed}<path d="M48 30.7V53.8" stroke="#fff" stroke-width="6" stroke-linecap="round" /><circle cx="48" cy="65.3" r="3.5" fill="#fff" />
						{:else}<path d="M31.7 49.9L43.2 61.4 65.3 36.5" fill="none" stroke="#fff" stroke-width="6" stroke-linecap="round" stroke-linejoin="round" />{/if}
					</svg>
					<h1 class="title">{failed ? _("The install didn't finish") : _('ZOOMRIYA OS is installed')}</h1>
					<p class="lead">{failed ? failed + ' ' + _('The drive may not start now; installing again fixes it.') : _('Your ZOOMRIYA Box is ready for its first start.')}</p>
					<div class="steps">
						{#each failed ? [_("Check the drive's connection, or try another drive."), _('Press Try again.'),
							fill(_('If it fails again, support notes are in {file}.'), { file: box?.log ?? '' })]
							: [_('Take out the USB stick.'), _('Press Restart now to set it up, or Power off to leave it for its owner.'),
							_('On its next start, the box opens its first setup on this screen.')] as text, n (n)}
							<div><span class="step-number">{n + 1}</span><span class="fact">{text}</span></div>
						{/each}
					</div>
				{/if}
			</div>
			{#if step !== INSTALL}
				<footer class="buttons">
					{#if problem}<p class="error grow" role="alert">{problem}</p>{:else}<span class="grow"></span>{/if}
					{#if busy}<Spinner />{/if}
					{#if step >= WELCOME && step <= SUMMARY}<button onclick={() => go(step - 1)}>{_('Back')}</button>{/if}
					<!-- Done: switched off instead of restarted, as at the factory, so the box's first start is its owner's. -->
					{#if step === DONE && !failed}<button onclick={() => call('/api/power-off').catch(() => {})}>{_('Power off')}</button>{/if}
					<button class:primary={step !== SUMMARY} class:danger={step === SUMMARY} disabled={!ready || busy} onclick={confirm}>{nextWords}</button>
				</footer>
			{/if}
		</section>
	</div>
</div>

<style>
	.place { position: fixed; inset: 0; display: grid; place-items: center; padding: 24px; }
	.installer {
		width: min(1120px, 100%); height: min(760px, 100%); display: flex; border-radius: 26px; overflow: hidden;
		background: var(--card-glass); backdrop-filter: blur(40px) saturate(1.6);
		box-shadow: inset 0 0 0 1px var(--glass-edge), 0 0 0 1px var(--hairline), var(--shadow-window);
		animation: rise 360ms cubic-bezier(0.2, 0.9, 0.3, 1.05);
	}
	.side { width: 270px; flex: none; padding: 28px 18px; border-right: 1px solid var(--hairline); display: flex; flex-direction: column; gap: 24px; }
	.brand { display: flex; flex-direction: column; gap: 6px; padding: 0 10px; color: var(--muted); font-size: 13px; }
	ol { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 4px; }
	li { display: flex; align-items: center; gap: 12px; padding: 9px 10px; border-radius: 10px; color: var(--muted); }
	li.current { background: var(--selected); color: var(--text); font-weight: 600; }
	li.done { color: var(--text-secondary); }
	.marker { width: 26px; height: 26px; border-radius: 999px; display: grid; place-items: center; font-size: 12px; font-weight: 600; background: var(--hover); flex: none; }
	li.current .marker { background: var(--accent); color: var(--on-accent); }
	li.done .marker { background: var(--ok-bg); color: var(--ok); }
	.main { flex: 1; display: flex; flex-direction: column; min-width: 0; background: var(--panel); }
	.page { flex: 1; overflow-y: auto; padding: 40px 48px 24px; display: flex; flex-direction: column; gap: 16px; }
	.page .title { font-size: 28px; }
	.buttons { display: flex; align-items: center; gap: 10px; padding: 16px 24px; border-top: 1px solid var(--line); }
	.grow { flex: 1; }
	.choices, .disks { display: flex; flex-direction: column; gap: 4px; }
	.choices button { text-align: left; border: 1px solid var(--line); border-radius: 12px; min-height: 48px; font-weight: 500; }
	.choices button.on, .disk.on { border-color: var(--accent); background: var(--selected); }
	.disk { display: flex; align-items: center; gap: 16px; text-align: left; border: 1px solid var(--line); border-radius: 12px; padding: 12px 16px;
		min-height: 0; white-space: normal; font-weight: 400; }
	.disk-icon { display: inline-flex; }
	.disk-words { flex: 1; display: flex; flex-direction: column; gap: 2px; }
	.disk-name { font-weight: 600; }
	.disk-reason { color: var(--warn); font-size: 13px; }
	.disk-size { font-weight: 600; font-variant-numeric: tabular-nums; }
	.hero { border-radius: 18px; overflow: hidden; height: 220px; flex: none; }
	.hero svg { width: 100%; height: 100%; display: block; }
	.checks { display: flex; flex-direction: column; gap: 10px; }
	.checks > div { display: flex; align-items: center; gap: 12px; }
	.mark { width: 26px; height: 26px; border-radius: 999px; display: grid; place-items: center; font-weight: 700; flex: none;
		background: var(--ok-bg); color: var(--ok); }
	.mark.bad { background: var(--danger-bg); color: var(--danger); }
	.mark.warn { background: var(--warn-bg); color: var(--warn); }
	.mark.wait { background: var(--hover); color: var(--muted); }
	.bar-block { display: flex; flex-direction: column; gap: 8px; }
	.bar { display: flex; height: 30px; border-radius: 8px; overflow: hidden; gap: 2px; background: #dfe5ec; }
	.bar span { display: block; flex-basis: 0; }
	.legend { display: flex; flex-wrap: wrap; gap: 4px 18px; font-size: 13px; }
	.legend > span { display: inline-flex; align-items: center; gap: 8px; }
	.legend i { width: 12px; height: 12px; border-radius: 3px; display: inline-block; }
	.warning { display: flex; align-items: center; gap: 14px; background: #fdebd3; color: #7a4510; border-radius: 12px; padding: 14px 16px; font-weight: 500; cursor: pointer; }
	.slides { position: relative; height: 230px; flex: none; }
	.slide { position: absolute; inset: 0; border-radius: 18px; display: flex; align-items: center; gap: 28px; padding: 0 40px; color: #fff;
		opacity: 0; transform: translateX(24px); transition: opacity 600ms ease, transform 600ms ease; overflow: hidden; }
	.slide.shown { opacity: 1; transform: none; }
	.slide-words { display: flex; flex-direction: column; gap: 8px; max-width: 48ch; }
	.slide-title { font-size: 22px; font-weight: 700; }
	.slide-text { font-size: 15px; opacity: 0.92; }
	.dots { display: flex; gap: 8px; justify-content: center; }
	.dots span { width: 8px; height: 8px; border-radius: 999px; background: var(--line-strong); transition: width 300ms ease; }
	.dots span.current { width: 22px; background: var(--accent); }
	.stage { font-weight: 600; font-variant-numeric: tabular-nums; }
	.steps { display: flex; flex-direction: column; gap: 10px; }
	.steps > div { display: flex; gap: 12px; align-items: flex-start; }
	@media (max-width: 900px) {
		.side { width: 76px; padding: 20px 10px; }
		.brand, li > span:not(.marker) { display: none; }
	}
</style>

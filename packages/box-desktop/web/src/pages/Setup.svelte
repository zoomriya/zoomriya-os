<script lang="ts">
	// The first setup (design §5, the native setup's app.py), on the box's own
	// screen: language, keyboard, network, region, time zone, the box's admin
	// in ZOOMRIYA ID, the code from their phone, and done. A setup cut short (a
	// power cut) goes on from where it was. The language, the region and the
	// time zone are chosen by the person, never assumed.
	import { onDestroy } from 'svelte';
	import { get, post, type Answer } from '../lib/api';
	import { _, fill, lang, locale as uiLocale, N_, use } from '../lib/i18n.svelte';
	import { fixedSettings } from '../lib/network';
	import Logo from '../components/Logo.svelte';
	import Password from '../components/Password.svelte';
	import Spinner from '../components/Spinner.svelte';
	import Backdrop from './Backdrop.svelte';

	const STEPS = [N_('Language'), N_('Keyboard'), N_('Network'), N_('Region'), N_('Time zone'), N_('Admin'), N_('Phone'), N_('Done')];
	const [LANGUAGE, KEYBOARD, NETWORK, REGION, ZONE, ADMIN, PHONE, DONE] = [0, 1, 2, 3, 4, 5, 6, 7];
	const MIN_PASSWORD = 8;

	interface Choice { code: string; name: string }
	let step = $state(-1);
	let busy = $state(false);
	let problem = $state('');
	let languages = $state<Choice[]>([]);
	let regions = $state<Choice[]>([]);
	let zones = $state<string[]>([]);
	let language = $state('');
	let layouts = $state<Answer[]>([]);
	let layout = $state('');
	let network = $state<Answer>({});
	let online = $state<boolean | null>(null);
	let fixedOn = $state(false);
	let fixed = $state({ address: '', mask: '', router: '', dns: '' });
	let region = $state('');
	let regionSearch = $state('');
	let subdivisions = $state<Choice[]>([]);
	let subdivision = $state('');
	let zone = $state('');
	let zoneSearch = $state('');
	let regionZones = $state<string[]>([]);
	let everyZone = $state(false);
	let username = $state('');
	let password = $state('');
	let again = $state('');
	let admin = $state('');
	/** The admin's password, kept in memory only until their phone is set up. */
	let adminPassword = '';
	let ticket = $state('');
	let qr = $state('');
	let key = $state('');
	let code = $state('');
	let codes = $state<string[]>([]);
	let codesKept = $state(false);
	let phoneLater = $state(false);
	let phoneRetry = $state(false);
	let setupCode = $state('');
	let setupCodeKept = $state(false);
	let boxName = $state('');
	let now = $state(new Date());
	let poll: ReturnType<typeof setInterval> | null = null;
	const clock = setInterval(() => (now = new Date()), 1000);
	onDestroy(() => {
		clearInterval(clock);
		if (poll) clearInterval(poll);
	});

	const ask = (op: string, fields: Record<string, unknown> = {}) => post<Answer>('/api/setup/agent', { request: { op, ...fields } });
	const saveProgress = (done: Record<string, unknown>) => post('/api/setup/progress', done).catch(() => {});

	const ethernet = $derived.by(() => {
		const links = ((network.links ?? []) as Answer[]).filter((l) => l.kind === 'ethernet');
		return links.find((l) => l.connected) ?? links[0] ?? null;
	});
	const address = $derived(String(ethernet?.ipv4?.[0] ?? ''));

	function passwordProblem(): string | null {
		if (!username.trim()) return _('Choose a username.');
		if (!password) return _('Choose a password.');
		if (username.trim().length > 128) return _('The username can be at most 128 characters.');
		if (password.length < MIN_PASSWORD) return fill(_('The password needs at least {count} characters.'), { count: MIN_PASSWORD });
		if (!again) return _('Type the password again.');
		if (password !== again) return _("The two passwords aren't the same.");
		return null;
	}

	const ready = $derived.by(() => {
		if (busy) return false;
		if (step === LANGUAGE) return !!language;
		if (step === NETWORK) return !!address;
		if (step === REGION) return !!region;
		if (step === ZONE) return !!zone;
		if (step === ADMIN) return passwordProblem() === null;
		if (step === PHONE) return codes.length ? codesKept : !!ticket && !!code.trim();
		if (step === DONE) return !setupCode || setupCodeKept;
		return true;
	});

	async function call<T>(work: () => Promise<T>): Promise<T | undefined> {
		busy = true;
		problem = '';
		try {
			return await work();
		} catch (e) {
			problem = (e as Error).message;
			return undefined;
		} finally {
			busy = false;
		}
	}

	// --- moving between steps -------------------------------------------------

	async function start(): Promise<void> {
		const listed = await get<{ languages: Choice[]; regions: Choice[]; time_zones: string[] }>(`/api/places?lang=${lang()}`).catch(() => null);
		languages = listed?.languages ?? [];
		regions = listed?.regions ?? [];
		zones = listed?.time_zones ?? [];
		const [status, saved] = await Promise.all([ask('setup-status').catch(() => ({}) as Answer),
			get<{ progress: Answer }>('/api/setup').then((a) => a.progress).catch(() => ({}) as Answer)]);
		if (saved.language) await chooseLanguage(saved.language);
		if (status.region) {
			region = status.region;
			await showSubdivisions(status.region, status.subdivision ?? '');
		}
		if (saved.zone && status.zone) zone = status.zone;
		go(status.has_admin ? DONE : saved.keyboard ? NETWORK : saved.language ? KEYBOARD : LANGUAGE);
	}

	function go(next: number): void {
		step = Math.max(0, Math.min(next, STEPS.length - 1));
		problem = '';
		if (poll) clearInterval(poll);
		poll = null;
		if (step === KEYBOARD) void call(async () => {
			const answer = await ask('keyboard');
			layouts = answer.layouts ?? [];
			layout = answer.layout ?? layouts[0]?.id ?? '';
		});
		if (step === NETWORK) {
			void refreshNetwork();
			poll = setInterval(refreshNetwork, 2000);
		}
		if (step === ZONE) void showZones();
		if (step === PHONE) void startPhone();
		if (step === DONE) void showDone();
	}

	async function next(): Promise<void> {
		if (!ready) return;
		if (step === LANGUAGE) {
			const done = await call(async () => {
				const { locale } = await get<{ locale: string }>(`/api/places/locale?language=${language}&region=${region}`);
				await ask('language-set', { locale });
				return true;
			});
			if (done) {
				await saveProgress({ language });
				go(KEYBOARD);
			}
		} else if (step === KEYBOARD) {
			if (await call(() => (layout ? ask('keyboard-set', { layout }) : Promise.resolve({})))) {
				await saveProgress({ keyboard: true });
				go(NETWORK);
			}
		} else if (step === NETWORK) {
			// A change waits two minutes to be kept, then goes back by itself.
			await ask('network-keep').catch(() => {});
			go(REGION);
		} else if (step === REGION) {
			const done = await call(async () => {
				await ask('region-set', { region, subdivision });
				// The language as written there, if the region has its own: Tamil
				// in Sri Lanka is ta_LK, with Sri Lanka's dates and numbers.
				const { locale } = await get<{ locale: string }>(`/api/places/locale?language=${language}&region=${region}`);
				if (locale) await ask('language-set', { locale });
				return true;
			});
			if (done) go(ZONE);
		} else if (step === ZONE) {
			if (await call(() => ask('time-zone-set', { zone }))) {
				await saveProgress({ zone: true });
				go(ADMIN);
			}
		} else if (step === ADMIN) {
			await makeAdmin();
		} else if (step === PHONE) {
			if (codes.length) leavePhone();
			else await finishPhone();
		} else if (step === DONE) {
			if (await call(() => ask('setup-finish'))) location.reload();
		}
	}

	// --- Language -------------------------------------------------------------

	async function chooseLanguage(code: string): Promise<void> {
		language = code;
		// The setup in that language at once (English for one our screens don't come in).
		await use(code === 'zh' ? 'zh_CN' : code);
		const listed = await get<{ regions: Choice[] }>(`/api/places?lang=${lang()}`).catch(() => null);
		if (listed) regions = listed.regions;
	}

	// --- Network --------------------------------------------------------------

	async function refreshNetwork(): Promise<void> {
		try {
			const had = address;
			network = await ask('network');
			if (address && (!had || online === null)) void checkInternet();
			if (ethernet && !ethernet.connected) online = null;
		} catch {
			// Asked again in two seconds.
		}
	}

	async function checkInternet(): Promise<void> {
		online = null;
		online = (await get<{ online: boolean }>('/api/setup/internet').catch(() => ({ online: false }))).online;
	}

	async function applyFixed(): Promise<void> {
		if (!ethernet) return;
		let settings: Record<string, unknown>;
		try {
			settings = fixedSettings(fixed.address, fixed.mask, fixed.router, fixed.dns);
		} catch (e) {
			problem = (e as Error).message;
			return;
		}
		online = null;
		await call(() => ask('network-set', { link: ethernet!.name, ...settings }));
	}

	// --- Region and time zone --------------------------------------------------

	async function showSubdivisions(code: string, chosen = ''): Promise<void> {
		subdivisions = await get<Choice[]>(`/api/places/subdivisions?region=${code}&lang=${lang()}`).catch(() => []);
		subdivision = subdivisions.some((s) => s.code === chosen) ? chosen : '';
	}

	async function chooseRegion(code: string): Promise<void> {
		if (code !== region) {
			// A new region: its time zone is chosen again.
			zone = '';
			await showSubdivisions(code);
		}
		region = code;
	}

	async function showZones(): Promise<void> {
		regionZones = await get<string[]>(`/api/places/zones?region=${region}`).catch(() => []);
		everyZone = !regionZones.length;
	}

	const shownRegions = $derived(regions.filter((r) => r.name.toLowerCase().includes(regionSearch.toLowerCase())));
	const shownZones = $derived.by(() => {
		const words = zoneSearch.toLowerCase().replaceAll(' ', '_').split(/\s+/).filter(Boolean);
		return zones.filter((z) => (everyZone || regionZones.includes(z)) && words.every((w) => z.toLowerCase().includes(w)));
	});
	const regionName = $derived(regions.find((r) => r.code === region)?.name ?? '');
	const zoneTime = $derived.by(() => {
		if (!zone) return _('Choose one to go on.');
		try {
			const time = new Intl.DateTimeFormat(uiLocale(), { hour: '2-digit', minute: '2-digit', hourCycle: 'h23', timeZone: zone }).format(now);
			return fill(_('The time there now: {time}'), { time });
		} catch {
			return '';
		}
	});

	// --- Admin and phone ------------------------------------------------------

	async function makeAdmin(): Promise<void> {
		const name = username.trim();
		busy = true;
		problem = '';
		try {
			await ask('first-admin', { username: name, password });
			admin = name;
			adminPassword = password;
			password = again = '';
			go(PHONE);
		} catch (e) {
			const words = (e as Error).message;
			// Made before a power cut: the setup goes on from there.
			if (words.includes('already has an admin')) go(DONE);
			else problem = words;
		} finally {
			busy = false;
		}
	}

	async function startPhone(): Promise<void> {
		ticket = qr = key = code = '';
		phoneRetry = false;
		codes = [];
		if (!adminPassword) {
			// After a power cut the password is gone: Box Desktop asks at the first sign-in.
			skipPhone();
			return;
		}
		const answer = await call(() => post<Answer>('/api/setup/second-step/start', { username: admin, password: adminPassword }));
		if (!answer) {
			phoneRetry = true;
			return;
		}
		if (answer.already) {
			leavePhone();
			return;
		}
		ticket = answer.ticket;
		qr = answer.qr_svg;
		key = fill(_("Can't scan it? Type this key: {key}"), { key: answer.key });
	}

	async function finishPhone(): Promise<void> {
		const answer = await call(() => post<{ backup_codes: string[] }>('/api/setup/second-step/finish', { ticket, code }));
		if (!answer) {
			code = '';
			phoneRetry = true;
			return;
		}
		codes = answer.backup_codes;
		codesKept = false;
	}

	function skipPhone(): void {
		phoneLater = true;
		leavePhone();
	}

	/** On to the end; the password isn't kept any longer. */
	function leavePhone(): void {
		adminPassword = '';
		ticket = '';
		go(DONE);
	}

	// --- Done -----------------------------------------------------------------

	async function showDone(): Promise<void> {
		const [net, status] = await Promise.all([ask('network').catch(() => ({}) as Answer), ask('setup-status').catch(() => ({}) as Answer)]);
		network = net;
		boxName = net.name ?? '';
		// A setup code the box made itself (no sticker carries it): written down before Finish.
		setupCode = String(status.setup_code ?? '');
	}

	const doneFacts = $derived([
		...(admin ? [[_('Admin'), admin]] : []),
		[_("Box's name"), boxName],
		[_('Address'), address.split('/')[0] || _('Not connected')],
		[_('Language'), languages.find((l) => l.code === language)?.name ?? ''],
		[_('Region'), [subdivisions.find((s) => s.code === subdivision)?.name ?? '', regionName].filter(Boolean).join(', ')],
		[_('Time zone'), zone.replaceAll('_', ' ')]
	]);

	void start();
</script>

<Backdrop />
<div class="setup-place">
	<div class="setup">
		<aside class="setup-side">
			<div class="setup-brand"><Logo height={22} /><span>{_('Box setup')}</span></div>
			<ol>
				{#each STEPS as name, i (name)}
					<li class:current={i === step} class:done={i < step}>
						<span class="marker">{i < step ? '✓' : i + 1}</span>
						<span>{_(name)}</span>
					</li>
				{/each}
			</ol>
		</aside>
		<section class="setup-main">
			<div class="setup-page">
				{#if step === LANGUAGE}
					<h1 class="title">{_('Welcome to your ZOOMRIYA Box')}</h1>
					<p class="lead">{_('Setup takes a few minutes. First, choose a language.')}</p>
					<div class="choices" role="listbox" aria-label={_('Language')}>
						{#each languages as l (l.code)}
							<button class:on={language === l.code} role="option" aria-selected={language === l.code} onclick={() => chooseLanguage(l.code)}>{l.name}</button>
						{/each}
					</div>
					<p class="state">{language ? fill(_('Chosen: {name}'), { name: languages.find((l) => l.code === language)?.name ?? '' }) : _('Choose one to go on.')}</p>
				{:else if step === KEYBOARD}
					<h1 class="title">{_('Your keyboard')}</h1>
					<p class="lead">{_("Used right away, including for the admin's password.")}</p>
					<label class="field"><span class="field-label">{_('Keyboard')}</span>
						<select bind:value={layout}>{#each layouts as l (l.id)}<option value={l.id}>{l.name}</option>{/each}</select>
					</label>
				{:else if step === NETWORK}
					<h1 class="title">{_('Connect the box')}</h1>
					{#if !network.links}
						<p class="lead">{_("Checking the box's network port…")}</p>
					{:else if !ethernet}
						<p class="lead">{_('No usable Ethernet port found.')}</p>
					{:else if !ethernet.connected}
						<p class="lead">{_("Connect the box to your router with a network cable. Setup continues once it's plugged in.")}</p>
						<p class="state">{_('Waiting for the cable…')}</p>
					{:else if !address}
						<p class="lead">{_('Cable connected. Getting an address…')}</p>
					{:else}
						<p class="lead">{_('The box is on your network.')}</p>
						<div class="facts">
							<span class="fact-name">{_('Address')}</span><span class="fact">{address.split('/')[0]}</span>
							<span class="fact-name">{_('Router')}</span><span class="fact">{ethernet.gateway || _('None')}</span>
							<span class="fact-name">{_('Name servers')}</span><span class="fact">{(network.dns ?? []).join(', ') || _('None')}</span>
							<span class="fact-name">{_('Settings')}</span><span class="fact">{ethernet.mode === 'fixed' ? _('Static address') : _('From the router (DHCP)')}</span>
							{#if ethernet.speed_mbps}<span class="fact-name">{_('Speed')}</span><span class="fact">{ethernet.speed_mbps} Mbit/s</span>{/if}
						</div>
						<p class="state">{online === null ? _('Checking the internet…') : online ? _('Internet: connected.')
							: _('No internet. You can continue; updates and the clock catch up once online.')}</p>
					{/if}
					<label class="check"><input type="checkbox" bind:checked={fixedOn} />{_("Use a static address instead of the router's")}</label>
					{#if fixedOn}
						<div class="fixed">
							{#each [['address', _("Box's address"), '192.168.1.40'], ['mask', _('Subnet mask'), '255.255.255.0'],
								['router', _('Router'), '192.168.1.1'], ['dns', _('Name servers'), '192.168.1.1']] as [k, caption, hint] (k)}
								<label class="field"><span class="field-label">{caption}</span>
									<input type="text" placeholder={hint} bind:value={fixed[k as keyof typeof fixed]} /></label>
							{/each}
							<div><button onclick={applyFixed} disabled={busy}>{_('Use these settings')}</button></div>
						</div>
					{/if}
				{:else if step === REGION}
					<h1 class="title">{_('Which country or region?')}</h1>
					<p class="lead">{_('Where the box is. Sets the time zones shown first.')}</p>
					<!-- svelte-ignore a11y_autofocus -->
					<input type="search" bind:value={regionSearch} placeholder={_('Search, for example Canada')} aria-label={_('Search, for example Canada')} autofocus />
					<div class="choices tall" role="listbox" aria-label={_('Region')}>
						{#each shownRegions as r (r.code)}
							<button class:on={region === r.code} role="option" aria-selected={region === r.code} onclick={() => chooseRegion(r.code)}>{r.name}</button>
						{/each}
					</div>
					<p class="state">{region ? fill(_('Chosen: {name}'), { name: regionName }) : _('Choose one to go on.')}</p>
					{#if subdivisions.length}
						<label class="field"><span class="field-label">{_('State or province (optional)')}</span>
							<select bind:value={subdivision}>
								<option value="">{_('Not chosen')}</option>
								{#each subdivisions as s (s.code)}<option value={s.code}>{s.name}</option>{/each}
							</select>
						</label>
					{/if}
				{:else if step === ZONE}
					<h1 class="title">{_('Which time zone?')}</h1>
					<p class="lead">{regionZones.length ? fill(_("Time zones in {region}. Used by the box's clock, maintenance hour and apps."), { region: regionName })
						: _("Used by the box's clock, maintenance hour and apps.")}</p>
					<input type="search" bind:value={zoneSearch} placeholder={_('Search, for example Toronto')} aria-label={_('Search, for example Toronto')} />
					<label class="check"><input type="checkbox" bind:checked={everyZone} />{_('Show every time zone')}</label>
					<div class="choices tall" role="listbox" aria-label={_('Time zone')}>
						{#each shownZones as z (z)}
							<button class:on={zone === z} role="option" aria-selected={zone === z} onclick={() => (zone = z)}>{z.replaceAll('_', ' ').replaceAll('/', ' / ')}</button>
						{/each}
					</div>
					<p class="state">{zoneTime}</p>
				{:else if step === ADMIN}
					<h1 class="title">{_("The box's admin")}</h1>
					<p class="lead">{_("Your ZOOMRIYA ID signs in to the box and every ZOOMRIYA app. There's no other way in, so keep the password safe.")}</p>
					<label class="field"><span class="field-label">{_('Username')}</span>
						<!-- svelte-ignore a11y_autofocus -->
						<input type="text" bind:value={username} placeholder={_('For example, amma')} autocapitalize="none" autofocus /></label>
					<label class="field"><span class="field-label">{fill(_('Password (at least {count} characters)'), { count: MIN_PASSWORD })}</span>
						<Password bind:value={password} autocomplete="new-password" /></label>
					<label class="field"><span class="field-label">{_('The password again')}</span>
						<Password bind:value={again} autocomplete="new-password" enter={next} /></label>
					<p class="state">{passwordProblem() ?? ''}</p>
				{:else if step === PHONE}
					<h1 class="title">{_('Your phone')}</h1>
					<p class="lead">{_("Admins sign in with their password and a code from their phone, so someone who learns the password still can't get in. It takes a minute.")}</p>
					{#if !codes.length}
						<p style:white-space="pre-line" style:margin="0">{_("1. On your phone, open an authenticator app, any app that makes sign-in codes. If you don't have one, install one from your phone's app store.\n2. Add an account and scan this code.\n3. Enter the 6-digit code it shows.")}</p>
						<div class="phone">
							<div class="qr" style:width="198px" style:height="198px">{#if qr}{@html qr}{:else if busy}<Spinner />{/if}</div>
							<div class="phone-side">
								<span class="state selectable">{key}</span>
								<label class="field"><span class="field-label">{_('The code')}</span>
									<input type="text" inputmode="numeric" bind:value={code} placeholder={_('6-digit code')} autocomplete="one-time-code"
										onkeydown={(e) => e.key === 'Enter' && next()} /></label>
							</div>
						</div>
						<div class="row-between">
							{#if phoneRetry}<button onclick={startPhone}>{_('Try again')}</button>{/if}
							<button class="flat" onclick={skipPhone}>{_('Do it later')}</button>
						</div>
					{:else}
						<p class="lead">{_("Your backup codes. If you ever don't have your phone, each one signs you in once. Write them down and keep them somewhere safe at home. They won't be shown again.")}</p>
						<div class="facts codes selectable">{#each codes as c (c)}<span class="fact mono">{c}</span>{/each}</div>
						<label class="check"><input type="checkbox" bind:checked={codesKept} />{_("I've written them down")}</label>
					{/if}
				{:else if step === DONE}
					<svg class="tick" viewBox="0 0 96 96" width="96" height="96" aria-hidden="true">
						<circle cx="48" cy="48" r="48" fill="#1f9d55" fill-opacity=".15" /><circle cx="48" cy="48" r="36" fill="#1f9d55" />
						<path d="M31.7 49.9L43.2 61.4 65.3 36.5" fill="none" stroke="#fff" stroke-width="6" stroke-linecap="round" stroke-linejoin="round" />
					</svg>
					<h1 class="title">{_('Your box is ready')}</h1>
					<p class="lead">{phoneLater
						? _("Press Finish. From now on, sign in with the admin's username and password; Box Desktop sets up the code from your phone at the first sign-in. To start over, use Erase everything.")
						: _("Press Finish. From now on, sign in with the admin's username, password and the code from your phone. To start over, use Erase everything.")}</p>
					<div class="facts">{#each doneFacts as [name, value] (name)}<span class="fact-name">{name}</span><span class="fact">{value}</span>{/each}</div>
					{#if setupCode}
						<div class="facts code-card">
							<span class="fact-name">{_("Your box's setup code")}</span>
							<span class="title selectable">{setupCode}</span>
							<span>{_("Write it down and keep it with the box's papers, not on the box. You need it if an admin can't sign in, and to erase the box. It's shown only now.")}</span>
							<label class="check"><input type="checkbox" bind:checked={setupCodeKept} />{_("I've written down the setup code")}</label>
						</div>
					{/if}
				{/if}
			</div>
			<footer class="setup-buttons">
				{#if problem}<p class="error grow" role="alert">{problem}</p>{:else}<span class="grow"></span>{/if}
				{#if busy}<Spinner />{/if}
				<!-- The admin is made by then: no going back past it. -->
				{#if step > LANGUAGE && step < PHONE}<button onclick={() => go(step - 1)}>{_('Back')}</button>{/if}
				<button class="primary" disabled={!ready} onclick={next}>{step === DONE ? _('Finish') : _('Next')}</button>
			</footer>
		</section>
	</div>
</div>

<style>
	.setup-place { position: fixed; inset: 0; display: grid; place-items: center; padding: 24px; }
	.setup {
		width: min(1080px, 100%); height: min(720px, 100%); display: flex; border-radius: 26px; overflow: hidden;
		background: var(--card-glass); backdrop-filter: blur(40px) saturate(1.6);
		box-shadow: inset 0 0 0 1px var(--glass-edge), 0 0 0 1px var(--hairline), var(--shadow-window);
		animation: rise 360ms cubic-bezier(0.2, 0.9, 0.3, 1.05);
	}
	.setup-side { width: 270px; flex: none; padding: 28px 18px; border-right: 1px solid var(--hairline);
		display: flex; flex-direction: column; gap: 24px; }
	.setup-brand { display: flex; flex-direction: column; gap: 6px; padding: 0 10px; color: var(--muted); font-size: 13px; }
	ol { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 4px; }
	li { display: flex; align-items: center; gap: 12px; padding: 9px 10px; border-radius: 10px; color: var(--muted); }
	li.current { background: var(--selected); color: var(--text); font-weight: 600; }
	li.done { color: var(--text-secondary); }
	.marker { width: 26px; height: 26px; border-radius: 999px; display: grid; place-items: center; font-size: 12px; font-weight: 600;
		background: var(--hover); flex: none; }
	li.current .marker { background: var(--accent); color: var(--on-accent); }
	li.done .marker { background: var(--ok-bg); color: var(--ok); }
	.setup-main { flex: 1; display: flex; flex-direction: column; min-width: 0; background: var(--panel); }
	.setup-page { flex: 1; overflow-y: auto; padding: 40px 48px 24px; display: flex; flex-direction: column; gap: 14px; }
	.setup-page .title { font-size: 28px; }
	.setup-buttons { display: flex; align-items: center; gap: 10px; padding: 16px 24px; border-top: 1px solid var(--line); }
	.grow { flex: 1; }
	.choices { display: flex; flex-direction: column; gap: 2px; max-height: 300px; overflow-y: auto; border: 1px solid var(--line);
		border-radius: 12px; padding: 4px; }
	.choices.tall { max-height: 260px; flex: 1; }
	.choices button { text-align: left; border: none; background: none; border-radius: 8px; min-height: 36px; font-weight: 400; }
	.choices button:hover { background: var(--hover); }
	.choices button.on { background: var(--selected); color: var(--accent); font-weight: 600; }
	.state { color: var(--text-secondary); margin: 0; }
	.field { display: flex; flex-direction: column; gap: 6px; }
	.field select { width: 100%; }
	.fixed { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
	.phone { display: flex; gap: 24px; align-items: center; }
	.phone-side { display: flex; flex-direction: column; gap: 10px; flex: 1; }
	.qr { display: grid; place-items: center; background: #fff; border-radius: 12px; overflow: hidden; }
	.qr :global(svg) { width: 100%; height: 100%; }
	.codes { grid-template-columns: auto auto; justify-content: start; gap: 6px 32px; }
	.code-card { display: flex; flex-direction: column; gap: 10px; }
	.tick { flex: none; }
	@media (max-width: 900px) {
		.setup-side { width: 76px; padding: 20px 10px; }
		.setup-brand, li > span:not(.marker) { display: none; }
	}
</style>

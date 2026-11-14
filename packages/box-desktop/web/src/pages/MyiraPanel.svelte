<script lang="ts">
	// Myira's panel (box-desktop.md §4, Myira): one panel, whichever Myira is
	// on the box. box-desktopd decides who answers each question: the Myira
	// app when it's on for this admin (with their own chats), or the OS's own
	// Myira for the box. Either may pick one of the OS's tools; a change shows
	// on a card first, and only Confirm runs it. Never a link to another Myira.
	import { tick } from 'svelte';
	import { onDestroy } from 'svelte';
	import { ask, get, Problem, stream, type Answer } from '../lib/api';
	import { desk, TOOLS } from '../lib/desk.svelte';
	import { _, fill, lang, N_ } from '../lib/i18n.svelte';
	import { HELP } from '../lib/words';
	import Icon from '../components/Icon.svelte';
	import MyiraMark from '../components/MyiraMark.svelte';

	/** The OS's own Myira's fixed replies, by box-desktopd's "say" code. */
	const SAY: Record<string, string> = {
		not_box: N_('For now I can help with the box. With the ZOOMRIYA apps installed, I can answer much more.'),
		no_model: N_('Myira needs her model to answer (4.4 GB).'),
		unavailable: N_("Myira can't answer right now. Try again in a moment."),
		declined: N_('Nothing was changed.'),
		needs_password: N_("That asks for your password again, so it's done in its own place. Here it is."),
		emergency: N_('If someone is hurt or in danger, call your local emergency number now.')
	};
	/** The emergency line, with the number of where the family lives (BD-18). */
	const EMERGENCY_NUMBER = N_('If someone is hurt or in danger, call {number} now.');

	function fixedWords(event: Answer): string {
		if (event.say === 'emergency' && event.number) return fill(_(EMERGENCY_NUMBER), { number: event.number });
		return _(SAY[event.say] ?? SAY.unavailable);
	}

	interface Tool { app: string; name: string; title: string; arguments: Record<string, unknown>; writes: boolean }
	interface Said {
		who: 'you' | 'myira';
		text: string;
		busy?: boolean;
		source?: string;
		open?: { tool: string; section: string };
		/** A change waiting on the admin's Confirm. */
		card?: Tool;
		failed?: boolean;
	}
	interface Chat { id: number; title: string }

	let who = $state<'app' | 'os' | ''>('');
	let model = $state(true);
	let keepsChats = $state(false);
	let said = $state<Said[]>([]);
	let question = $state('');
	let busy = $state(false);
	let conversation: number | null = null;
	let questionId: number | null = null;
	let lastQuestion = '';
	/** The tools already used for this request, for a next one ("check, then change"). */
	let steps: Answer[] = [];
	let history = $state<Chat[] | null>(null);
	let list: HTMLElement | undefined = $state();

	// --- Myira's model, on a box that has none yet (BD-17) ---
	interface ModelState { models: { name: string; complete: boolean; bytes: number; total: number }[]; busy: string; download_blocked?: string; failed?: string }
	let fetching = $state<ModelState | null>(null);
	let sticks = $state<{ stick: string; device: string; name: string }[] | null>(null);
	let modelProblem = $state('');
	let polling: ReturnType<typeof setInterval> | undefined;
	onDestroy(() => clearInterval(polling));

	const progress = $derived.by(() => {
		const e2b = fetching?.models.find((m) => m.name === 'e2b');
		return e2b && e2b.total ? Math.floor((e2b.bytes / e2b.total) * 100) : 0;
	});

	async function modelState(): Promise<void> {
		try {
			fetching = (await ask('model')) as ModelState;
			if (fetching.busy && !polling) polling = setInterval(() => void modelState(), 2000);
			if (!fetching.busy && polling) {
				clearInterval(polling);
				polling = undefined;
				// Loaded once the model service has seen it (every 10 s).
				setTimeout(() => void look(), 12000);
			}
		} catch (e) {
			modelProblem = e instanceof Problem ? e.message : '';
		}
	}

	async function download(): Promise<void> {
		modelProblem = '';
		try {
			fetching = (await ask('model', { action: 'download', name: 'e2b' })) as ModelState;
			await modelState();
		} catch (e) {
			modelProblem = e instanceof Problem ? e.message : '';
		}
	}

	async function lookOnSticks(): Promise<void> {
		modelProblem = '';
		try {
			sticks = ((await ask('model', { action: 'usb' })).found ?? []) as typeof sticks;
		} catch (e) {
			modelProblem = e instanceof Problem ? e.message : '';
		}
	}

	async function copyFrom(device: string, name: string): Promise<void> {
		modelProblem = '';
		sticks = null;
		try {
			fetching = (await ask('model', { action: 'copy', device, name })) as ModelState;
			await modelState();
		} catch (e) {
			modelProblem = e instanceof Problem ? e.message : '';
		}
	}

	$effect(() => {
		if (desk.myiraOpen) void look();
	});

	async function look(): Promise<void> {
		try {
			const state = await get<{ who: 'app' | 'os'; model: boolean; chats: boolean }>('/api/myira');
			who = state.who;
			model = state.model;
			keepsChats = state.chats;
			if (!model) await modelState();
		} catch {
			who = 'os';
		}
	}

	function close(): void {
		desk.myiraOpen = false;
		// The OS's own Myira keeps no chats: closed is gone.
		if (who !== 'app') newChat();
	}

	function newChat(): void {
		said = [];
		conversation = null;
		questionId = null;
		history = null;
	}

	async function scrolled(): Promise<void> {
		await tick();
		list?.scrollTo({ top: list.scrollHeight });
	}

	/** Help's topics in the admin's language, for the OS's own Myira. */
	function help(): { title: string; text: string; tool: string; section: string }[] {
		return HELP.map(([title, text, tool, section]) => ({ title: _(title), text: _(text), tool, section }));
	}

	function zone(): string {
		return Intl.DateTimeFormat().resolvedOptions().timeZone ?? '';
	}

	/** One answer, as it streams: each event changes Myira's last line. */
	async function answer(path: string, body: Answer): Promise<void> {
		const line: Said = { who: 'myira', text: '', busy: true };
		said.push(line);
		const mine = said[said.length - 1];
		busy = true;
		let picked: Tool | null = null;
		void scrolled();
		try {
			await stream(path, body, (event) => {
				if (event.type === 'route') {
					conversation = event.conversation_id ?? conversation;
					questionId = event.question_id ?? questionId;
				} else if (event.type === 'text') {
					mine.text += event.text;
					void scrolled();
				} else if (event.type === 'step') {
					const { type: _type, ...step } = event;
					steps.push(step);
				} else if (event.type === 'tool') {
					picked = { app: event.app, name: event.name, title: event.title, arguments: event.arguments ?? {}, writes: !!event.writes };
				} else if (event.type === 'done') {
					if (event.say) mine.text = fixedWords(event);
					else if (event.answer) mine.text = event.answer;
					if (event.source?.name) mine.source = event.source.name;
					if (event.open?.tool) mine.open = event.open;
					if (event.tool && !picked) picked = event.tool;
				} else if (event.type === 'error') {
					mine.text = event.say ? fixedWords(event) : event.detail || _(SAY.unavailable);
					mine.failed = true;
				}
			});
		} catch (e) {
			mine.text = e instanceof Problem ? e.message : _(SAY.unavailable);
			mine.failed = true;
		}
		mine.busy = false;
		busy = false;
		const tool = picked as Tool | null;
		if (tool) {
			// Reading is asked for already; a change waits for Confirm.
			said.pop();
			if (tool.writes) said.push({ who: 'myira', text: '', card: tool });
			else await useTool(tool, false);
		}
		void scrolled();
	}

	async function send(): Promise<void> {
		const asked = question.trim();
		if (!asked || busy) return;
		question = '';
		said.push({ who: 'you', text: asked });
		history = null;
		// From a tool's ?, the tool is named for Myira (not shown again).
		const about = desk.myiraAbout && TOOLS[desk.myiraAbout] ? fill(_('In {tool}: '), { tool: _(TOOLS[desk.myiraAbout][0]) }) : '';
		lastQuestion = about + asked;
		steps = [];
		await answer('/api/myira/ask', {
			question: lastQuestion, lang: lang(), tz: zone(), conversation_id: conversation, help: help()
		});
	}

	async function useTool(tool: Tool, declined: boolean): Promise<void> {
		const card = said.findIndex((s) => s.card === tool);
		if (card >= 0) said.splice(card, 1);
		await answer('/api/myira/tool', {
			question: lastQuestion, lang: lang(), tz: zone(), conversation_id: conversation, question_id: questionId,
			app: tool.app, name: tool.name, title: tool.title, arguments: tool.arguments, declined,
			earlier: steps.slice(-2)
		});
	}

	async function showHistory(): Promise<void> {
		if (history) {
			history = null;
			return;
		}
		try {
			history = await get<Chat[]>('/api/myira/chats');
		} catch {
			history = [];
		}
	}

	async function openChat(id: number): Promise<void> {
		try {
			const chat = await get<{ id: number; turns: { role: string; content: string }[] }>(`/api/myira/chats/${id}`);
			said = chat.turns.map((t) => ({ who: t.role === 'user' ? 'you' : 'myira', text: t.content }));
			conversation = chat.id;
			questionId = null;
			history = null;
			void scrolled();
		} catch {
			history = null;
		}
	}

	function keyDown(e: KeyboardEvent): void {
		if (e.key === 'Escape') close();
	}

	/** What the change will be, as the admin reads it: its values ("04:30", "30%"). */
	function argumentsOf(tool: Tool): string {
		return Object.entries(tool.arguments).map(([k, v]) => (k === 'percent' ? `${v}%` : String(v))).join(', ');
	}
</script>

{#if desk.myiraOpen}
	<aside class="myira-panel" aria-label={_('Myira')}>
		<header class="myira-head">
			<span class="myira-head-mark"><MyiraMark px={20} /></span>
			<span class="myira-title grow">{_('Myira for ZOOMRIYA OS')}</span>
			{#if keepsChats}
				<button class="bar-button" class:on={!!history} title={_('History')} aria-label={_('History')} onclick={showHistory}>
					<Icon name="clock" px={18} />
				</button>
			{/if}
			<button class="bar-button" title={_('New chat')} aria-label={_('New chat')} onclick={newChat}><Icon name="plus" px={18} /></button>
			<button class="bar-button" title={_('Close')} aria-label={_('Close')} onclick={close}><Icon name="close" px={16} /></button>
		</header>

		<div class="myira-said" bind:this={list}>
			{#if history}
				{#if !history.length}<p class="lead">{_('No chats yet.')}</p>{/if}
				{#each history as chat (chat.id)}
					<button class="menu-item" onclick={() => openChat(chat.id)}>{chat.title}</button>
				{/each}
			{:else if !said.length}
				<div class="myira-hello">
					<!-- Myira's mark and name, and where she is: ZOOMRIYA OS (as the apps say "Myira for Family Photos"). -->
					<span class="myira-mark"><MyiraMark px={40} /></span>
					<div class="myira-brand">
						<span class="myira-name">{_('Myira')}</span>
						<span class="myira-for">{_('for ZOOMRIYA OS')}</span>
					</div>
					<p class="lead">{who === 'app' ? _('Ask me anything, about the box or anything else.') : _('Ask me about the box: storage, drives, backups, updates, the network and more.')}</p>
					{#if desk.myiraAbout && TOOLS[desk.myiraAbout]}
						<p class="field-label">{fill(_('Asking about {tool}'), { tool: _(TOOLS[desk.myiraAbout][0]) })}</p>
					{/if}
				</div>
			{/if}
			{#if !model && !history}
				<div class="myira-card">
					<span class="section">{_(SAY.no_model)}</span>
					{#if fetching?.busy}
						<span class="fact">{fill(fetching.busy === 'copy' ? _('Copying from the USB stick… {percent}%') : _('Downloading… {percent}%'), { percent: progress })}</span>
						<progress max="100" value={progress}></progress>
					{:else}
						{#if fetching?.failed}<span class="fact">{fetching.failed}</span>{/if}
						{#if fetching?.download_blocked}<span class="fact">{fetching.download_blocked}</span>{/if}
						<div class="row-between">
							<button onclick={lookOnSticks}>{_('From a USB stick')}</button>
							<button class="primary" disabled={!!fetching?.download_blocked} onclick={download}>{_('Download')}</button>
						</div>
						{#if sticks}
							{#if !sticks.length}<span class="fact">{_('No USB stick with the model was found.')}</span>{/if}
							{#each sticks as s (s.device + s.name)}
								<button onclick={() => copyFrom(s.device, s.name)}>{fill(_('Copy from {stick}'), { stick: s.stick || s.device })}</button>
							{/each}
						{/if}
					{/if}
					{#if modelProblem}<span class="fact">{modelProblem}</span>{/if}
				</div>
			{/if}
			{#if !history}
				{#each said as s, i (i)}
					{#if s.card}
						<div class="myira-card">
							<span class="section">{s.card.title}</span>
							{#if argumentsOf(s.card)}<span class="fact">{argumentsOf(s.card)}</span>{/if}
							<div class="row-between">
								<button onclick={() => useTool(s.card as Tool, true)} disabled={busy}>{_('Not now')}</button>
								<button class="primary" onclick={() => useTool(s.card as Tool, false)} disabled={busy}>{_('Confirm')}</button>
							</div>
						</div>
					{:else}
						<div class="myira-line {s.who}" class:failed={s.failed}>
							<p>{s.text}{#if s.busy}<span class="myira-dots" aria-label={_('Myira is answering')}>…</span>{/if}</p>
							{#if s.source}<span class="field-label">{fill(_('From Help: {name}'), { name: s.source })}</span>{/if}
							{#if s.open && TOOLS[s.open.tool]}
								<button onclick={() => desk.openTool(s.open!.tool, s.open!.section)}>{fill(_('Open {tool}'), { tool: _(TOOLS[s.open.tool][0]) })}</button>
							{/if}
						</div>
					{/if}
				{/each}
			{/if}
		</div>

		<!-- As the apps' Myira drawer: one line, its send button inside it. -->
		<form class="myira-ask" onsubmit={(e) => { e.preventDefault(); void send(); }}>
			<label class="myira-field">
				<input type="text" placeholder={_('Ask Myira…')} aria-label={_('Ask Myira…')} bind:value={question} onkeydown={keyDown} />
				<button type="submit" class="myira-send" title={_('Send')} aria-label={_('Send')} disabled={busy || !question.trim()}>
					<Icon name="send" px={18} />
				</button>
			</label>
		</form>
	</aside>
{/if}

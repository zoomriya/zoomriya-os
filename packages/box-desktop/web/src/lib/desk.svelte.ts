// Box Desktop's state and what it does (the native desktop's app.py): who's
// signed in and their settings, the windows (where they are, which is in
// front, minimised or maximised), the launcher's apps, the shortcuts, the
// widgets' figures, the notifications, sheets over the desktop, and locking
// when no one's using it. The pages draw it; everything they change goes
// through here, so each admin's layout is kept as the native desktop kept it.

import type { Component } from 'svelte';
import { ask, get, onSignedOut, patch, post, Problem, put, setToken, type Answer } from './api';
import { _, fill, N_, use as useLanguage } from './i18n.svelte';
import { setZone } from './format';
import { ACCENTS, DEFAULT_ACCENT } from './words';

/** The tools, in the menu's order: name, and the window's first size. */
export const TOOLS: Record<string, [string, number, number]> = {
	control: [N_('Control Panel'), 780, 900],
	storage: [N_('Storage Manager'), 780, 560],
	apps: [N_('App Center'), 800, 600],
	resources: [N_('Resource Monitor'), 720, 480],
	logs: [N_('Log Center'), 820, 580],
	support: [N_('Support Center'), 760, 620]
};
/** An app's window: its size before the screen makes it smaller. */
export const APP_WINDOW: [number, number] = [1180, 800];
export const TOP_BAR = 44;
/** The room kept between an opened window and the top bar, and the screen's bottom edge. */
export const GAP = 24;
/** The launcher along the bottom: the room above the screen's edge it keeps clear of windows. */
export const DOCK_ROOM = 92;
/** How long a window takes to grow in or shrink away. */
export const FADE_MS = 180;
export const DOCK_MAX = 8;
export const SHORTCUTS_MAX = 8;

/** Each tool's sections: id and name in English (translated where shown). */
export function sections(kind: string, withSound: boolean): [string, string][] {
	switch (kind) {
		case 'control':
			return [
				['network', N_('Network')], ['display', N_('Display')], ['appearance', N_('Appearance')],
				...(withSound ? [['sound', N_('Sound')] as [string, string]] : []),
				['keyboard', N_('Keyboard & Mouse')], ['language', N_('Language & Region')],
				['time', N_('Date and time')], ['people', N_('People')], ['notifications', N_('Notifications')],
				['security', N_('Security')], ['access', N_('Advanced access')], ['restarts', N_('Restarts')],
				['restore', N_('Restore & Reset')], ['about', N_('System Info')]
			];
		case 'storage':
			return [['overview', N_('Overview')], ['drives', N_('Drives')], ['space', N_('Space')], ['backups', N_('Backups')]];
		case 'apps':
			return [['installed', N_('Installed')], ['usb', N_('Add from USB')]];
		case 'resources':
			return [['overview', N_('Overview')], ['apps', N_('By app')], ['temperature', N_('Temperature')]];
		case 'support':
			return [['diagnostics', N_('Diagnostics')], ['contact', N_('Contact us')], ['help', N_('Help')]];
		case 'logs':
			return [
				['events', N_('Box events')], ['notifications', N_('Notifications')], ['audit', N_('Admin actions')],
				['signins', N_('Sign-ins')], ['activity', N_('Family activity')], ['system', N_('System log')],
				['diagnostics', N_('Diagnostics')]
			];
	}
	return [];
}

/** Each tool's sections' badges in its sidebar, by section id. */
export const SECTION_ICONS: Record<string, Record<string, string>> = {
	control: {
		network: 'ethernet', display: 'monitor', appearance: 'palette', sound: 'speaker', keyboard: 'keyboard',
		language: 'globe', time: 'clock', people: 'user', notifications: 'bell', security: 'shield',
		access: 'terminal', restarts: 'power', restore: 'restore-point', about: 'info'
	},
	storage: { overview: 'box', drives: 'disk', space: 'layers', backups: 'backup' },
	apps: { installed: 'app', usb: 'usb' },
	resources: { overview: 'speed', apps: 'app', temperature: 'thermometer' },
	support: { diagnostics: 'scan', contact: 'lifebuoy', help: 'help' },
	logs: {
		events: 'logs', notifications: 'bell', audit: 'user', signins: 'lock', activity: 'calendar-clock',
		system: 'terminal', diagnostics: 'scan'
	}
};

export interface Member {
	id: number;
	username: string;
	name: string;
}

export interface Prefs {
	version: number;
	theme: 'daylight' | 'midnight';
	accent: string;
	keyboard: string;
	handed: 'right' | 'left';
	mouse: { speed: number; accel: boolean; natural: boolean };
	windows: SavedWindow[];
	dock: string[];
	shortcuts: string[];
	notifications: { off: string[]; read_until: string };
}

export interface SavedWindow {
	kind: string;
	x: number;
	y: number;
	maximised: boolean;
	minimised: boolean;
	section: string;
}

export interface BoxSettings {
	background: string;
	lock_after: number;
	screen_off_after: number;
}

export interface App {
	id: string;
	name?: string;
	health?: string;
	[key: string]: unknown;
}

export interface Note {
	time?: string;
	kind?: string;
	level?: string;
	message?: string;
}

export interface Win {
	kind: string;
	x: number;
	y: number;
	width: number;
	height: number;
	preferred: [number, number];
	maximised: boolean;
	minimised: boolean;
	section: string;
	/** Growing in, or shrinking away (desktop.css's .opening, .closing). */
	opening: boolean;
	closing: boolean;
}

interface SessionAnswer {
	signed_in: boolean;
	side: 'home' | 'screen';
	member?: Member;
	token?: string;
	prefs?: Prefs;
	box: BoxSettings;
	language: string;
	os: string;
	has_picture: boolean;
	locked?: boolean;
	setup?: boolean;
}

export const DEFAULT_THEME = 'midnight';

export function defaultPrefs(): Prefs {
	return {
		version: 1, theme: DEFAULT_THEME, accent: DEFAULT_ACCENT, keyboard: '', handed: 'right',
		mouse: { speed: 0, accel: true, natural: false }, windows: [], dock: [], shortcuts: [],
		notifications: { off: [], read_until: '' }
	};
}

interface SheetEntry {
	id: number;
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	component: Component<any>;
	props: Record<string, unknown>;
}

/** What "your password again" asks, and what it does with it. */
export interface Confirming {
	title: string;
	text: string;
	button: string;
	/** One more field, such as the setup code. */
	extra?: string;
	action: (password: string, extra: string) => Promise<Answer | unknown>;
	done?: (answer: unknown) => void;
}

class Desk {
	page = $state<'loading' | 'sign-in' | 'desktop' | 'lock' | 'setup' | 'setup-elsewhere'>('loading');
	side = $state<'home' | 'screen'>('home');
	member = $state<Member | null>(null);
	prefs = $state<Prefs>(defaultPrefs());
	box = $state<BoxSettings>({ background: 'smoke', lock_after: 5, screen_off_after: 10 });
	hasPicture = $state(false);
	/** Changes when the family's own picture does, so it's drawn again. */
	pictureStamp = $state(Date.now());
	osName = $state('ZOOMRIYA OS');
	boxName = $state('');
	/** The windows in the order they were opened (the top bar's), and front to back. */
	windows = $state<Win[]>([]);
	order = $state<string[]>([]);
	active = $state('');
	apps = $state<Record<string, App>>({});
	notes = $state<Note[]>([]);
	sheets = $state<SheetEntry[]>([]);
	widgetsShown = $state(true);
	menuOpen = $state(false);
	hasSound = $state(false);
	/** Where the apps' pages are, when not the box's own address (the preview). */
	appBase = $state('');
	/** Words for Log Center's system log to search, from the menu's search. */
	logSearch = $state('');
	/** The desktop below the top bar (window.innerWidth, minus the bars). */
	deskWidth = $state(1440);
	deskHeight = $state(900 - TOP_BAR - DOCK_ROOM);
	/** The widgets' figures (refreshWidgets, refreshCpu, refreshNetwork). */
	status = $state<Answer>({});
	storage = $state<Answer>({});
	health = $state<Answer>({});
	network = $state<Answer>({});
	resources = $state<Answer>({});
	cpuHistory = $state<number[]>([]);
	netHistory = $state<[number, number][]>([]);

	private restoring = false;
	private saveTimer: ReturnType<typeof setTimeout> | null = null;
	private timers: ReturnType<typeof setInterval>[] = [];
	private sheetCount = 0;
	private lastUsed = Date.now();
	private screenOff = false;
	private events: EventSource | null = null;

	get theme(): 'daylight' | 'midnight' {
		return this.member ? this.prefs.theme : DEFAULT_THEME;
	}

	get accent(): string {
		return this.member && this.prefs.accent in ACCENTS ? this.prefs.accent : DEFAULT_ACCENT;
	}

	// --- starting, signing in and out --------------------------------------------

	async start(): Promise<void> {
		onSignedOut(() => this.signedOutHere(), () => this.lockedHere());
		const answer = await get<SessionAnswer>('/api/session').catch(() => null);
		if (!answer) {
			await useLanguage('en');
			this.page = 'sign-in';
			return;
		}
		await this.showSession(answer);
		this.watchForUse();
		this.timers.push(setInterval(() => this.checkIdle(), 5000));
	}

	private async showSession(answer: SessionAnswer): Promise<void> {
		this.side = answer.side;
		this.box = answer.box;
		this.osName = answer.os || 'ZOOMRIYA OS';
		this.hasPicture = answer.has_picture;
		await useLanguage(answer.language);
		if (answer.setup) {
			// Not set up yet: the first setup, at the box's own screen only.
			this.page = answer.side === 'screen' ? 'setup' : 'setup-elsewhere';
		} else if (answer.signed_in && answer.member) {
			this.enter(answer.member, answer.token ?? '', answer.prefs ?? defaultPrefs());
			if (answer.locked) this.page = 'lock';
		} else {
			this.page = 'sign-in';
		}
	}

	/** Signed in by the sign-in page: this admin's desktop, theirs as they left it. */
	signedIn(answer: SessionAnswer): void {
		this.box = answer.box;
		this.hasPicture = answer.has_picture;
		if (answer.member) this.enter(answer.member, answer.token ?? '', answer.prefs ?? defaultPrefs());
	}

	private enter(member: Member, token: string, prefs: Prefs): void {
		setToken(token);
		this.member = member;
		this.prefs = { ...defaultPrefs(), ...prefs };
		this.page = 'desktop';
		this.lastUsed = Date.now();
		void this.loadBoxFacts();
		this.restoreLayout(this.prefs.windows ?? []);
		this.refreshApps();
		this.refreshWidgets();
		this.refreshCpu();
		this.refreshNetwork();
		this.stopTimers();
		this.timers.push(
			setInterval(() => this.refreshCpu(), 3000),
			setInterval(() => this.refreshWidgets(), 60000),
			setInterval(() => this.refreshNetwork(), 10000),
			setInterval(() => this.checkIdle(), 5000)
		);
		this.listen();
	}

	private async loadBoxFacts(): Promise<void> {
		try {
			const info = await get<Answer>('/api/info');
			this.hasSound = !!info.sound;
			this.appBase = info.app_base ?? '';
			if (info.os) this.osName = info.os;
		} catch {
			// Shown without them.
		}
		try {
			const net = await ask('network');
			this.boxName = net.name ?? '';
			this.network = net;
		} catch {
			// The name shows once the agent answers.
		}
		try {
			const setup = await ask('setup-status');
			setZone(setup.zone);
		} catch {
			// The browser's own time zone, meanwhile.
		}
	}

	/** The page's event stream: while it's open the sign-in isn't idle, and
	 * the moment it ends (elsewhere, or run out) the page says so. */
	private listen(): void {
		this.events?.close();
		this.events = new EventSource('/api/events');
		this.events.addEventListener('signed-out', () => this.signedOutHere());
	}

	private stopTimers(): void {
		for (const t of this.timers) clearInterval(t);
		this.timers = [];
		this.timers.push(setInterval(() => this.checkIdle(), 5000));
	}

	/** Ended by box-desktopd: back to the sign-in, nothing of theirs left on the page. */
	private signedOutHere(): void {
		if (!this.member) return;
		this.leave();
	}

	private lockedHere(): void {
		if (this.member && this.page === 'desktop') this.page = 'lock';
	}

	async signOut(): Promise<void> {
		if (this.member) {
			this.rememberLayout();
			await this.savePrefsNow();
		}
		try {
			await post('/api/sign-out');
		} catch {
			// Signed out here either way.
		}
		this.leave();
	}

	private leave(): void {
		this.events?.close();
		this.events = null;
		this.member = null;
		this.restoring = true;
		this.windows = [];
		this.order = [];
		this.active = '';
		this.restoring = false;
		this.sheets = [];
		this.prefs = defaultPrefs();
		this.apps = {};
		this.notes = [];
		this.menuOpen = false;
		this.status = this.storage = this.health = this.resources = {};
		this.cpuHistory = [];
		this.netHistory = [];
		setToken('');
		this.stopTimers();
		this.page = 'sign-in';
	}

	// --- locking -----------------------------------------------------------------

	/** Locks the screen; the admin stays signed in and nothing stops. */
	async lock(): Promise<void> {
		if (!this.member) return;
		this.menuOpen = false;
		this.rememberLayout();
		await this.savePrefsNow();
		this.page = 'lock';
		try {
			await post('/api/lock');
		} catch {
			// Locked on this page all the same.
		}
	}

	/** The lock screen's password: right, and the desktop is back as it was. */
	async unlock(password: string): Promise<void> {
		await post('/api/unlock', { password });
		this.lastUsed = Date.now();
		this.page = 'desktop';
	}

	private watchForUse(): void {
		const used = () => {
			this.lastUsed = Date.now();
			if (this.screenOff) {
				this.screenOff = false;
				void post('/api/screen/power', { on: true }).catch(() => {});
			}
		};
		for (const name of ['pointermove', 'pointerdown', 'keydown', 'wheel', 'touchstart']) {
			window.addEventListener(name, used, { capture: true, passive: true });
		}
	}

	/** With no one using it, this page locks; on the box's own screen, the
	 * screen then turns off too (Control Panel › Display); the box runs on. */
	private checkIdle(): void {
		const idle = (Date.now() - this.lastUsed) / 1000;
		const lockAfter = this.box.lock_after * 60;
		const offAfter = this.box.screen_off_after * 60;
		if (lockAfter && idle >= lockAfter && this.member && this.page === 'desktop') void this.lock();
		if (this.side === 'screen' && this.member && offAfter && idle >= offAfter && !this.screenOff) {
			this.screenOff = true;
			void post('/api/screen/power', { on: false }).catch(() => {});
		}
	}

	// --- each admin's own desktop ------------------------------------------------

	setTheme(theme: 'daylight' | 'midnight'): void {
		if (!this.member) return;
		this.prefs.theme = theme;
		this.savePrefs();
	}

	changeLook(accent: string): void {
		if (!this.member || !(accent in ACCENTS)) return;
		this.prefs.accent = accent;
		this.savePrefs();
	}

	/** The box's background, for everyone: the sign-in screen, the desktop and the lock screen. */
	async changeBackground(key: string): Promise<void> {
		if (!this.member) return;
		const before = this.box.background;
		this.box.background = key;
		try {
			await this.patchBox({ background: key });
		} catch {
			this.box.background = before;
		}
	}

	/** The box's own settings (box.json): its background, and when the screen locks and turns off. */
	async patchBox(changes: Partial<BoxSettings>): Promise<BoxSettings> {
		this.box = await patch<BoxSettings>('/api/box', changes);
		return this.box;
	}

	/** Soon, once: moving a window saves when it stops. */
	savePrefs(): void {
		if (this.saveTimer) clearTimeout(this.saveTimer);
		this.saveTimer = setTimeout(() => void this.savePrefsNow(), 800);
	}

	async savePrefsNow(): Promise<void> {
		if (this.saveTimer) clearTimeout(this.saveTimer);
		this.saveTimer = null;
		if (!this.member) return;
		try {
			const saved = await put<Prefs>('/api/prefs', $state.snapshot(this.prefs));
			// What box-desktopd kept (checked), without moving what's on screen.
			this.prefs.dock = saved.dock;
			this.prefs.shortcuts = saved.shortcuts;
		} catch {
			// The desktop works on without its memory.
		}
	}

	rememberLayout(): void {
		if (this.restoring || !this.member) return;
		this.prefs.windows = this.order
			.map((kind) => this.windows.find((w) => w.kind === kind))
			.filter((w): w is Win => !!w)
			.map((w) => ({
				kind: w.kind, x: Math.round(w.x), y: Math.round(w.y), maximised: w.maximised,
				minimised: w.minimised, section: w.section
			}));
		this.savePrefs();
	}

	private restoreLayout(saved: SavedWindow[]): void {
		this.restoring = true;
		for (const entry of saved) {
			if (!(entry.kind in TOOLS)) continue;
			this.openTool(entry.kind, entry.section, true);
			const win = this.win(entry.kind);
			if (!win) continue;
			win.x = Number(entry.x ?? win.x);
			win.y = Number(entry.y ?? win.y);
			win.maximised = !!entry.maximised;
			win.minimised = !!entry.minimised;
			this.fitWindow(win);
		}
		this.active = [...this.order].reverse().find((k) => !this.win(k)?.minimised) ?? '';
		this.restoring = false;
	}

	// --- windows -------------------------------------------------------------------

	win(kind: string): Win | undefined {
		return this.windows.find((w) => w.kind === kind);
	}

	/** A tool's or an app's name. */
	title(kind: string): string {
		if (kind.startsWith('app:')) return this.apps[kind.slice(4)]?.name || kind.slice(4);
		return TOOLS[kind] ? _(TOOLS[kind][0]) : kind;
	}

	openTool(kind: string, section?: string | null, restoring = false): void {
		if (!(kind in TOOLS) && !(kind.startsWith('app:') && kind.slice(4))) return;
		this.menuOpen = false;
		// Reset is part of Restore & Reset now (a layout kept before may name it).
		if (section === 'reset') section = 'restore';
		let win = this.win(kind);
		if (!win) {
			const [width, height] = kind.startsWith('app:') ? APP_WINDOW : [TOOLS[kind][1], TOOLS[kind][2]];
			const n = this.windows.length;
			const listed = sections(kind, this.hasSound);
			this.windows.push({
				kind, x: 80 + 28 * n, y: GAP + 28 * n, width, height, preferred: [width, height], maximised: false,
				minimised: false, section: listed[0]?.[0] ?? '', opening: !restoring, closing: false
			});
			win = this.win(kind)!;
			this.order.push(kind);
			this.fitWindow(win);
		}
		if (win.minimised || win.opening) this.fadeIn(win);
		win.minimised = false;
		if (section && sections(kind, this.hasSound).some(([id]) => id === section)) win.section = section;
		this.raise(kind);
	}

	private fadeIn(win: Win): void {
		win.closing = false;
		win.opening = true;
		setTimeout(() => {
			const shown = this.win(win.kind);
			if (shown) shown.opening = false;
		}, 30);
	}

	private fadeOut(win: Win, then: () => void): void {
		if (this.restoring) {
			then();
			return;
		}
		win.closing = true;
		setTimeout(() => {
			const shown = this.win(win.kind);
			if (shown) shown.closing = false;
			then();
		}, FADE_MS);
	}

	raise(kind: string): void {
		const win = this.win(kind);
		if (!win) return;
		this.order = [...this.order.filter((k) => k !== kind), kind];
		this.active = kind;
		this.rememberLayout();
	}

	showSection(kind: string, section: string): void {
		const win = this.win(kind);
		if (!win) return;
		win.section = section;
		this.rememberLayout();
	}

	tabClicked(kind: string): void {
		const win = this.win(kind);
		if (!win) return;
		if (!win.minimised && this.active === kind) {
			this.minimise(kind);
		} else {
			if (win.minimised) this.fadeIn(win);
			win.minimised = false;
			this.raise(kind);
		}
	}

	dockClicked(kind: string): void {
		if (this.win(kind)) this.tabClicked(kind);
		else this.openTool(kind);
	}

	minimise(kind: string): void {
		const win = this.win(kind);
		if (!win) return;
		if (this.active === kind) this.active = '';
		this.fadeOut(win, () => {
			win.minimised = true;
			this.rememberLayout();
		});
	}

	maximise(kind: string): void {
		const win = this.win(kind);
		if (!win) return;
		win.maximised = !win.maximised;
		this.fitWindow(win);
		this.rememberLayout();
	}

	close(kind: string): void {
		const win = this.win(kind);
		if (!win) return;
		if (this.active === kind) this.active = '';
		this.fadeOut(win, () => {
			this.windows = this.windows.filter((w) => w.kind !== kind);
			this.order = this.order.filter((k) => k !== kind);
			this.rememberLayout();
		});
	}

	/** The window in front, if one is shown. */
	front(): string {
		const win = this.win(this.active);
		return win && !win.minimised ? this.active : '';
	}

	/** The next window in the top bar's order, brought back if it was minimised. */
	nextWindow(): void {
		if (!this.windows.length) return;
		const kinds = this.windows.map((w) => w.kind);
		const start = kinds.indexOf(this.active);
		const kind = kinds[(start + 1) % kinds.length];
		const win = this.win(kind)!;
		if (win.minimised) {
			this.fadeIn(win);
			win.minimised = false;
		}
		this.raise(kind);
	}

	/** A maximised window fills the desktop. Others are no wider than the
	 * screen with GAP free at each side, no taller than the desktop with GAP
	 * free above and below, and moved up if they'd reach past the bottom. */
	fitWindow(win: Win): void {
		const width = this.deskWidth, height = this.deskHeight;
		if (win.maximised) return;
		if (width > 400) win.width = Math.min(win.preferred[0], width - 2 * GAP);
		const room = height - GAP;
		if (room > 300) {
			win.height = Math.min(win.preferred[1], room - GAP);
			win.y = Math.max(GAP, Math.min(win.y, room - win.height));
		}
	}

	resized(width: number, height: number): void {
		this.deskWidth = width;
		this.deskHeight = height - TOP_BAR - DOCK_ROOM;
		for (const win of this.windows) this.fitWindow(win);
	}

	drag(kind: string, x0: number, y0: number, dx: number, dy: number): void {
		const win = this.win(kind);
		if (!win || win.maximised) return;
		win.x = Math.max(120 - win.width, Math.min(x0 + dx, this.deskWidth - 120));
		win.y = Math.max(0, Math.min(y0 + dy, this.deskHeight + DOCK_ROOM - 46));
	}

	// --- sheets over the desktop -------------------------------------------------

	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	sheet(component: Component<any>, props: Record<string, unknown> = {}): () => void {
		const id = ++this.sheetCount;
		const close = () => {
			this.sheets = this.sheets.filter((s) => s.id !== id);
		};
		this.sheets.push({ id, component, props: { ...props, close } });
		return close;
	}

	/** Closes the sheet on top, if one's open (Escape). */
	closeTopSheet(): boolean {
		if (!this.sheets.length) return false;
		this.sheets = this.sheets.slice(0, -1);
		return true;
	}

	/** The admin's password again (and maybe one more field), then action. */
	confirming: ((c: Confirming) => void) | null = null;

	confirm(c: Confirming): void {
		this.confirming?.(c);
	}

	restart(): void {
		this.confirm({
			title: _('Restart the box?'),
			text: _('Everything stops for a minute or two, then starts again.'),
			button: _('Restart'),
			action: (password) => ask('restart', {}, password)
		});
	}

	/** Straight after a restore or a reset, whose password was just asked: not asked again. */
	restartNow(): void {
		void ask('restart').catch(() => {});
	}

	shutDown(): void {
		this.confirm({
			title: _('Shut the box down?'),
			text: _('All apps stop and backups pause until someone presses the power button.'),
			button: _('Shut down'),
			action: (password) => ask('shutdown', {}, password)
		});
	}

	// --- apps, the dock and shortcuts ----------------------------------------------

	refreshApps(): void {
		ask('apps').then((a) => this.gotApps(a.apps ?? [])).catch(() => {});
	}

	gotApps(apps: App[]): void {
		this.apps = Object.fromEntries(apps.filter((a) => a.id).map((a) => [a.id, a]));
		// An app removed from the box leaves the dock too.
		const pinned = this.prefs.dock.filter((id) => id in this.apps);
		if (this.member && pinned.length !== this.prefs.dock.length) {
			this.prefs.dock = pinned;
			this.savePrefs();
		}
		for (const w of this.windows) if (w.kind.startsWith('app:') && !(w.kind.slice(4) in this.apps)) this.close(w.kind);
	}

	inDock(id: string): boolean {
		return this.prefs.dock.includes(id);
	}

	pinApp(id: string, on: boolean): void {
		if (!this.member) return;
		let dock = this.prefs.dock.filter((i) => i !== id);
		if (on) dock = [...dock, id].slice(0, DOCK_MAX);
		this.prefs.dock = dock;
		this.savePrefs();
	}

	addShortcut(kind: string): boolean {
		if (!this.member) return false;
		if (!(kind in TOOLS) && !(kind.startsWith('app:') && kind.slice(4) in this.apps)) return false;
		const shortcuts = [...this.prefs.shortcuts.filter((k) => k !== kind), kind];
		this.prefs.shortcuts = shortcuts.slice(-SHORTCUTS_MAX);
		this.savePrefs();
		return true;
	}

	removeShortcut(kind: string): void {
		this.prefs.shortcuts = this.prefs.shortcuts.filter((k) => k !== kind);
		this.savePrefs();
	}

	/** Where an app is: the box's address and the app's id, as a phone opens it. */
	appAddress(id: string): string {
		if (this.appBase) return `${this.appBase}${id}/`;
		return `https://${this.boxName || location.hostname}/${id}/`;
	}

	// --- the widgets, the bell and the pill --------------------------------------

	refreshWidgets(): void {
		if (!this.member) return;
		Promise.all([ask('status'), ask('storage'), ask('disk-health'), ask('notifications')])
			.then(([status, storage, health, notes]) => {
				this.status = status;
				this.storage = storage;
				this.health = health;
				if (status.os) this.osName = status.os;
				this.notes = notes.notifications ?? [];
				const check = storage.library?.check ?? {};
				// One follow-up at a time while the library's check runs, for its progress.
				if (check.running && !check.paused) setTimeout(() => this.refreshWidgets(), 10000);
			})
			.catch(() => {});
	}

	refreshCpu(): void {
		if (!this.member) return;
		ask('resources')
			.then((answer) => {
				this.resources = answer;
				this.cpuHistory = [...this.cpuHistory, answer.cpu_percent ?? 0].slice(-60);
				const net = answer.network ?? {};
				this.netHistory = [...this.netHistory, [Number(net.in_bps ?? 0), Number(net.out_bps ?? 0)] as [number, number]].slice(-120);
			})
			.catch(() => {});
	}

	refreshNetwork(): void {
		if (!this.member) return;
		ask('network').then((a) => (this.network = a)).catch(() => {});
	}

	/** The bell's notifications: the kinds the admin hasn't turned off, newest first. */
	shownNotes(): Note[] {
		const off = new Set(this.prefs.notifications?.off ?? []);
		return this.notes
			.filter((n) => !off.has(n.kind ?? ''))
			.sort((a, b) => (b.time ?? '').localeCompare(a.time ?? ''));
	}

	unread(): Note[] {
		const until = this.prefs.notifications?.read_until ?? '';
		return this.shownNotes().filter((n) => (n.time ?? '') > until);
	}

	markRead(): void {
		const newest = this.notes.reduce((m, n) => ((n.time ?? '') > m ? (n.time ?? '') : m), '');
		this.prefs.notifications = { ...this.prefs.notifications, read_until: newest };
		this.savePrefs();
	}

	turnNotifications(kind: string, on: boolean): void {
		const off = new Set(this.prefs.notifications?.off ?? []);
		if (on) off.delete(kind);
		else off.add(kind);
		this.prefs.notifications = { ...this.prefs.notifications, off: [...off].sort() };
		this.savePrefs();
	}

	/** Log Center's system log, searched for text. */
	searchLog(text: string): void {
		this.logSearch = text;
		this.openTool('logs', 'system');
	}

	/** The box's language changed (Language & Region): every word in it, at once. */
	async languageChanged(code: string): Promise<void> {
		await useLanguage(code);
	}
}

export const desk = new Desk();

/** For words built from parts in places where only a string fits. */
export function named(english: string, parts: Record<string, string | number>): string {
	return fill(_(english), parts);
}

export { Problem };

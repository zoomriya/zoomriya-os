// The web desktop's own rules, as the native desktop's tests checked them
// (tests/box-desktop before): the menu's search, the accents, sizes and
// times, app tiles, the fixed address and People's rules.

import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import { sections, TOOLS } from './desk.svelte';
import { appLetters, duration, initials, size } from './format';
import { fixedSettings, maskOf, prefixLength } from './network';
import { detailsProblem, guardianChoices, inOrder, passwordProblem } from './people';
import { fold, search } from './search';
import { appSvg, crc32 } from './tiles';
import { ACCENTS, HELP, SECTION_WORDS, SETTINGS } from './words';

function everySection(): Map<string, string> {
	const found = new Map<string, string>();
	for (const kind of Object.keys(TOOLS)) for (const [id, title] of sections(kind, true)) found.set(`${kind} ${id}`, title);
	return found;
}

const find = (typed: string) =>
	search(typed, Object.fromEntries(Object.entries(TOOLS).map(([k, [e]]) => [k, e])), everySection(), HELP);

describe("the top bar's search", () => {
	it('points every entry at a real section', () => {
		const known = everySection();
		for (const [kind, section] of SECTION_WORDS) expect(known.has(`${kind} ${section}`), `${kind} ${section}`).toBe(true);
		for (const [kind, section] of SETTINGS) expect(known.has(`${kind} ${section}`), `${kind} ${section}`).toBe(true);
		for (const [, , kind, section] of HELP) expect(known.has(`${kind} ${section}`), `${kind} ${section}`).toBe(true);
	});

	it('can find every section', () => {
		expect(new Set(SECTION_WORDS.map(([k, s]) => `${k} ${s}`))).toEqual(new Set(everySection().keys()));
	});

	it('finds what people type', () => {
		for (const [typed, kind, section] of [
			['ssh', 'control', 'access'], ['reboot', 'control', 'restarts'], ['wifi', 'control', 'network'],
			['time zone', 'control', 'time'], ['firewall', 'control', 'security'], ['diagnostics', 'support', 'diagnostics'],
			['serial', 'support', 'contact'], ['RESOLUTION', 'control', 'display'], ['factory reset', 'control', 'restore'],
			['hot', 'resources', 'temperature'], ['wallpaper', 'control', 'appearance'], ['accent', 'control', 'appearance'],
			['dark', 'control', 'appearance']
		]) {
			expect(find(typed).map((f) => `${f.kind} ${f.section}`), typed).toContain(`${kind} ${section}`);
		}
	});

	it('finds actions, and nothing for nothing', () => {
		expect(find('log out').map((f) => f.action)).toContain('sign-out');
		expect(find('dark').map((f) => f.action)).toContain('midnight');
		expect(find('lock').map((f) => f.action)).toContain('lock');
		expect(find('lock').map((f) => `${f.kind} ${f.section}`)).toContain('control display');
		expect(find('backup').map((f) => `${f.kind} ${f.section}`)).toContain('storage backups');
		expect(find('zzzq')).toEqual([]);
		expect(find('   ')).toEqual([]);
		expect(fold('Réseau')).toBe('reseau');
	});
});

describe('the accents', () => {
	const lum = (colour: string) => {
		const channel = (i: number) => {
			const c = parseInt(colour.slice(i, i + 2), 16) / 255;
			return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
		};
		return 0.2126 * channel(1) + 0.7152 * channel(3) + 0.0722 * channel(5);
	};
	const contrast = (a: string, b: string) => {
		const [high, low] = [lum(a), lum(b)].sort((x, y) => y - x);
		return (high + 0.05) / (low + 0.05);
	};

	it('each reads at 4.5:1 in both themes', () => {
		for (const [key, [, day, , night]] of Object.entries(ACCENTS)) {
			expect(contrast(day, '#f2f4f7'), key).toBeGreaterThanOrEqual(4.5);
			expect(contrast(day, '#ffffff'), key).toBeGreaterThanOrEqual(4.5);
			expect(contrast(night, '#21283a'), key).toBeGreaterThanOrEqual(4.5);
			expect(contrast(night, '#0b0e1a'), key).toBeGreaterThanOrEqual(4.5);
		}
	});
});

describe('words for numbers', () => {
	it('sizes', () => {
		expect(size(0)).toBe('0 bytes');
		expect(size(999)).toBe('999 bytes');
		expect(size(1500)).toBe('1.5 KB');
		expect(size(2_000_000_000_000)).toBe('2.0 TB');
	});

	it('how long it has run', () => {
		expect(duration(3 * 86400 + 4 * 3600 + 120)).toBe('3 days, 4 hours');
		expect(duration(86400)).toBe('1 day, 0 hours');
		expect(duration(600)).toBe('10 minutes');
	});

	it("initials and an app's letters", () => {
		expect(initials('Anna Kumar')).toBe('AK');
		expect(initials('amma')).toBe('AM');
		expect(appLetters('Photos')).toBe('Ph');
		expect(appLetters('Family Hub')).toBe('FH');
	});

	it("keeps each app's colour as the native desktop chose it (zlib's CRC-32)", () => {
		expect(crc32('photos') % 8).toBe(1);
		expect(crc32('hub') % 8).toBe(5);
		expect(appSvg('photos', 'Photos', 64)).toContain('>Ph<');
	});
});

describe('a fixed address', () => {
	it('takes a mask either way', () => {
		expect(prefixLength('255.255.255.0')).toBe(24);
		expect(prefixLength('24')).toBe(24);
		expect(prefixLength('255.0.255.0')).toBeNull();
		expect(maskOf(20)).toBe('255.255.240.0');
	});

	it("is the agent's fields, or words for the person", () => {
		expect(fixedSettings('192.168.1.40', '255.255.255.0', '192.168.1.1', '')).toEqual(
			{ mode: 'fixed', address: '192.168.1.40/24', gateway: '192.168.1.1', dns: ['192.168.1.1'] });
		expect(() => fixedSettings('192.168.1', '255.255.255.0', '', '')).toThrow(/192.168.1.40/);
		expect(() => fixedSettings('192.168.1.40', 'nope', '', '')).toThrow(/255.255.255.0/);
	});
});

describe('People', () => {
	const family = [
		{ id: 1, username: 'amma', display_name: 'Amma', role: 'admin', is_active: true, kind: 'adult', level: 'adult' },
		{ id: 2, username: 'zed', display_name: 'Zed', role: 'family', is_active: true, kind: 'adult', level: 'adult' },
		{ id: 3, username: 'kid', display_name: 'Nila', role: 'family', is_active: true, kind: 'child', level: 'child' },
		{ id: 4, username: 'paati', display_name: 'Paati', role: 'family', is_active: false, kind: 'adult', level: 'adult' }
	];

	it('lists admins first, then by name, the turned off last', () => {
		expect(inOrder(family).map((p) => p.id)).toEqual([1, 3, 2, 4]);
	});

	it("a child's parents are adults whose account is on", () => {
		expect(guardianChoices(family).map((p) => p.id)).toEqual([1, 2]);
		expect(guardianChoices(family, 1).map((p) => p.id)).toEqual([2]);
	});

	it('checks passwords and details', () => {
		expect(passwordProblem('short', 'short')).toMatch(/8 characters/);
		expect(passwordProblem('long enough', 'long enougH')).toMatch(/match/);
		expect(passwordProblem('long enough', 'long enough')).toBe('');
		expect(detailsProblem('', 'x', false, [])).toMatch(/name/);
		expect(detailsProblem('Priya', 'pri ya', false, [])).toMatch(/no spaces/);
		expect(detailsProblem('Priya', 'priya', true, [])).toMatch(/guardian/);
		expect(detailsProblem('Priya', 'priya', true, [1])).toBe('');
	});
});

describe("the OS's name", () => {
	it('is never written into a page with its version (the os-release template has it)', () => {
		const walk = (dir: string): string[] => readdirSync(dir).flatMap((f: string) => {
			const path = join(dir, f);
			return statSync(path).isDirectory() ? walk(path) : [path];
		});
		for (const path of walk(join(import.meta.dirname, '..')).filter((p) => /\.(ts|svelte)$/.test(p) && !p.endsWith('.test.ts'))) {
			expect(readFileSync(path, 'utf8'), path).not.toMatch(/ZOOMRIYA OS \d/);
		}
	});
});

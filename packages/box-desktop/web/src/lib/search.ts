// The top bar's search (the native desktop's search.py): each tool, each
// section with the other words people use for it, each setting in it,
// Support Center's questions and the top bar's actions; and, last, the
// system log. Words match in the box's language and in English, so "ssh" or
// "wifi" finds the same in every language.

import { _ } from './i18n.svelte';
import { ACTIONS, SECTION_WORDS, SETTINGS } from './words';

export interface Found {
	title: string;
	/** "Control Panel › Network", or "" for an action. */
	where: string;
	kind: string;
	section: string;
	action: string;
	score: number;
}

/** Lower case, accents dropped, so "reseau" finds "Réseau". */
export function fold(text: string): string {
	return text.toLowerCase().normalize('NFKD').replace(/\p{M}/gu, '');
}

/** The words in the box's language and in English, to match either. */
function both(english: string): string {
	return `${_(english)} ${english}`;
}

/** 0 unless every word typed is somewhere; more when the title starts with it or has it. */
export function score(query: string[], title: string, rest: string): number {
	title = fold(title);
	rest = fold(rest);
	if (!query.every((w) => title.includes(w) || rest.includes(w))) return 0;
	const joined = query.join(' ');
	if (title.startsWith(joined)) return 4;
	if (title.includes(joined)) return 3;
	if (query.every((w) => title.includes(w))) return 2;
	return 1;
}

/** Everything that matches what was typed, best first. tools: each tool's
 * name (English); sections: each "tool section"'s title (English). */
export function search(
	text: string,
	tools: Record<string, string>,
	sections: Map<string, string>,
	help: [string, string, string, string][],
	limit = 8
): Found[] {
	const query = fold(text).split(/\s+/).filter(Boolean);
	if (!query.length) return [];
	const found: Found[] = [];
	const where = (kind: string, section: string) => `${_(tools[kind])} › ${_(sections.get(`${kind} ${section}`)!)}`;
	const add = (f: Omit<Found, 'kind' | 'section' | 'action'> & Partial<Found>) =>
		found.push({ kind: '', section: '', action: '', ...f });
	for (const [kind, section, keywords] of SECTION_WORDS) {
		const title = sections.get(`${kind} ${section}`);
		if (!title) continue; // Sound, on a box without it
		const s = score(query, both(title), `${both(tools[kind])} ${both(keywords)}`);
		if (s) add({ title: _(title), where: _(tools[kind]), kind, section, score: s + 1 });
	}
	for (const [kind, section, title] of SETTINGS) {
		const named = sections.get(`${kind} ${section}`);
		if (!named) continue;
		const s = score(query, both(title), both(named));
		if (s) add({ title: _(title), where: where(kind, section), kind, section, score: s });
	}
	for (const [question, answer] of help) {
		const s = score(query, both(question), both(answer));
		if (s) add({ title: _(question), where: `${_('Support Center')} › ${_('Help')}`, kind: 'support', section: 'help', score: s - 1 });
	}
	for (const [title, action, keywords] of ACTIONS) {
		const s = score(query, both(title), both(keywords));
		if (s) add({ title: _(title), where: '', action, score: s });
	}
	// Stable, as Python's sort: equal scores keep their order.
	return found.map((f, i) => [f, i] as const).sort((a, b) => b[0].score - a[0].score || a[1] - b[1]).map(([f]) => f).slice(0, limit);
}

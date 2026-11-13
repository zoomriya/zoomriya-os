// Box Desktop's words in the box's language: English as written in the code,
// the others from the package's po/ (scripts/i18n.py extract and json), loaded
// as /words/<lang>.json. Changing the language changes every word on the page
// at once (the native desktop drew itself again).
//
// _("…") gives the words in the language in use; N_("…") only marks words kept
// in a list, translated with _() where they're shown. Words with {name} in
// them are filled in with fill().

/** Our screens' languages (gettext's codes), and each as Intl names it. */
export const OURS = ['en', 'fr', 'de', 'es', 'ta', 'ja', 'zh_CN'] as const;

let words = $state<Record<string, string>>({});
let language = $state('en');

export function _(text: string): string {
	return words[text] || text;
}

export function N_(text: string): string {
	return text;
}

/** Words with {name} parts, filled in: fill(_("{n} days"), { n: 3 }). */
export function fill(text: string, parts: Record<string, string | number>): string {
	return text.replace(/\{(\w+)\}/g, (all, key: string) => (key in parts ? String(parts[key]) : all));
}

/** The language in use, one of OURS. */
export function lang(): string {
	return language;
}

/** The language as Intl and HTML name it (zh-CN). */
export function locale(): string {
	return language.replace('_', '-');
}

/** From now on, words in this language; English where it has none. */
export async function use(code: string): Promise<void> {
	const chosen = (OURS as readonly string[]).includes(code) ? code : 'en';
	if (chosen === language && (chosen === 'en' || Object.keys(words).length)) return;
	let loaded: Record<string, string> = {};
	if (chosen !== 'en') {
		try {
			const answer = await fetch(`/words/${chosen}.json`, { cache: 'no-cache' });
			if (answer.ok) loaded = await answer.json();
		} catch {
			// English, then: the page works on without its translations.
		}
	}
	words = loaded;
	language = chosen;
	document.documentElement.lang = locale();
}

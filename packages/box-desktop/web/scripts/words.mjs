#!/usr/bin/env node
// The page's words in each language (public/words/<lang>.json), from the
// package's po/ (scripts/i18n.py extract): {English: translation}. Drafts
// are used, as gettext uses them; untranslated words are left out, so the
// page shows them in English.

import { mkdirSync, readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
// node scripts/words.mjs [PO-FOLDER OUT-FOLDER]: Box Desktop's by default.
const po = resolve(process.argv[2] ?? resolve(here, '../../po'));
const out = resolve(process.argv[3] ?? resolve(here, '../public/words'));

const unquote = (text) => JSON.parse(text.replace(/\\t/g, '\\t'));

function read(path) {
	const found = {};
	let field = '', parts = { msgid: '', msgstr: '' };
	const keep = () => {
		if (parts.msgid && parts.msgstr) found[parts.msgid] = parts.msgstr;
	};
	for (const raw of [...readFileSync(path, 'utf8').split('\n'), '']) {
		const line = raw.trim();
		if (!line) {
			keep();
			field = '';
			parts = { msgid: '', msgstr: '' };
		} else if (line.startsWith('#')) {
			continue;
		} else if (line.startsWith('msgid ') || line.startsWith('msgstr ')) {
			const space = line.indexOf(' ');
			field = line.slice(0, space);
			parts[field] = unquote(line.slice(space + 1));
		} else if (line.startsWith('"') && field) {
			parts[field] += unquote(line);
		}
	}
	return found;
}

mkdirSync(out, { recursive: true });
for (const file of readdirSync(po).filter((f) => f.endsWith('.po')).sort()) {
	const words = read(join(po, file));
	const sorted = Object.fromEntries(Object.keys(words).sort().map((k) => [k, words[k]]));
	writeFileSync(join(out, file.replace(/\.po$/, '.json')), JSON.stringify(sorted) + '\n');
	console.log(`words/${file.replace(/\.po$/, '.json')}: ${Object.keys(words).length}`);
}

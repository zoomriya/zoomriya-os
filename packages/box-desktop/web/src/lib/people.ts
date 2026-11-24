// People's rules, as the native desktop's people.py had them.

import { _ } from './i18n.svelte';

/** ZOOMRIYA ID's shortest password, and a child's most parents or guardians. */
export const MIN_PASSWORD = 8;
export const MAX_GUARDIANS = 2;

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type Person = Record<string, any>;

export function shownName(person: Person): string {
	return person.display_name || person.username || '';
}

/** Accounts that are on first, admins first among them, then by name. */
export function inOrder(people: Person[]): Person[] {
	const key = (p: Person) => [p.is_active === false ? 1 : 0, p.role !== 'admin' ? 1 : 0, shownName(p).toLowerCase()] as const;
	return [...people].sort((a, b) => {
		const x = key(a), y = key(b);
		return x[0] - y[0] || x[1] - y[1] || x[2].localeCompare(y[2]);
	});
}

/** Who can be a child's parent or guardian: adults whose account is on. */
export function guardianChoices(people: Person[], leavingOut?: number): Person[] {
	return inOrder(people).filter((p) => p.is_active && p.kind !== 'child' && (p.level ?? 'adult') === 'adult' && p.id !== leavingOut);
}

export function passwordProblem(first: string, again: string): string {
	if (first.length < MIN_PASSWORD) return _('Use at least 8 characters.');
	if (first !== again) return _("The passwords don't match.");
	return '';
}

export function detailsProblem(name: string, username: string, child: boolean, guardians: number[]): string {
	if (!name.trim()) return _('Type their name.');
	if (!username.trim() || /\s/.test(username.trim())) return _('Type a username with no spaces.');
	if (child && !guardians.length) return _('Choose a parent or guardian.');
	return '';
}

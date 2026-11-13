// Talking to box-desktopd: JSON both ways, the page's token with every
// change, and each refusal worded as the native desktop worded it (its codes
// are box-desktopd's, its sentences ours, in the admin's language).

import { _ } from './i18n.svelte';

let token = '';
/** Told when box-desktopd says the sign-in is over (expired, or signed out elsewhere). */
let signedOut: () => void = () => {};
/** Told when the sign-in is locked (the screen locked in this browser). */
let lockedOut: () => void = () => {};

export function setToken(value: string): void {
	token = value;
}

export function onSignedOut(then: () => void, locked: () => void): void {
	signedOut = then;
	lockedOut = locked;
}

/** A refusal: its code, and the words for the person. */
export class Problem extends Error {
	constructor(
		public code: string,
		message: string,
		public ticket = '',
		public status = 0
	) {
		super(message);
	}
}

interface Refusal {
	code?: string;
	error?: string;
	detail?: string;
	ticket?: string;
}

/** box-desktopd's codes in the native desktop's words. */
export function words(r: Refusal): string {
	switch (r.code) {
		case 'wrong_password': return _("That username and password don't match.");
		case 'new_password_needed': return _('Choose your own password to finish signing in.');
		case 'second_step_needed': return _('Enter the code from your authenticator app.');
		case 'second_step_setup_needed': return _('Set up a second sign-in step first.');
		case 'wrong_code': return _("That code isn't right. Check the time on your phone, or use a backup code.");
		case 'too_slow': return _('Signing in took too long. Start again with your password.');
		case 'password_in_use': return _("Choose a password you aren't using now.");
		case 'locked': return _("Too many wrong passwords. Try again in 15 minutes, or use Can't sign in?");
		case 'too_many': return _('Too many tries. Try again in a minute.');
		case 'id_failed': return _("ZOOMRIYA ID couldn't sign you in just now.");
		case 'id_down': return _("ZOOMRIYA ID isn't responding. Try again shortly.");
		case 'not_admin': return _("Box Desktop is for the box's admins.");
		case 'check_failed': return _("ZOOMRIYA ID couldn't check your password just now.");
		case 'wrong_unlock': return _("That password isn't right.");
		case 'password_needed':
		case 'not_your_password': return _("That isn't your password.");
		case 'sign_in_again': return _('Sign in again to make changes.');
		case 'admins_only': return _("Only the box's admins can do that.");
		case 'username_taken': return _('That username is taken. Try another.');
		case 'not_accepted': return r.detail || _("ZOOMRIYA ID didn't accept that.");
		case 'change_failed': return _("ZOOMRIYA ID couldn't make that change just now.");
		case 'box_down': return _("The box isn't responding. Try again shortly.");
		case 'box_unreadable': return _("The box gave an answer the setup can't read.");
		case 'box_refused': return r.error || _("The box couldn't do that.");
		case 'at_the_box': return _("This works at the box's own screen only.");
		case 'picture_not_kept': return _("The box couldn't keep that picture.");
		case 'id_down_later': return _("ZOOMRIYA ID isn't responding. Try again, or do this later.");
		case 'id_start_failed': return _("ZOOMRIYA ID couldn't start this just now. Try again, or do this later.");
		case 'id_finish_failed': return _("ZOOMRIYA ID couldn't finish this just now. Try again, or do this later.");
		case 'setup_wrong_code': return _("That code isn't right. Enter the code the app shows now.");
		case 'setup_too_slow': return _('That took too long. Press Try again for a new code.');
		default: return _("The box couldn't do that.");
	}
}

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
	const headers: Record<string, string> = {};
	if (body !== undefined) headers['Content-Type'] = 'application/json';
	if (method !== 'GET' && token) headers['X-Box-Desktop-Token'] = token;
	let answer: Response;
	try {
		answer = await fetch(path, {
			method,
			headers,
			body: body === undefined ? undefined : JSON.stringify(body),
			credentials: 'same-origin',
			cache: 'no-store'
		});
	} catch {
		throw new Problem('box_down', words({ code: 'box_down' }));
	}
	let data: unknown = null;
	try {
		data = await answer.json();
	} catch {
		data = null;
	}
	if (!answer.ok) {
		const refusal = (data ?? {}) as Refusal;
		if (refusal.code === 'signed_out') signedOut();
		if (refusal.code === 'screen_locked') lockedOut();
		throw new Problem(refusal.code ?? 'unknown', words(refusal), refusal.ticket ?? '', answer.status);
	}
	return data as T;
}

export const get = <T>(path: string) => call<T>('GET', path);
export const post = <T>(path: string, body: unknown = {}) => call<T>('POST', path, body);
export const put = <T>(path: string, body: unknown) => call<T>('PUT', path, body);
export const patch = <T>(path: string, body: unknown) => call<T>('PATCH', path, body);

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type Answer = Record<string, any>;

/** The agent, in the signed-in admin's name; password when the request is a risky one. */
export function ask(op: string, fields: Record<string, unknown> = {}, password?: string): Promise<Answer> {
	const request: Record<string, unknown> = { op };
	for (const [k, v] of Object.entries(fields)) if (v !== undefined && v !== null) request[k] = v;
	return post<Answer>('/api/agent', password === undefined ? { request } : { request, password });
}

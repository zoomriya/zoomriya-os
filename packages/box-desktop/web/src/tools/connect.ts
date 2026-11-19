// ZOOMRIYA Connect's tool: what connectd says, through box-desktopd
// (/api/connect/...), and its refusals in Box Desktop's own words.

import { Problem, del, get, post } from '../lib/api';
import { _ } from '../lib/i18n.svelte';

export interface ConnectStatus {
	on: boolean;
	name: string | null;
	fqdn: string | null;
	/** The ZOOMRIYA Connect account the box is linked to. */
	account: string | null;
	/** Where the family changes the account's password. */
	account_page?: string;
	server: string;
	our_server: boolean;
	relay_off: boolean;
	box_key: string;
	devices: number;
	connected: number;
	pairing: boolean;
	clock_synced: boolean;
	relay: string | null;
	relay_connected: boolean;
	relay_refused?: string;
	certificate_days?: number;
	/** This month's use of the server's relays, once the server has said. */
	relay_use?: { month: string; used: number; allowance: number; slowed: boolean };
	certificate_authority?: string;
	home: { ipv4: string[]; ipv6: string[] };
	problems: Record<string, string>;
}

export interface Device {
	id: string;
	name: string;
	admin: boolean;
	paired_at: number;
	last_seen: number;
	pass_until: number;
	connected: boolean;
	reports: { ms_to_connect?: number; ms_to_direct?: number | null; secs_on_relay?: number }[];
}

/** connectd's codes in Box Desktop's words; anything else, the box's own sentence. */
export function connectWords(e: unknown): string {
	const code = e instanceof Problem ? e.code : '';
	switch (code) {
		case 'connect_unavailable': return _("ZOOMRIYA Connect isn't running on this box. Restart the box; if this stays, open Support Center.");
		case 'name_length': return _('Pick a name of 3 to 32 letters or numbers.');
		case 'name_characters': return _('Use only small letters, numbers and hyphens, with a letter or number at each end.');
		case 'name_share': return _('Names can\'t end in "-share". That ending is kept for share links.');
		case 'name_reserved': return _('That name is kept for ZOOMRIYA. Pick another.');
		case 'name_blocked': return _("That name isn't allowed. Pick another.");
		case 'name_taken': return _('That name is taken. Pick another.');
		case 'box_has_name': return _('This box already has a name. Give it up first to pick another.');
		case 'too_many_boxes': return _('Too many new boxes from your internet connection today. Try again tomorrow.');
		case 'factory_token': return _("This box's factory token isn't valid, or was already used. Contact ZOOMRIYA support.");
		case 'account_needed': return _('Type the ZOOMRIYA Connect username and password you were given.');
		case 'wrong_login': return _("That username or password isn't right.");
		case 'account_locked': return _('Too many wrong passwords. Try again in 15 minutes.');
		case 'account_blocked': return _('This ZOOMRIYA Connect account is blocked. Ask whoever gave you the account.');
		case 'account_full': return _('This account already has all the boxes it can. Ask whoever gave you the account for room for another.');
		case 'please_update': return _('Your box needs an update to use ZOOMRIYA Connect. It updates by itself overnight.');
		case 'unreachable': return _("The box can't reach the ZOOMRIYA Connect server. Check the internet connection, then try again.");
		case 'clock': return _("Your box can't get the time from the internet yet. Check its internet connection, then try again.");
		case 'not_on': return _('Switch ZOOMRIYA Connect on before pairing a phone.');
		case 'not_https': return _("Type your server's address starting with https://");
		case 'not_a_server': return _("That address doesn't answer like a ZOOMRIYA Connect server.");
		default: return e instanceof Error ? e.message : _("The box couldn't do that.");
	}
}

export const status = () => get<ConnectStatus>('/api/connect/v1/status');
export const devices = () => get<Device[]>('/api/connect/v1/devices');
export const setup = (name: string, username: string, password: string) =>
	post('/api/connect/v1/setup', { name, username, password });
export const turnOn = () => post('/api/connect/v1/on');
export const turnOff = (password: string) => post('/api/connect/v1/off', { password });
export const release = (password: string) => post('/api/connect/v1/release', { password });
export const pairing = (name: string, admin: boolean) =>
	post<{ url: string; expires_in: number }>('/api/connect/v1/pairing', { name, admin });
export const stopPairing = () => del('/api/connect/v1/pairing');
export const removeDevice = (id: string, password: string) => del(`/api/connect/v1/devices/${id}`, { password });
export const useServer = (url: string | null, password: string) => post('/api/connect/v1/server', { url, password });
export const setRelay = (off: boolean) => post('/api/connect/v1/relay', { off });

/** Seconds since 1970 as an ISO time, for format.ts. */
export const iso = (seconds: number) => new Date(seconds * 1000).toISOString();

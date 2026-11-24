// Control Panel › Remote access: how the box is reached away from home, and
// what would make it faster (zoomriya-connect docs/direct-plus.md §4, §8 A).
// What connectd measures, through box-desktopd (/api/connect/v1/...). The
// way through when nothing direct works is never named on screen.

import { del, get, put } from '../lib/api';
import type { DevicesStatus } from './devices';

/** What connectd knows about being reached from away. */
export interface Reach {
	/** The home's address as the internet sees it, if known. */
	outside_ipv4: string | null;
	/** The port the router opened for the box, if it did. */
	port_mapped: string | null;
	/** The box's own lasting IPv6 addresses. */
	ipv6: string[];
	/** Two routers in a row, or an address shared with other homes. */
	double_nat: boolean | null;
	/** The addresses paired devices find. */
	published: string[];
	published_at: number | null;
}

export interface RemoteStatus extends DevicesStatus {
	reach: Reach;
	/** The way through when no direct path works is ready. */
	tor: boolean;
	/** Remote access away from home is on. */
	away: boolean;
	port: number;
}

/** The home's case, best first. */
export type Case = 'off' | 'open' | 'ipv6' | 'double' | 'shared';

export function homeCase(s: RemoteStatus): Case {
	if (!s.away) return 'off';
	if (s.reach.port_mapped && !s.reach.double_nat) return 'open';
	if (s.reach.ipv6.length) return 'ipv6';
	if (s.reach.double_nat) return 'double';
	return 'shared';
}

/** Each change that would help, for this case. */
export type Step = 'ipv6' | 'ipv6-firewall' | 'upnp' | 'bridge' | 'provider';

export function steps(s: RemoteStatus): Step[] {
	const c = homeCase(s);
	if (c === 'off' || c === 'open') return [];
	const out: Step[] = [];
	if (s.reach.ipv6.length) out.push('ipv6-firewall');
	else out.push('ipv6');
	if (s.reach.double_nat) out.push('bridge');
	else if (!s.reach.port_mapped) out.push('upnp');
	// No address of its own: only the provider can change that.
	if (s.reach.double_nat || !s.reach.outside_ipv4) out.push('provider');
	return out;
}

export const setAway = (on: boolean) => put('/api/connect/v1/away', { on });

/** The family's own name, as connectd reports it (never the key). */
export interface NamesStatus {
	domain: string | null;
	home: string | null;
	share: string | null;
	records: string[];
	records_at: number | null;
	certificate_days: number | null;
	/** The CAA record to add, so only this box can get certificates. */
	caa: string | null;
	problem: string | null;
	/** Certificates in the public logs that the box did not ask for. */
	strangers: string[];
}

/** What the admin types: the domain, and the key that changes its records (RFC 2136). */
export interface NamesSettings {
	domain: string;
	dns: { server: string; zone: string; key_name: string; algorithm: string; secret: string };
	acme_dns?: { url: string; subdomain: string; username: string; password: string } | null;
}

/** How guests reach the box: the door, with the family's own name. */
export interface GuestsStatus {
	door: string | null;
	door_mapped: boolean;
	door_reachable: boolean | null;
	door_only_viewer: boolean | null;
	checked_at: number | null;
	problem: string | null;
}

export const names = () => get<NamesStatus>('/api/connect/v1/names');
export const guests = () => get<GuestsStatus>('/api/connect/v1/guests');
export const setNames = (s: NamesSettings, password: string) => put('/api/connect/v1/names', { ...s, password });
export const forgetNames = (password: string) => del('/api/connect/v1/names', { password });

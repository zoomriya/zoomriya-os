import { describe, expect, it } from 'vitest';
import { homeCase, steps, type RemoteStatus } from '../tools/remote';

const base: RemoteStatus = {
	box_key: 'k', devices: 1, connected: 0, pairing: false, tor: true, away: true, port: 443,
	reach: { outside_ipv4: null, port_mapped: null, ipv6: [], double_nat: null, published: [], published_at: null }
};
const with_ = (reach: Partial<RemoteStatus['reach']>, more: Partial<RemoteStatus> = {}): RemoteStatus =>
	({ ...base, ...more, reach: { ...base.reach, ...reach } });

describe('remote access: the home case and its steps', () => {
	it('off says nothing more', () => {
		const s = with_({ ipv6: ['2a02::1'] }, { away: false });
		expect(homeCase(s)).toBe('off');
		expect(steps(s)).toEqual([]);
	});
	it('a port the router opened is best', () => {
		const s = with_({ port_mapped: '81.2.69.142:443', outside_ipv4: '81.2.69.142', double_nat: false });
		expect(homeCase(s)).toBe('open');
		expect(steps(s)).toEqual([]);
	});
	it('IPv6 without a port: open the IPv6 firewall, turn on UPnP', () => {
		const s = with_({ ipv6: ['2a02::1'], outside_ipv4: '81.2.69.142', double_nat: false });
		expect(homeCase(s)).toBe('ipv6');
		expect(steps(s)).toEqual(['ipv6-firewall', 'upnp']);
	});
	it('two routers in a row: bridge mode, and the provider', () => {
		const s = with_({ outside_ipv4: '81.2.69.142', double_nat: true });
		expect(homeCase(s)).toBe('double');
		expect(steps(s)).toEqual(['ipv6', 'bridge', 'provider']);
	});
	it('a shared address with no IPv6: the provider', () => {
		expect(homeCase(base)).toBe('shared');
		expect(steps(base)).toEqual(['ipv6', 'upnp', 'provider']);
	});
});

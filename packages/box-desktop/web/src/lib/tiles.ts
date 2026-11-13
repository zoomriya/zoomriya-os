// The tools' tiles, drawn for ZOOMRIYA OS (the native desktop's tiles.py,
// unchanged): a rounded tile in the tool's own shade of ZOOMRIYA blue, with a
// sheen and a shadow beneath, and the tool's object on it in its own
// materials. Ours: nothing here comes from an icon set. Each tile is an SVG
// on a 64-unit grid; small ones leave out the finest lines so they stay crisp.
// Every id in a drawing is prefixed, so many tiles can be on one page.

import { appLetters } from './format';

/** Each tool's tile, top to bottom of its gradient. */
export const COLOURS: Record<string, [string, string]> = {
	control: ['#A3B0C6', '#3A475E'],
	storage: ['#4FD6E6', '#08708A'],
	resources: ['#A79BFF', '#4A2FC4'],
	logs: ['#7CCBFF', '#1E7FD8'],
	support: ['#5EDDB9', '#08795F'],
	apps: ['#5B8DFF', '#1747B5']
};

/** How much of the drawing the tile itself fills; the rest is its shadow. */
export const BODY_SHARE = 58 / 64;

export function squircle(x: number, y: number, s: number): string {
	const a = s / 2, b = s * 0.883, e = s * 0.117;
	return `M${x + a} ${y}C${x + b} ${y} ${x + s} ${y + e} ${x + s} ${y + a}S${x + b} ${y + s} ${x + a} ${y + s} ` +
		`${x} ${y + b} ${x} ${y + a} ${x + e} ${y} ${x + a} ${y}Z`;
}

const BODY = squircle(3, 2.5, 58);

type Stop = [number, string, number];

export function linear(id: string, stops: Stop[], x2 = 0, y2 = 1): string {
	const drawn = stops
		.map(([o, c, a]) => `<stop offset="${o}" stop-color="${c}"` + (a !== 1 ? ` stop-opacity="${a}"` : '') + '/>')
		.join('');
	return `<linearGradient id="${id}" x1="0" y1="0" x2="${x2}" y2="${y2}">${drawn}</linearGradient>`;
}

type Drawing = (fine: boolean) => [string, string];

const control: Drawing = (fine) => {
	let defs = linear('groove', [[0, '#121821', 1], [1, '#3B4757', 1]]);
	defs += '<radialGradient id="knob" cx=".4" cy=".35" r=".7"><stop offset="0" stop-color="#fff"/>' +
		'<stop offset=".6" stop-color="#EEF1F5"/><stop offset="1" stop-color="#B8C1CD"/></radialGradient>';
	let shapes = '';
	([[20, 36, '#7DD3FC', '#0EA5E9'], [32, 24, '#86EFAC', '#16A34A'], [44, 44, '#FDBA74', '#EA580C']] as const)
		.forEach(([y, x, top, bottom], n) => {
			defs += linear(`fill${n}`, [[0, top, 1], [1, bottom, 1]]);
			shapes += `<rect x="13" y="${y - 3}" width="38" height="6" rx="3" fill="url(#groove)"/>` +
				`<rect x="13.5" y="${y - 2.5}" width="${x - 13.5}" height="5" rx="2.5" fill="url(#fill${n})"/>` +
				`<g filter="url(#lift)"><circle cx="${x}" cy="${y}" r="5.6" fill="url(#knob)"/></g>`;
			if (fine) {
				shapes += `<rect x="13" y="${y + 2.2}" width="38" height=".8" rx=".4" fill="#fff" fill-opacity=".18"/>` +
					`<circle cx="${x}" cy="${y}" r="2.2" fill="#C7CFD9"/>` +
					`<circle cx="${x}" cy="${y - 0.4}" r="1.8" fill="#E4E8EE"/>`;
			}
		});
	return [defs, shapes];
};

const storage: Drawing = (fine) => {
	let defs = linear('alu', [[0, '#FFFFFF', 1], [0.5, '#E3E8EE', 1], [1, '#B9C3CF', 1]]);
	defs += '<radialGradient id="led"><stop offset="0" stop-color="#ECFDF5"/><stop offset=".45" stop-color="#4ADE80"/>' +
		'<stop offset="1" stop-color="#15803D"/></radialGradient>' +
		'<filter id="glow" x="-1" y="-1" width="3" height="3"><feGaussianBlur stdDeviation="1.6"/></filter>';
	let shapes = '';
	for (const y of [14.5, 33]) {
		shapes += `<g filter="url(#lift)"><rect x="11.5" y="${y}" width="41" height="15.5" rx="3.5" fill="url(#alu)"/></g>` +
			`<rect x="16.5" y="${y + 6}" width="20" height="3.2" rx="1.6" fill="#6B7684" fill-opacity=".55"/>` +
			`<circle cx="45" cy="${y + 7.6}" r="4.2" fill="#4ADE80" fill-opacity=".7" filter="url(#glow)"/>` +
			`<circle cx="45" cy="${y + 7.6}" r="2.3" fill="url(#led)"/>`;
		if (fine) {
			shapes += `<rect x="12" y="${y + 0.5}" width="40" height="1" rx=".5" fill="#fff"/>` +
				`<rect x="16.5" y="${y + 8.2}" width="20" height="1" rx=".5" fill="#fff" fill-opacity=".8"/>`;
		}
	}
	return [defs, shapes];
};

const resources: Drawing = (fine) => {
	let defs = linear('glass', [[0, '#FFFFFF', 1], [1, '#E6DEFA', 1]]);
	defs += linear('area', [[0, '#8B5CF6', 0.35], [1, '#8B5CF6', 0]]);
	defs += linear('line', [[0, '#7C3AED', 1], [1, '#EC4899', 1]], 1, 0);
	defs += '<filter id="glow" x="-.2" y="-.5" width="1.4" height="2"><feGaussianBlur stdDeviation="1.4"/></filter>';
	const pulse = 'M15.5 32h7l4-10 6 17 5-12 3 5h7.5';
	const width = fine ? 2.8 : 3.6;
	let shapes = '<g filter="url(#lift)"><rect x="22" y="46.5" width="20" height="3.5" rx="1.75" fill="#EDE7FB"/>' +
		'<rect x="10.5" y="13" width="43" height="31" rx="5.5" fill="url(#glass)"/></g>' +
		'<rect x="28" y="43.5" width="8" height="3.5" fill="#D7CCF5"/>';
	if (fine) {
		shapes += `<path d="${pulse}V40h-32.5z" fill="url(#area)"/>` +
			`<path d="${pulse}" fill="none" stroke="#C084FC" stroke-width="4" stroke-linecap="round" ` +
			'stroke-linejoin="round" filter="url(#glow)" opacity=".8"/>' +
			'<g fill="#C4B5FD"><circle cx="15.5" cy="17.5" r="1.1"/><circle cx="19" cy="17.5" r="1.1"/>' +
			'<circle cx="22.5" cy="17.5" r="1.1"/></g>';
	}
	shapes += `<path d="${pulse}" fill="none" stroke="url(#line)" stroke-width="${width}" stroke-linecap="round" ` +
		'stroke-linejoin="round"/>';
	return [defs, shapes];
};

const logs: Drawing = (fine) => {
	let defs = linear('paper', [[0, '#FFFFFF', 1], [1, '#F1ECE3', 1]]);
	defs += linear('fold', [[0, '#FFFFFF', 1], [1, '#D9CDB9', 1]], 1, 1);
	const page = 'M20 11h17l10 10v28.5a3.5 3.5 0 0 1-3.5 3.5H20a3.5 3.5 0 0 1-3.5-3.5v-35A3.5 3.5 0 0 1 20 11z';
	const rows: [number, number, string][] = fine
		? [[27, 20, '#22C55E'], [33.5, 16, '#22C55E'], [40, 18, '#F59E0B'], [46.5, 12, '#EF4444']]
		: [[28, 20, '#22C55E'], [36, 16, '#F59E0B'], [44, 12, '#EF4444']];
	let shapes = '<g filter="url(#lift)"><rect x="18" y="13" width="30" height="40" rx="3.5" fill="#FBE3B8" ' +
		`transform="rotate(-7 33 33)"/><path d="${page}" fill="url(#paper)"/></g>` +
		'<path d="M37 11v6.5a3.5 3.5 0 0 0 3.5 3.5H47z" fill="url(#fold)"/>';
	if (fine) shapes += '<rect x="21" y="16" width="11" height="3" rx="1.5" fill="#E8A33D"/>';
	for (const [y, w, colour] of rows) {
		shapes += `<circle cx="22" cy="${y}" r="${fine ? 1.7 : 2.2}" fill="${colour}"/>` +
			`<rect x="25.5" y="${y - 1.2}" width="${w}" height="${fine ? 2.4 : 3}" rx="1.2" fill="#B8AE9F"/>`;
	}
	return [defs, shapes];
};

const support: Drawing = (fine) => {
	let defs = '<radialGradient id="tube" cx="32" cy="32" r="19.5" gradientUnits="userSpaceOnUse">' +
		'<stop offset=".44" stop-color="#000" stop-opacity=".35"/><stop offset=".6" stop-color="#000" stop-opacity="0"/>' +
		'<stop offset=".7" stop-color="#fff" stop-opacity=".55"/><stop offset=".8" stop-color="#fff" stop-opacity="0"/>' +
		'<stop offset="1" stop-color="#000" stop-opacity=".3"/></radialGradient>';
	defs += linear('light', [[0, '#fff', 0.3], [0.55, '#fff', 0], [1, '#000', 0.22]]);
	const ring = 'cx="32" cy="32" r="14.5" fill="none" stroke-width="10.5"';
	let shapes = `<g filter="url(#lift)"><circle ${ring} stroke="#FFFFFF"/></g>` +
		`<circle ${ring} stroke="#E11D48" stroke-dasharray="10 12.78" stroke-dashoffset="-6.39"/>` +
		`<circle ${ring} stroke="url(#tube)"/><circle ${ring} stroke="url(#light)"/>`;
	if (fine) shapes += '<circle cx="32" cy="32" r="19.5" fill="none" stroke="#7F1D1D" stroke-opacity=".25" stroke-width=".6"/>';
	return [defs, shapes];
};

const apps: Drawing = (fine) => {
	let defs = '', shapes = '';
	([[12, 12, '#FDBA74', '#EA580C'], [35, 12, '#86EFAC', '#16A34A'], [12, 35, '#F9A8D4', '#DB2777']] as const)
		.forEach(([x, y, top, bottom], n) => {
			defs += linear(`app${n}`, [[0, top, 1], [1, bottom, 1]]);
			shapes += `<g filter="url(#lift)"><path d="${squircle(x, y, 17)}" fill="url(#app${n})"/></g>`;
			if (fine) shapes += `<path d="${squircle(x, y, 17)}" fill="url(#sheen)"/>`;
		});
	defs += linear('disc', [[0, '#FFFFFF', 1], [1, '#DCE5F2', 1]]);
	shapes += '<g filter="url(#lift)"><circle cx="43.5" cy="43.5" r="9.5" fill="url(#disc)"/></g>' +
		'<path d="M43.5 38.5v10M38.5 43.5h10" stroke="#2563EB" stroke-width="3.2" stroke-linecap="round"/>';
	return [defs, shapes];
};

const DRAW: Record<string, Drawing> = { control, storage, resources, logs, support, apps };

/** Every id="x" and url(#x) in an SVG made unique for this drawing. */
function prefixed(svg: string, prefix: string): string {
	return svg.replace(/id="([^"]+)"/g, `id="${prefix}$1"`).replace(/url\(#([^)]+)\)/g, `url(#${prefix}$1)`);
}

let made = 0;

const COMMON = (top: string, bottom: string, lift = '1.3" stdDeviation="1.1" flood-color="#000" flood-opacity=".33') =>
	linear('bg', [[0, top, 1], [1, bottom, 1]]) +
	linear('sheen', [[0, '#fff', 0.32], [0.5, '#fff', 0]]) +
	linear('edge', [[0, '#fff', 0.6], [0.25, '#fff', 0], [0.85, '#000', 0], [1, '#000', 0.25]]) +
	'<filter id="drop" x="-20%" y="-20%" width="140%" height="140%"><feDropShadow dx="0" dy="1.4" stdDeviation="1.3" flood-color="#000" flood-opacity=".32"/></filter>' +
	`<filter id="lift" x="-30%" y="-30%" width="160%" height="160%"><feDropShadow dx="0" dy="${lift}"/></filter>`;

/** The tile for kind as an SVG px wide, its shadow included. */
export function tileSvg(kind: string, px: number, fine = true): string {
	const [top, bottom] = COLOURS[kind];
	const [defs, shapes] = DRAW[kind](fine);
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${px}" height="${px}" viewBox="0 0 64 64"><defs>` +
		COMMON(top, bottom) + defs + '</defs>' +
		`<g filter="url(#drop)"><path d="${BODY}" fill="url(#bg)"/></g>` +
		`<path d="${BODY}" fill="url(#sheen)"/>` + shapes +
		`<path d="${BODY}" fill="none" stroke="url(#edge)" stroke-width="1"/></svg>`;
	return prefixed(svg, `t${++made}-`);
}

/** An app's tile: one of these shades, chosen by its id, so each app keeps its
 * colour, with its letters; until apps carry their own icons. */
export const APP_SHADES: [string, string][] = [
	['#FF8FB1', '#D61F69'], ['#FFB565', '#E5560B'], ['#6EE29A', '#15803D'], ['#5FD8EA', '#0A7A96'],
	['#A49BFF', '#4B34C8'], ['#FFD965', '#D98C00'], ['#FF8A80', '#D2261C'], ['#7CB6FF', '#1D63D8']
];

/** zlib's CRC-32, as tiles.py chose the shade by it. */
export function crc32(text: string): number {
	let crc = 0xffffffff;
	for (const byte of new TextEncoder().encode(text)) {
		crc ^= byte;
		for (let k = 0; k < 8; k++) crc = crc & 1 ? (crc >>> 1) ^ 0xedb88320 : crc >>> 1;
	}
	return (crc ^ 0xffffffff) >>> 0;
}

function escape(text: string): string {
	return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

export function appSvg(appId: string, name: string, px: number): string {
	const [top, bottom] = APP_SHADES[crc32(appId) % APP_SHADES.length];
	const letters = escape(appLetters(name));
	const size = appLetters(name).length === 1 ? 26 : 22;
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${px}" height="${px}" viewBox="0 0 64 64"><defs>` +
		COMMON(top, bottom, '1" stdDeviation=".9" flood-color="#000" flood-opacity=".3') + '</defs>' +
		`<g filter="url(#drop)"><path d="${BODY}" fill="url(#bg)"/></g>` +
		`<path d="${BODY}" fill="url(#sheen)"/>` +
		`<text x="32" y="${(31.5 + size * 0.36).toFixed(1)}" text-anchor="middle" font-family="Inter Variable, Inter, sans-serif" font-weight="700" ` +
		`font-size="${size}" fill="#fff" filter="url(#lift)">${letters}</text>` +
		`<path d="${BODY}" fill="none" stroke="url(#edge)" stroke-width="1"/></svg>`;
	return prefixed(svg, `a${++made}-`);
}

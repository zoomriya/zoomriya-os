// The line icons of the top bar, the windows and Control Panel's rows, and
// their badges (the native desktop's icons.py): drawn in the theme's colours
// (CSS's currentColor, from the role's class), or white on a small tile in
// their badge shade.

import { BADGE, BADGE_SHADES, GLYPHS } from './words';
import { linear, squircle } from './tiles';

/** A line icon, px across, in currentColor. */
export function glyphSvg(name: string, px: number, width = 1.8): string {
	const body = (GLYPHS[name] ?? '').replaceAll('{c}', 'currentColor');
	return `<svg xmlns="http://www.w3.org/2000/svg" width="${px}" height="${px}" viewBox="0 0 24 24" ` +
		`fill="none" stroke="currentColor" stroke-width="${width}" stroke-linecap="round" ` +
		`stroke-linejoin="round" aria-hidden="true">${body}</svg>`;
}

/** A row's role, where it isn't the accent, says the badge's colour instead. */
const BADGE_ROLE: Record<string, string> = { ok: 'green', warn: 'orange', muted: 'graphite' };

export function badgeShade(name: string, role = 'accent', shade = ''): string {
	return shade || BADGE_ROLE[role] || BADGE[name] || 'blue';
}

let made = 0;

/** A line icon as a small tile: white on its colour. */
export function badgeSvg(name: string, px: number, shade: string): string {
	const [top, bottom] = BADGE_SHADES[shade] ?? BADGE_SHADES.blue;
	const body = squircle(0.5, 0.5, 23);
	const p = `b${++made}-`;
	return `<svg xmlns="http://www.w3.org/2000/svg" width="${px}" height="${px}" viewBox="0 0 24 24" aria-hidden="true"><defs>` +
		linear(`${p}bg`, [[0, top, 1], [1, bottom, 1]]) +
		linear(`${p}edge`, [[0, '#fff', 0.55], [0.3, '#fff', 0], [0.8, '#000', 0], [1, '#000', 0.22]]) +
		`<filter id="${p}lift" x="-20%" y="-20%" width="140%" height="140%"><feDropShadow dx="0" dy=".5" ` +
		'stdDeviation=".45" flood-color="#000" flood-opacity=".3"/></filter></defs>' +
		`<path d="${body}" fill="url(#${p}bg)"/><path d="${body}" fill="none" stroke="url(#${p}edge)" stroke-width=".5"/>` +
		'<g transform="translate(3.6 3.6) scale(.7)" fill="none" stroke="#fff" stroke-width="2.2" ' +
		`stroke-linecap="round" stroke-linejoin="round" filter="url(#${p}lift)">${(GLYPHS[name] ?? '').replaceAll('{c}', '#fff')}</g>` +
		'</svg>';
}

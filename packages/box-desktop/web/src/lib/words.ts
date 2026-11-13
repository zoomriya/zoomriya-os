// Box Desktop's fixed words and drawings, as the native desktop had them
// (icons.py, search.py, support.py, control.py, appearance.py, tools.py,
// apps.py, display.py): the same English, so the same translations. N_("…")
// only marks words for scripts/i18n.py; _() translates them where shown.

import { N_ } from './i18n.svelte';

/** The line icons, on a 24-unit grid; {c} is the icon's colour, for the shapes drawn filled. */
export const GLYPHS: Record<string, string> = {
	// The password fields' eye: shown or hidden.
	"eye": "<path d=\"M2.5 12s3.5-6.5 9.5-6.5 9.5 6.5 9.5 6.5-3.5 6.5-9.5 6.5S2.5 12 2.5 12z\"/><circle cx=\"12\" cy=\"12\" r=\"3\"/>",
	"eye-off": "<path d=\"M2.5 12s3.5-6.5 9.5-6.5 9.5 6.5 9.5 6.5-3.5 6.5-9.5 6.5S2.5 12 2.5 12z\"/><circle cx=\"12\" cy=\"12\" r=\"3\"/><path d=\"M4 4l16 16\"/>",
	"control": "<path d=\"M4 7h9M17 7h3M4 17h4M12 17h8\"/><circle cx=\"15\" cy=\"7\" r=\"2\"/><circle cx=\"10\" cy=\"17\" r=\"2\"/>",
	"storage": "<rect x=\"3\" y=\"5\" width=\"18\" height=\"14\" rx=\"2\"/><path d=\"M3 13h18M7 16h4\"/><circle cx=\"17\" cy=\"16\" r=\"0.8\"/>",
	"resources": "<path d=\"M3 12h4l3-7 4 14 3-7h4\"/>",
	"logs": "<path d=\"M7 3h7l4 4v14H7z\"/><path d=\"M14 3v4h4M10 11h5M10 15h5\"/>",
	"apps": "<rect x=\"4\" y=\"4\" width=\"7\" height=\"7\" rx=\"1.5\"/><rect x=\"13\" y=\"4\" width=\"7\" height=\"7\" rx=\"1.5\"/><rect x=\"4\" y=\"13\" width=\"7\" height=\"7\" rx=\"1.5\"/><path d=\"M16.5 13.5v6M13.5 16.5h6\"/>",
	"support": "<circle cx=\"12\" cy=\"12\" r=\"9\"/><circle cx=\"12\" cy=\"12\" r=\"4\"/><path d=\"M5.6 5.6l3.6 3.6M14.8 14.8l3.6 3.6M18.4 5.6l-3.6 3.6M9.2 14.8l-3.6 3.6\"/>",
	"lifebuoy": "<circle cx=\"12\" cy=\"12\" r=\"9\"/><circle cx=\"12\" cy=\"12\" r=\"4\"/><path d=\"M5.6 5.6l3.6 3.6M14.8 14.8l3.6 3.6M18.4 5.6l-3.6 3.6M9.2 14.8l-3.6 3.6\"/>",
	"help": "<circle cx=\"12\" cy=\"12\" r=\"9\"/><path d=\"M9.5 9.5a2.5 2.5 0 1 1 3.5 2.3c-.6.3-1 .9-1 1.6v.6M12 17h.01\"/>",
	"bundle": "<path d=\"M4 7.5l8-4 8 4v9l-8 4-8-4z\"/><path d=\"M4 7.5l8 4 8-4M12 11.5v9\"/>",
	"bell": "<path d=\"M6 16V11a6 6 0 0 1 12 0v5l1.5 2h-15z\"/><path d=\"M10 20.5a2 2 0 0 0 4 0\"/>",
	"search": "<circle cx=\"11\" cy=\"11\" r=\"7\"/><path d=\"M21 21l-4.35-4.35\"/>",
	"widgets": "<rect x=\"3\" y=\"4\" width=\"18\" height=\"16\" rx=\"2\"/><path d=\"M14 4v16\"/>",
	"volume": "<path d=\"M4 9.5h3.5L12 6v12l-4.5-3.5H4z\"/><path d=\"M15.5 9a4 4 0 0 1 0 6M18 6.5a7.5 7.5 0 0 1 0 11\"/>",
	"muted": "<path d=\"M4 9.5h3.5L12 6v12l-4.5-3.5H4z\"/><path d=\"M16 9.5l5 5M21 9.5l-5 5\"/>",
	"mic": "<rect x=\"9\" y=\"3\" width=\"6\" height=\"11\" rx=\"3\"/><path d=\"M5.5 11a6.5 6.5 0 0 0 13 0M12 17.5V21\"/>",
	"mic-off": "<rect x=\"9\" y=\"3\" width=\"6\" height=\"11\" rx=\"3\"/><path d=\"M5.5 11a6.5 6.5 0 0 0 13 0M12 17.5V21M4 4l16 16\"/>",
	"gear": "<circle cx=\"12\" cy=\"12\" r=\"3\"/><path d=\"M12 2.5v3M12 18.5v3M2.5 12h3M18.5 12h3M5.3 5.3l2.1 2.1M16.6 16.6l2.1 2.1M5.3 18.7l2.1-2.1M16.6 7.4l2.1-2.1\"/>",
	"lock": "<rect x=\"5\" y=\"10.5\" width=\"14\" height=\"10\" rx=\"2\"/><path d=\"M8 10.5V7.5a4 4 0 0 1 8 0v3\"/>",
	"alert": "<path d=\"M12 3.5l9.5 16.5h-19z\"/><path d=\"M12 10v4.5M12 17.5v.01\"/>",
	"shield": "<path d=\"M12 3l7.5 3v5.5c0 4.5-3.2 8-7.5 9.5-4.3-1.5-7.5-5-7.5-9.5V6z\"/><path d=\"M8.5 12l2.5 2.5 4.5-5\"/>",
	"minimise": "<path d=\"M6 12h12\"/>",
	"maximise": "<rect x=\"6\" y=\"6\" width=\"12\" height=\"12\" rx=\"1.5\"/>",
	"close": "<path d=\"M7 7l10 10M17 7L7 17\"/>",
	"check": "<path d=\"M5 12.5l4.5 4.5L19 7.5\"/>",
	"updates": "<path d=\"M12 3l7.5 3v5.5c0 4.5-3.2 8-7.5 9.5-4.3-1.5-7.5-5-7.5-9.5V6z\"/><path d=\"M12 8v6.5M9 11.5l3 3 3-3\"/>",
	"firewall": "<path d=\"M3 5h18v14H3zM3 9.7h18M3 14.3h18M8 5v4.7M15 5v4.7M11.5 9.7v4.6M6 14.3V19M17 14.3V19\"/>",
	"apparmor": "<path d=\"M12 3l7.5 3v5.5c0 4.5-3.2 8-7.5 9.5-4.3-1.5-7.5-5-7.5-9.5V6z\"/><rect x=\"9\" y=\"11\" width=\"6\" height=\"5\" rx=\"1\"/><path d=\"M10.2 11V9.5a1.8 1.8 0 0 1 3.6 0V11\"/>",
	"key": "<circle cx=\"8\" cy=\"12\" r=\"4\"/><path d=\"M12 12h9M18 12v3M21 12v2\"/>",
	"refresh": "<path d=\"M20 11a8 8 0 0 0-14.3-4.9L4 8M4 4v4h4M4 13a8 8 0 0 0 14.3 4.9L20 16M20 20v-4h-4\"/>",
	"chevron-down": "<path d=\"M6 9.5l6 6 6-6\"/>",
	"chevron-up": "<path d=\"M6 14.5l6-6 6 6\"/>",
	"keyboard": "<rect x=\"2.5\" y=\"6\" width=\"19\" height=\"12\" rx=\"2\"/><path d=\"M6 9.5h.01M9 9.5h.01M12 9.5h.01M15 9.5h.01M18 9.5h.01M6 12.5h.01M9 12.5h.01M12 12.5h.01M15 12.5h.01M18 12.5h.01M8 15.5h8\"/>",
	"mouse": "<rect x=\"6.5\" y=\"3\" width=\"11\" height=\"18\" rx=\"5.5\"/><path d=\"M12 6.5v3\"/>",
	"pointer": "<path d=\"M6 3.5l12 8.5-5.2 1 3 5.6-2.4 1.3-3-5.6L6.5 17.5z\"/>",
	"accel": "<path d=\"M10 4.5l10 7.2-4.4.9 2.5 4.7-2 1.1-2.5-4.7-3.4 3z\"/><path d=\"M2.5 8h4.5M3.5 11.5h4M2.5 15h4\"/>",
	"scroll": "<rect x=\"7.5\" y=\"5.5\" width=\"9\" height=\"13\" rx=\"4.5\"/><path d=\"M12 8.5v2.5M10 3l2-1.5L14 3M10 21l2 1.5 2-1.5\"/>",
	"screen-move": "<rect x=\"3\" y=\"4\" width=\"18\" height=\"12.5\" rx=\"1.5\"/><path d=\"M8 20.5h8M12 16.5v4M7.5 12l3-3M7.5 12v-2.2M7.5 12h2.2M16.5 8.5l-3 3M16.5 8.5v2.2M16.5 8.5h-2.2\"/>",
	"content-move": "<path d=\"M10 21v-8.5a1.5 1.5 0 0 1 3 0v3.5l4 .8a2 2 0 0 1 1.6 2.3L18 21\"/><path d=\"M13 11V6.5a1.5 1.5 0 0 0-3 0v6M15.5 4.5h5M18.5 2.5l2 2-2 2\"/>",
	"ethernet": "<rect x=\"7\" y=\"3\" width=\"10\" height=\"9\" rx=\"1\"/><path d=\"M9.5 3v2.5M12 3v2.5M14.5 3v2.5M9 12v3h6v-3M12 15v6\"/>",
	"ethernet-off": "<rect x=\"7\" y=\"3\" width=\"10\" height=\"9\" rx=\"1\"/><path d=\"M9.5 3v2.5M12 3v2.5M14.5 3v2.5M9 12v3h6v-3M12 15v6M3.5 3.5l17 17\"/>",
	"speed": "<path d=\"M4.5 17a8.5 8.5 0 1 1 15 0\"/><path d=\"M12 13l4-4.5\"/><circle cx=\"12\" cy=\"13\" r=\"1.2\"/>",
	"wifi-off": "<path d=\"M5 12.5a10 10 0 0 1 4-2.3M2 9a14.5 14.5 0 0 1 4.5-2.8M14.5 10.2A10 10 0 0 1 19 12.5M17.5 6.2A14.5 14.5 0 0 1 22 9M8.5 16a5 5 0 0 1 7 0M12 19.5h.01M3 3l18 18\"/>",
	"swap": "<path d=\"M4 8h15M15 4l4 4-4 4M20 16H5M9 12l-4 4 4 4\"/>",
	"ip-screen": "<rect x=\"2.5\" y=\"4\" width=\"14\" height=\"10.5\" rx=\"1.5\"/><path d=\"M6 18.5h5M9.5 14.5v4\"/><rect x=\"13.5\" y=\"12\" width=\"8\" height=\"8\" rx=\"1.5\"/><path d=\"M15.7 14v4M17.8 18v-4h1.2a1.1 1.1 0 0 1 0 2.2h-1.2\"/>",
	"layers": "<path d=\"M12 3.5l9 4.5-9 4.5-9-4.5z\"/><path d=\"M3 12l9 4.5 9-4.5M3 16l9 4.5 9-4.5\"/>",
	"router": "<rect x=\"3\" y=\"13\" width=\"18\" height=\"6.5\" rx=\"1.5\"/><path d=\"M7 16.2h.01M10 16.2h.01M12 13V9M8.5 7.5a5 5 0 0 1 7 0M6 5a8.5 8.5 0 0 1 12 0\"/>",
	"dns": "<rect x=\"3\" y=\"4\" width=\"11\" height=\"7\" rx=\"1.2\"/><rect x=\"3\" y=\"13\" width=\"11\" height=\"7\" rx=\"1.2\"/><path d=\"M6 7.5h.01M6 16.5h.01M17 4.5v6h1a3 3 0 0 0 0-6zM21 14.5a2 2 0 0 0-2-1.5 1.8 1.8 0 0 0 0 3.6 1.8 1.8 0 0 1 0 3.6 2 2 0 0 1-2-1.5\"/>",
	"monitor": "<rect x=\"3\" y=\"4\" width=\"18\" height=\"12.5\" rx=\"1.5\"/><path d=\"M8 20.5h8M12 16.5v4\"/>",
	"rotate": "<rect x=\"8\" y=\"8\" width=\"10\" height=\"13\" rx=\"1.5\"/><path d=\"M4 11a8 8 0 0 1 10-7.5M14 3.5l-2.5-1.5M14 3.5l-1.5 2.5\"/>",
	"resolution": "<path d=\"M4 8V4h4M16 4h4v4M20 16v4h-4M8 20H4v-4\"/><path d=\"M7.5 15V9h2a1.6 1.6 0 0 1 0 3.2h-2M13 9l3.5 6M16.5 9L13 15\"/>",
	"rate": "<circle cx=\"12\" cy=\"13.5\" r=\"7.5\"/><path d=\"M10 2.5h4M12 2.5v3.5M7.5 13.5h2.5l1.2-2.5 1.6 5 1.2-2.5h2.5\"/>",
	"text-size": "<path d=\"M3 17l4-11 4 11M4.5 13h5M17.5 17v-5.5a2.5 2.5 0 0 0-5 0M17.5 14.5h-2.5a2 2 0 0 0 0 4h2.5M3 20.5h18\"/>",
	"info": "<circle cx=\"12\" cy=\"12\" r=\"8.5\"/><path d=\"M12 11v5M12 8h.01\"/>",
	"alert-gear": "<path d=\"M12 2.5l2 2.2 2.9-.6.6 2.9 2.7 1.3-1.2 2.7 1.2 2.7-2.7 1.3-.6 2.9-2.9-.6-2 2.2-2-2.2-2.9.6-.6-2.9-2.7-1.3 1.2-2.7-1.2-2.7 2.7-1.3.6-2.9 2.9.6z\"/><path d=\"M12 8v4.5M12 15.5h.01\"/>",
	"maintenance": "<path d=\"M4 12a8 8 0 1 0 2.3-5.7L4 8.5M4 4.5v4h4\"/><path d=\"M12 8v4.5l3 2\"/>",
	"scan": "<rect x=\"3.5\" y=\"5\" width=\"15\" height=\"14\" rx=\"2\"/><path d=\"M3.5 9.5h15M7.5 3v4M14.5 3v4\"/><circle cx=\"18\" cy=\"18\" r=\"3.5\"/><path d=\"M16.5 18l1 1 2-2\"/>",
	"power": "<path d=\"M12 3v8M7 6.3a7.5 7.5 0 1 0 10 0\"/>",
	"restart": "<path d=\"M19.5 12a7.5 7.5 0 1 1-2.2-5.3L19.5 9M19.5 4.5V9H15\"/>",
	"sign-out": "<path d=\"M10 4H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h4\"/><path d=\"M15 7.5l4.5 4.5-4.5 4.5M19.5 12H9\"/>",
	"bolt": "<path d=\"M13 2.5L5 13.5h6l-1 8 8-11h-6z\"/>",
	"calendar-clock": "<rect x=\"3.5\" y=\"5\" width=\"13\" height=\"13\" rx=\"2\"/><path d=\"M3.5 9.5h13M7 3v4M13 3v4\"/><circle cx=\"17\" cy=\"17\" r=\"4\"/><path d=\"M17 15v2l1.3 1\"/>",
	"restore-point": "<path d=\"M6 3.5h8l4 4v6M6 3.5v17h6\"/><path d=\"M14 3.5v4h4\"/><path d=\"M15 21a3.5 3.5 0 1 0-3.2-4.8M11.5 14v2.5H14\"/>",
	"factory": "<path d=\"M3 13.5l3-8h12l3 8v5a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 18.5z\"/><path d=\"M3 13.5h5l1.5 2.5h5l1.5-2.5h5\"/>",
	"held": "<circle cx=\"6\" cy=\"5.5\" r=\"2\"/><circle cx=\"6\" cy=\"18.5\" r=\"2\"/><circle cx=\"18\" cy=\"12\" r=\"2\"/><path d=\"M6 7.5v9M8 5.5h3a5 5 0 0 1 5 5v-.5\"/>",
	"reset": "<path d=\"M9 4L5 8l4 4\"/><path d=\"M5 8h9a5 5 0 0 1 0 10H8\"/>",
	"erase": "<rect x=\"3.5\" y=\"3.5\" width=\"12\" height=\"15\" rx=\"1.5\"/><path d=\"M7 8h5M7 11.5h5\"/><path d=\"M16 15.5l5 5M21 15.5l-5 5\"/>",
	"trash": "<path d=\"M4.5 6.5h15M9.5 6.5V4h5v2.5M6.5 6.5l1 13.5h9l1-13.5M10 10.5v6M14 10.5v6\"/>",
	"speaker": "<rect x=\"5\" y=\"2.5\" width=\"14\" height=\"19\" rx=\"2\"/><circle cx=\"12\" cy=\"14.5\" r=\"3.5\"/><path d=\"M12 7h.01\"/>",
	"sliders": "<path d=\"M5 4v16M12 4v16M19 4v16\"/><circle cx=\"5\" cy=\"14\" r=\"2\"/><circle cx=\"12\" cy=\"8\" r=\"2\"/><circle cx=\"19\" cy=\"16\" r=\"2\"/>",
	"wave": "<path d=\"M3 12h2M7 8v8M10 5v14M13 9v6M16 7v10M19 10v4M21 12h.01\"/>",
	"play": "<path d=\"M7 4.5v15l12-7.5z\"/>",
	"globe": "<circle cx=\"12\" cy=\"12\" r=\"8.5\"/><path d=\"M3.5 12h17M12 3.5a13 13 0 0 1 0 17M12 3.5a13 13 0 0 0 0 17\"/>",
	"pin": "<path d=\"M12 21s-6.5-6-6.5-11a6.5 6.5 0 0 1 13 0c0 5-6.5 11-6.5 11z\"/><circle cx=\"12\" cy=\"10\" r=\"2.3\"/>",
	"tag": "<path d=\"M3.5 12.5V4.5a1 1 0 0 1 1-1h8l8 8-9 9z\"/><circle cx=\"8\" cy=\"8\" r=\"1.4\"/>",
	"box": "<rect x=\"3\" y=\"8\" width=\"18\" height=\"9\" rx=\"2\"/><path d=\"M7 12.5h.01M10 12.5h.01M15 12.5h3\"/>",
	"barcode": "<path d=\"M4 5v14M7 5v14M10.5 5v14M13 5v14M17 5v14M20 5v14\"/>",
	"cpu": "<rect x=\"6\" y=\"6\" width=\"12\" height=\"12\" rx=\"1.5\"/><rect x=\"9.5\" y=\"9.5\" width=\"5\" height=\"5\"/><path d=\"M9 3v3M15 3v3M9 18v3M15 18v3M3 9h3M3 15h3M18 9h3M18 15h3\"/>",
	"memory": "<rect x=\"2.5\" y=\"7\" width=\"19\" height=\"9\" rx=\"1\"/><path d=\"M6 10v3M9.5 10v3M13 10v3M16.5 10v3M5 16v2.5M19 16v2.5\"/>",
	"disk": "<ellipse cx=\"12\" cy=\"6\" rx=\"8\" ry=\"2.8\"/><path d=\"M4 6v12c0 1.5 3.6 2.8 8 2.8s8-1.3 8-2.8V6M4 12c0 1.5 3.6 2.8 8 2.8s8-1.3 8-2.8\"/>",
	"thermometer": "<path d=\"M10 14.5V5a2 2 0 0 1 4 0v9.5a4 4 0 1 1-4 0z\"/><path d=\"M12 9v7\"/>",
	"user": "<circle cx=\"12\" cy=\"8.5\" r=\"4\"/><path d=\"M4.5 20.5a7.5 7.5 0 0 1 15 0\"/>",
	"package": "<path d=\"M12 3l8 4.5v9L12 21l-8-4.5v-9z\"/><path d=\"M4 7.5l8 4.5 8-4.5M12 12v9\"/>",
	"clock": "<circle cx=\"12\" cy=\"12\" r=\"8.5\"/><path d=\"M12 7.5V12l3 2\"/>",
	"web": "<rect x=\"3\" y=\"4.5\" width=\"18\" height=\"15\" rx=\"2\"/><path d=\"M3 9h18M6 6.75h.01M8.5 6.75h.01M11 6.75h.01\"/>",
	"discovery": "<circle cx=\"12\" cy=\"10\" r=\"1.6\"/><path d=\"M12 11.6L9 21h6l-3-9.4\"/><path d=\"M8.5 6.5a5 5 0 0 0 0 7M15.5 6.5a5 5 0 0 1 0 7M5.7 3.7a9 9 0 0 0 0 12.6M18.3 3.7a9 9 0 0 1 0 12.6\"/>",
	"address": "<rect x=\"4\" y=\"3.5\" width=\"16\" height=\"17\" rx=\"2.5\"/><path d=\"M4 14.5h16M15 17.5h2\"/><path d=\"M8.5 6.5v5M11.5 11.5v-5h2.5a1.5 1.5 0 0 1 0 3h-2.5\"/>",
	"terminal": "<rect x=\"3\" y=\"4.5\" width=\"18\" height=\"15\" rx=\"2\"/><path d=\"M7 10l3 2.5L7 15M12.5 15H17\"/>",
	"usb": "<rect x=\"7\" y=\"9\" width=\"10\" height=\"12\" rx=\"2\"/><path d=\"M9 9V3h6v6M11 5.5h.01M13 5.5h.01\"/>",
	"app": "<rect x=\"4\" y=\"4\" width=\"7\" height=\"7\" rx=\"1.5\"/><rect x=\"13\" y=\"4\" width=\"7\" height=\"7\" rx=\"1.5\"/><rect x=\"4\" y=\"13\" width=\"7\" height=\"7\" rx=\"1.5\"/><rect x=\"13\" y=\"13\" width=\"7\" height=\"7\" rx=\"1.5\"/>",
	"palette": "<path d=\"M12 3a9 9 0 1 0 0 18c1.1 0 1.7-.8 1.7-1.7 0-.5-.2-.9-.5-1.2-.3-.3-.5-.7-.5-1.2 0-.9.8-1.7 1.7-1.7h2.1A4.5 4.5 0 0 0 21 10.7C21 6.4 17 3 12 3z\"/><circle cx=\"7.5\" cy=\"11\" r=\"1.2\"/><circle cx=\"10\" cy=\"7\" r=\"1.2\"/><circle cx=\"14.5\" cy=\"7\" r=\"1.2\"/>",
	"picture": "<rect x=\"3\" y=\"4.5\" width=\"18\" height=\"15\" rx=\"2.5\"/><circle cx=\"8.5\" cy=\"9.5\" r=\"1.6\"/><path d=\"M21 15.5l-5-5L5.5 19.5\"/>",
	"plus": "<path d=\"M12 5v14M5 12h14\"/>",
	"phone": "<rect x=\"7\" y=\"2.5\" width=\"10\" height=\"19\" rx=\"2.5\"/><path d=\"M11 18.5h2\"/>",
	"backup": "<path d=\"M4 12a8 8 0 1 0 2.4-5.7\"/><path d=\"M4 4.5v4h4\"/><path d=\"M12 8v4.2l3 1.8\"/>",
	"pushpin": "<path d=\"M9 3.5h6M10 3.5v5.2L7 12.5h10l-3-3.8V3.5M12 12.5V20.5\"/>",
	"contrast": "<circle cx=\"12\" cy=\"12\" r=\"9\"/><path d=\"M12 3a9 9 0 0 1 0 18z\" fill=\"{c}\"/>",
	"lock-screen": "<rect x=\"5\" y=\"10.5\" width=\"14\" height=\"10\" rx=\"2\"/><path d=\"M8.5 10.5V7.5a3.5 3.5 0 0 1 7 0v3\"/><path d=\"M12 14.5v2.5\"/>",
	"moon": "<path d=\"M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5z\"/>",
	"sun": "<circle cx=\"12\" cy=\"12\" r=\"4\"/><path d=\"M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4\"/>",
};

/** Each badge shade, top to bottom of its gradient. */
export const BADGE_SHADES: Record<string, [string, string]> = {
	blue: ["#7CB6FF", "#1D63D8"],
	cyan: ["#5FD8EA", "#0A7A96"],
	indigo: ["#A49BFF", "#4B34C8"],
	pink: ["#FF8FB1", "#D61F69"],
	graphite: ["#AEB8C8", "#4A5568"],
	green: ["#6EE29A", "#15803D"],
	teal: ["#5EDDB9", "#08795F"],
	orange: ["#FFB565", "#E5560B"],
	red: ["#FF8A80", "#D2261C"],
	yellow: ["#FFD965", "#D98C00"],
};

/** Each line icon's badge colour, by what it's about; not listed: blue. */
export const BADGE: Record<string, string> = {};
for (const [shade, names] of Object.entries({
	cyan: "phone backup disk usb storage bundle layers package",
	indigo: "palette picture contrast monitor resolution rotate rate text-size screen-move factory",
	pink: "speaker sliders wave mic mic-off volume muted play",
	graphite: "keyboard mouse pointer accel scroll content-move gear box barcode tag cpu memory terminal",
	green: "check shield updates apparmor lock lock-screen key scan",
	teal: "lifebuoy support help",
	orange: "maintenance clock calendar-clock power bolt restore-point reset held thermometer",
	red: "alert alert-gear trash erase firewall wifi-off",
	yellow: "bell pin logs",
})) for (const name of names.split(' ')) BADGE[name] = shade;

/** Each section and the other words people use for it, in the menu's order. */
export const SECTION_WORDS: [string, string, string][] = [
	["control", "network", N_("ethernet cable ip address dhcp fixed static router gateway dns lan wifi wireless internet")],
	["control", "display", N_("screen monitor resolution refresh rate rotate orientation scale size text")],
	["control", "appearance", N_("theme dark light night day midnight daylight accent colour color wallpaper background picture photo desktop look")],
	["control", "sound", N_("audio speakers headphones volume microphone mic input output mute")],
	["control", "keyboard", N_("layout keys mouse cursor pointer speed left handed scroll touchpad")],
	["control", "language", N_("language translation country region province state locale")],
	["control", "time", N_("clock date time zone timezone ntp sync")],
	["control", "people", N_("users accounts members family admins add person password forgot children kids parents guardians")],
	["control", "notifications", N_("alerts bell messages")],
	["control", "security", N_("firewall ports updates patches secure boot apparmor scan integrity privacy outbound")],
	["control", "access", N_("ssh terminal command line shell remote sudo admin keys")],
	["control", "restarts", N_("reboot restart power shut down shutdown maintenance hour always on sleep")],
	["control", "restore", N_("restore rollback undo update snapshot reset factory erase wipe")],
	["control", "about", N_("about system info model serial number version processor memory support period")],
	["storage", "overview", N_("disk space library mirror check scrub damage full")],
	["storage", "drives", N_("drive disk health smart failing replace temperature")],
	["storage", "space", N_("space usage full photos apps databases")],
	["storage", "backups", N_("backup backups restic copy nightly restore safe offsite")],
	["apps", "installed", N_("apps applications installed remove uninstall delete restore version update running")],
	["apps", "usb", N_("install add app usb stick package zapp sideload new update")],
	["resources", "overview", N_("cpu processor memory ram load network activity performance")],
	["resources", "apps", N_("app service busy slow usage")],
	["resources", "temperature", N_("temperature heat hot fan sensors")],
	["logs", "events", N_("events history updates restarts")],
	["logs", "notifications", N_("past notifications alerts history")],
	["logs", "audit", N_("audit who changed admin actions history")],
	["logs", "signins", N_("sign in login logins who")],
	["logs", "activity", N_("family activity sign-ins wrong password devices share links guest links settings accounts second step who did what")],
	["logs", "system", N_("journal system log errors warnings")],
	["logs", "diagnostics", N_("diagnostics bundle logs support")],
	["support", "diagnostics", N_("diagnostics bundle logs debug report usb send support problem")],
	["support", "contact", N_("contact support help ticket serial number phone email")],
	["support", "help", N_("help questions faq how to troubleshoot problem")],
];

/** Each setting, under its section: the words its row shows. */
export const SETTINGS: [string, string, string][] = [
	["control", "network", N_("IP Address Assignment Method")],
	["control", "network", N_("Wi-Fi (Wireless Network)")],
	["control", "display", N_("Resolution")],
	["control", "display", N_("Orientation")],
	["control", "display", N_("Screen Update Rate")],
	["control", "display", N_("Text & UI Scaling")],
	["control", "appearance", N_("Style")],
	["control", "appearance", N_("Accent Colour")],
	["control", "appearance", N_("Background")],
	["control", "appearance", N_("Add Picture")],
	["control", "sound", N_("Output Device")],
	["control", "sound", N_("Output Volume")],
	["control", "sound", N_("Input Device")],
	["control", "sound", N_("Input Volume")],
	["control", "sound", N_("Input Level")],
	["control", "keyboard", N_("Input Layout")],
	["control", "keyboard", N_("Primary click side")],
	["control", "keyboard", N_("Cursor Speed")],
	["control", "keyboard", N_("Dynamic Cursor Acceleration")],
	["control", "keyboard", N_("Scroll direction")],
	["control", "language", N_("Language")],
	["control", "language", N_("Country or Region")],
	["control", "language", N_("State or Province")],
	["control", "time", N_("Time Zone")],
	["control", "time", N_("Current Time")],
	["control", "time", N_("Time Sync")],
	["control", "people", N_("Add a person")],
	["control", "people", N_("New password")],
	["control", "people", N_("Make admin")],
	["control", "notifications", N_("Show Notifications For")],
	["control", "security", N_("Firewall")],
	["control", "security", N_("Secure Boot")],
	["control", "security", N_("AppArmor")],
	["control", "security", N_("Security Updates")],
	["control", "security", N_("Last System Scan")],
	["control", "security", N_("Allowed Inbound Connections")],
	["control", "security", N_("Outbound Internet Connections")],
	["control", "access", N_("Keys")],
	["control", "access", N_("Add from a USB stick")],
	["control", "access", N_("Paste a key")],
	["control", "restarts", N_("Maintenance Hour")],
	["control", "restarts", N_("Always On")],
	["control", "restarts", N_("Shutdown")],
	["control", "restore", N_("Restore points")],
	["control", "restore", N_("Updates on Hold")],
	["control", "restore", N_("System Reset")],
	["control", "restore", N_("Factory Reset")],
	["control", "restore", N_("Erase everything")],
	["control", "about", N_("Family Hub")],
	["control", "about", N_("Open on a phone")],
	["control", "about", N_("Model")],
	["control", "about", N_("Serial Number")],
	["control", "about", N_("Operating System")],
	["control", "about", N_("Security updates")],
	["storage", "overview", N_("Library")],
	["storage", "overview", N_("Check the library for damage")],
	["storage", "backups", N_("Back up now")],
	["storage", "backups", N_("Nightly backup")],
	["control", "display", N_("Lock the screen after")],
	["control", "display", N_("Turn the screen off after")],
	["support", "diagnostics", N_("Make a bundle")],
	["support", "diagnostics", N_("Save to a USB stick")],
	["support", "contact", N_("Contact ZOOMRIYA support")],
	["support", "contact", N_("Serial number")],
];

/** The top bar's actions: what it says, what it does, and other words for it. */
export const ACTIONS: [string, string, string][] = [
	[N_("Restart the box"), "restart", N_("reboot")],
	[N_("Shut down system"), "shut-down", N_("power off turn off")],
	[N_("Lock screen"), "lock", N_("lock away screen saver leave")],
	[N_("Open Family Hub on a phone"), "hub-phone", N_("qr code phone mobile address family hub")],
	[N_("Sign out"), "sign-out", N_("log out logout switch user")],
	[N_("Midnight"), "midnight", N_("dark theme night")],
	[N_("Daylight"), "daylight", N_("light theme day")],
	[N_("Widgets"), "widgets", N_("side panel health cpu space")],
];

/** Help for the other tools: the question, its answer, and the tool and section that answers it. */
export const HELP: [string, string, string, string][] = [
	[N_("Something stopped working after an update"), N_("Restore the OS to before the update. Family files stay."), "control", "restore"],
	[N_("A drive needs checking"), N_("See which drive, and why. Replace it with one as big or bigger."), "storage", "drives"],
	[N_("The box is slow"), N_("See which app is busy. Photos works hard after many new photos."), "resources", "apps"],
	[N_("The box restarted by itself"), N_("Updates finish at the maintenance hour. You can change the hour."), "control", "restarts"],
	[N_("Family Hub doesn't open on the home network"), N_("Check the cable to the router, and the box's address."), "control", "network"],
	[N_("Someone can't sign in"), N_("Let them type a new password, or turn their account back on."), "control", "people"],
	[N_("Who changed a setting?"), N_("Every change, with who and when."), "logs", "audit"],
];

/** Each kind of notification: its name, what sends one and its icon. */
export const KINDS: Record<string, [string, string, string]> = {
	drive: [N_("Drive Health"), N_("When to check or replace, and when healthy."), "disk"],
	update: [N_("Updates"), N_("When security fixes are installed."), "updates"],
	restart: [N_("Restarts"), N_("Auto-restart schedule for updates and restores."), "power"],
	check: [N_("Library Check"), N_("Library scan completion time and results."), "scan"],
	network: [N_("Network"), N_("Network settings weren't kept and were rolled back."), "ethernet"],
	secure: [N_("Secure Connections"), N_("When the box's certificate can't be made or renewed, or is running out."), "lock"],
	signin: [N_("Admin Sign-in"), N_("When an admin is let back in at the box with the setup code, or a wrong code is typed."), "key"],
	access: [N_("Advanced Access"), N_("When SSH access is turned on or off."), "terminal"],
	apps: [N_("Apps"), N_("When an app is installed, restored or removed."), "app"],
};

/** Why a connection an app declares is blocked: the agent's words (internet.go). */
export const BLOCKED_REASONS = [N_("Everything offline is on"), N_("Apps offline is on"), N_("Switched off")];

/** Each accent: its name, then Daylight's colour and hover, Midnight's colour and hover. */
export const ACCENTS: Record<string, [string, string, string, string, string]> = {
	blue: [N_("Blue"), "#1765c1", "#0f4c95", "#7b93ff", "#9aabff"],
	teal: [N_("Teal"), "#0b7285", "#095c6b", "#3bc9db", "#66d9e8"],
	green: [N_("Green"), "#237a3b", "#1b5e2d", "#51cf66", "#8ce99a"],
	yellow: [N_("Yellow"), "#946200", "#7a5000", "#fcc419", "#ffd43b"],
	orange: [N_("Orange"), "#c2410c", "#9a3412", "#ff922b", "#ffa94d"],
	red: [N_("Red"), "#c92a2a", "#a51d1d", "#ff8787", "#ffa8a8"],
	pink: [N_("Pink"), "#c2255c", "#a01d4b", "#f783ac", "#faa2c1"],
	purple: [N_("Purple"), "#7048e8", "#5f3dc4", "#b197fc", "#c8b6ff"],
	slate: [N_("Slate"), "#4a5568", "#2d3748", "#a5b4c8", "#c3cedb"],
};
export const DEFAULT_ACCENT = "blue";

/** Our pictures: their names and files. Smoke is the box's own, the sign-in's. */
export const PICTURES: Record<string, [string, string]> = {
	smoke: [N_("Smoke"), "wallpaper.jpg"],
	slate: [N_("Smoke at dusk"), "slate.jpg"],
	dusk: [N_("Hills"), "dusk.jpg"],
	lakeside: [N_("Lakeside"), "lakeside.jpg"],
};
/** Plain colours, dark and quiet enough for the shortcuts' white labels. */
export const COLOURS: Record<string, [string, string]> = {
	night: [N_("Night"), "#0b1220"],
	ocean: [N_("Ocean"), "#0b4f6c"],
	forest: [N_("Forest"), "#1f4d3a"],
	plum: [N_("Plum"), "#4a2545"],
	graphite: [N_("Graphite"), "#2d3340"],
};
export const DEFAULT_BACKGROUND = "smoke";
/** An admin's own picture, from a USB stick. */
export const OWN = "own";

/** ZOOMRIYA ID's kinds of activity, as an admin reads them. */
export const ACTIVITY_KINDS: Record<string, string> = {
	"sign-in": N_("Sign-in"),
	"sign-in-failed": N_("Wrong try"),
	"second-step": N_("Second step"),
	"password": N_("Password"),
	"account": N_("Account"),
	"ai-key": N_("AI key"),
	"device": N_("New device"),
	"share-link": N_("Share link"),
	"setting": N_("Setting"),
};

export const LOG_LEVELS: [string, string][] = [["errors", N_("Errors")], ["warnings", N_("Warnings and errors")], ["everything", N_("Everything")]];
export const DRIVE_HEALTH: Record<string, string> = {healthy: N_("Healthy"), warning: N_("Needs checking"), failing: N_("Failing")};

/** What the agent says it's doing, and what it says of a package it can't use. */
export const APP_STEPS = [N_("Starting"), N_("Copying from the USB stick"), N_("Checking it's a ZOOMRIYA app"), N_("Adding it to the box"), N_("Starting it"), N_("Not a ZOOMRIYA app."), N_("Can't be read.")];
export const APP_HEALTH: Record<string, [string, string]> = {running: [N_("Running"), "ok"], starting: [N_("Starting"), "warn"], stopped: [N_("Stopped"), "warn"], failed: [N_("Not working"), "warn"]};

/** sway's transforms, as the person reads them. */
export const ORIENTATIONS: [string, string][] = [["normal", N_("Landscape")], ["90", N_("Portrait, turned right")], ["270", N_("Portrait, turned left")], ["180", N_("Landscape, upside down")]];
export const SCALES = [1.0, 1.25, 1.5, 1.75, 2.0];

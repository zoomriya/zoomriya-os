# Licences for ZOOMRIYA OS

Part of [ZOOMRIYA OS](../README.md). November 2, 2026.

ZOOMRIYA OS uses components whose licences allow them to be included in a
commercial product. This page lists the components named in the
[design](design.md), their licences, whether commercial use is allowed and
any obligations we need to meet. We review each new component before adding
it to an image.

This is our understanding of the licences, not legal advice.

## On the box

For Debian 13 packages, the licence information below was checked against
each package's `copyright` file.

| Component | Licence | Commercial use | What we need to do |
| --- | --- | --- | --- |
| Debian 13, `main` | Many free licences, all meeting the Debian Free Software Guidelines | Yes | GPL and LGPL: offer the source of what we ship (below). Don't call the product Debian or use its logo |
| Debian `non-free-firmware` (CPU microcode, Ethernet firmware) | Each vendor's own; redistribution allowed | Yes | Ship only what the box's hardware needs, with each licence file; no Wi-Fi firmware ([Design §5.1](design.md#51-wi-fi)) |
| Linux kernel, Btrfs | GPL-2.0 | Yes | Source |
| systemd, systemd-nspawn, machinectl, systemd-boot | LGPL-2.1+, some GPL-2.0 | Yes | Source |
| apt, unattended-upgrades, needrestart, snapper | GPL-2.0 | Yes | Source |
| smartmontools (drives' health, [Design §7](design.md#7-zoomriya-agent)) | GPL-2.0+ | Yes | Source |
| console-setup and keyboard-configuration (the keyboard layout on the box's own screen) | GPL-2.0+ | Yes | Source |
| OpenSSH server (support mode and advanced access, [Design §7.1](design.md#71-advanced-access)) | BSD-style (OpenSSH's own licence) | Yes | Keep the notices |
| nftables, auditd, AppArmor, AIDE, Avahi | GPL-2.0 or LGPL-2.1+ | Yes | Source |
| PostgreSQL 17 | PostgreSQL Licence (BSD-like) | Yes | Keep the notice |
| sway, wlroots (the box's own screen: the first setup and Box Desktop, [Design §5](design.md#5-first-boot)) | MIT | Yes | Keep the notices |
| GTK 4, PyGObject, librsvg (the first setup's app, the installer, and the window that shows Box Desktop on the box's screen; librsvg draws their icons) | LGPL-2.1+ | Yes | Source |
| PipeWire, WirePlumber (Box Desktop's sound) | MIT | Yes | Keep the notices |
| WebKitGTK, libsoup (Box Desktop on the box's screen, `box-desktop-screen`) | LGPL-2.1+ and BSD-2-Clause | Yes | Source |
| Svelte (Box Desktop's page, built into `box-desktopd`) | MIT | Yes | Keep the notice |
| qrcode-generator (the QR code that opens Family Hub or an app on a phone, in Box Desktop's page) | MIT | Yes | Keep the notice |
| Inter, the web font (`@fontsource-variable/inter`, in Box Desktop's page, as in the apps) | OFL-1.1 | Yes, bundled with software | Keep the licence; don't sell the font on its own |
| Adwaita icon theme (only the small icons GTK draws inside its own controls: a dropdown's arrow, a search field's magnifier, a password's eye; none of ours) | LGPL-3.0 or CC-BY-SA-3.0 | Yes | Source, or the attribution |
| Our icons: Box Desktop's line icons (`web/src/lib/words.ts`, `icons.ts`) and tool tiles (`tiles.ts`) and the installer's (`zoomriya_installer/icons.py`), drawn for ZOOMRIYA OS, none taken from an icon set | AGPL-3.0-only, with this repository | Yes, ours | Nothing more |
| Box Desktop's backgrounds (`packages/box-desktop/web/public/backgrounds/`: `wallpaper.jpg`, `slate.jpg`, `lakeside.jpg`), made for ZOOMRIYA with Adobe Firefly | Adobe's terms: Firefly images may be used commercially | Yes | Nothing |
| Box Desktop's Hills background (`backgrounds/dusk.jpg`, drawn from our own `docs/images/box-desktop-wallpaper.svg`) | AGPL-3.0-only, with this repository | Yes, ours | Nothing more |
| efibootmgr (the installer's boot entry, on the USB stick only) | GPL-2.0+ | Yes | Source |
| erofs-utils (builds the USB stick's read-only system; not shipped) | GPL-2.0+ | Yes; a build tool | Nothing |
| locales (glibc's language data: the box's language, [Design §5](design.md#5-first-boot)) | LGPL-2.1+ | Yes | Source |
| Noto fonts (`fonts-noto-core`, `fonts-noto-cjk`: every language's script on the box's screen) | SIL Open Font License 1.1 | Yes | Keep the licence with the fonts; don't sell the fonts on their own |
| ZOOMRIYA ID (`zoomriya-id`, our own) | AGPL-3.0-only | Yes | Ours; its source is public |
| Python 3.13 (Debian's, for ZOOMRIYA ID) | PSF-2.0 | Yes | Keep the notice |
| ZOOMRIYA ID's Python packages, in its venv: FastAPI, Starlette, Pydantic, SQLAlchemy, Alembic, PyJWT, redis-py, Pillow, httpx, uvicorn, python-dotenv, python-multipart, passlib, cryptography, segno (the second step's QR code) | MIT, BSD or Apache-2.0 (Pillow: MIT-CMU; cryptography: Apache-2.0 or BSD; segno: BSD-3-Clause) | Yes | Keep the notices |
| psycopg 3 (ZOOMRIYA ID's PostgreSQL driver, with its bundled libpq) | LGPL-3.0 (libpq: PostgreSQL Licence) | Yes | Source, and let the box's owner replace it (it's an ordinary Python package in the venv) |
| **Valkey** 8.1 (`valkey-server`, Debian 13) | BSD-3-Clause | Yes | Keep the notice |
| Caddy | Apache-2.0 | Yes | Keep the notice |
| restic (in the backup image) | BSD-2-Clause | Yes | Keep the notice |
| minisign (checking release signatures) | ISC | Yes | Keep the notice |
| Go standard library (inside `zoomriya-agent`) | BSD-3-Clause | Yes | Keep the notice |
| Inter, the font (Debian's `fonts-inter`, for the box's screen: the installer, the first setup and Box Desktop, as in the apps) | OFL-1.1 | Yes, bundled with software | Keep the licence; don't sell the font on its own |
| Qdrant (in Photos' image) | Apache-2.0 | Yes | Keep the notice |
| llama.cpp (in the Myira model service's image) | MIT | Yes | Keep the notice |
| FFmpeg (in Photos' image, Debian's build) | GPL-2.0+ (Debian builds it with `--enable-gpl`, x264 and x265) | Yes, as a separate program | Source |

## Only in our build and CI (never shipped)

| Component | Licence | Commercial use |
| --- | --- | --- |
| mkosi | LGPL-2.1+ | Yes |
| aptly (our mirror of the security archive) | MIT | Yes |
| Lynis | GPL-3.0 | Yes; we only run it |
| Go toolchain | BSD-3-Clause | Yes |
| Node.js, Vite, @sveltejs/vite-plugin-svelte, svelte-check, Vitest (building and testing Box Desktop's page) | MIT | Yes |
| TypeScript | Apache-2.0 | Yes |

## Source we must offer

For GPL and LGPL components shipped on the box, in the operating system or
app images, we must offer the corresponding source to everyone who receives a
box. Each release will publish the Debian source packages for everything it
contains, alongside a software bill of materials, and both will come from the
same build ([design §10](design.md#10-building-and-testing)). The USB
installer for personal computers also distributes these components and will
include the same source information.

**Owners must be able to install their own changes.** Debian includes
GPLv3-licensed software such as bash, coreutils, grep and tar. Because the box
is a consumer device, GPLv3 requires that its owner can install a modified
version. Secure Boot must therefore remain under the owner's control: the
computer's firmware must allow them to disable it or enroll their own keys.
The box must never be locked to our keys alone. This requirement also applies
to the planned read-only A/B system
([design §6.7](design.md#67-later-a-read-only-system)).

## Models

| Model | Licence | Commercial use | What we need to do |
| --- | --- | --- | --- |
| Gemma 4 E2B and E4B (the Myira model service; E2B by default), as GGUF files from `lmstudio-community` | Apache-2.0, Google's and the GGUF repackage's | Yes | Keep the licence and notice with the files |

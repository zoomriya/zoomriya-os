# Licences: what ZOOMRIYA OS may use

Part of [ZOOMRIYA OS](../README.md). November 2, 2026.

**The rule: ZOOMRIYA OS uses only what we may use in a product we sell.** Every
component the [design](design.md) names is below, with its licence, whether it
allows commercial use, and what it asks of us. A new component is added here,
checked, before it goes into an image.

This is our own reading of each licence, not legal advice.

## On the box

Debian's licences were read from the packages' own `copyright` files on Debian
13.

| Component | Licence | Commercial use | What it asks of us |
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
| GTK 4, PyGObject, librsvg (the first setup's app, the installer and Box Desktop on the screen; librsvg draws their icons) | LGPL-2.1+ | Yes | Source |
| PipeWire, WirePlumber (Box Desktop's sound) | MIT | Yes | Keep the notices |
| Adwaita icon theme (the first setup's icons) | LGPL-3.0 or CC-BY-SA-3.0 | Yes | Source, or the attribution |
| Box Desktop's line icons and tool tiles (`box_desktop/icons.py`), drawn for ZOOMRIYA OS, none taken from an icon set | MPL-2.0, with this repository | Yes, ours | Nothing more |
| efibootmgr (the installer's boot entry, on the USB stick only) | GPL-2.0+ | Yes | Source |
| erofs-utils (builds the USB stick's read-only system; not shipped) | GPL-2.0+ | Yes; a build tool | Nothing |
| locales (glibc's language data: the box's language, [Design §5](design.md#5-first-boot)) | LGPL-2.1+ | Yes | Source |
| Noto fonts (`fonts-noto-core`, `fonts-noto-cjk`: every language's script on the box's screen) | SIL Open Font License 1.1 | Yes | Keep the licence with the fonts; don't sell the fonts on their own |
| ZOOMRIYA ID (`zoomriya-id`, our own) | MPL-2.0 | Yes | Ours; its source is public |
| Python 3.13 (Debian's, for ZOOMRIYA ID) | PSF-2.0 | Yes | Keep the notice |
| ZOOMRIYA ID's Python packages, in its venv: FastAPI, Starlette, Pydantic, SQLAlchemy, Alembic, PyJWT, redis-py, Pillow, httpx, uvicorn, python-dotenv, python-multipart, passlib, cryptography | MIT, BSD or Apache-2.0 (Pillow: MIT-CMU; cryptography: Apache-2.0 or BSD) | Yes | Keep the notices |
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

## Source we must offer

GPL and LGPL components shipped on the box, in the OS and in the app images,
need their source offered to anyone who gets a box. Each release publishes the
Debian source packages of everything in it, next to its software bill of
materials, both from the same build ([design §10](design.md#10-building-and-testing)). The installer for
bring-your-own mini PCs is distribution too, so it gets the same.

**The owner must be able to install their own changes.** Debian ships GPLv3
software (bash, coreutils, grep, tar), and the box is a consumer device, so
GPLv3 requires that its owner can install a modified version. Secure Boot
therefore stays under the owner's control: the mini PC's firmware must let them
turn it off or enroll their own keys, and the box must never be locked to our
keys alone. The same holds for the later read-only A/B system ([design §6.7](design.md#67-later-a-read-only-system)).

## Models

| Model | Licence | Commercial use | What it asks of us |
| --- | --- | --- | --- |
| Gemma 4 E2B and E4B (the Myira model service; E2B by default), as GGUF files from `lmstudio-community` | Apache-2.0, Google's and the GGUF repackage's | Yes | Keep the licence and notice with the files |

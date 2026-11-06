# ZOOMRIYA OS: design

Design and implementation status · November 2, 2026

ZOOMRIYA OS is the system that runs the ZOOMRIYA Box. It is based on Debian 13
and is designed to stay out of the family's way: family members use Family
Hub, while administrators manage the box through [Box Desktop](box-desktop.md).
Box Desktop runs on the box's own screen and provides tools for storage,
updates, restore points and resets, without requiring a terminal for routine
administration.

ZOOMRIYA OS is not a separate Linux distribution or a fork of Debian. It is a
carefully configured Debian 13 system, with our settings package
(`zoomriya-base`), a small privileged helper (`zoomriya-agent`) and an image
build. The apps and box service are developed in the private `zoomriya-apps`
repository. The box sets up [ZOOMRIYA ID](https://github.com/zoomriya/zoomriya-id)
for sign-in, while [ZOOMRIYA Connect](https://github.com/zoomriya/zoomriya-connect)
provides remote access. This document describes the system beneath those
services.

**Contents**

1. [Decisions](#1-decisions)
2. [The layers](#2-the-layers)
3. [The disk](#3-the-disk)
4. [Boot, recovery and factory reset](#4-boot-recovery-and-factory-reset)
5. [First boot](#5-first-boot)
6. [Updates](#6-updates)
7. [zoomriya-agent](#7-zoomriya-agent)
8. [Hardening](#8-hardening)
9. [Names and versions](#9-names-and-versions)
10. [Building and testing](#10-building-and-testing)
11. [Layout](#11-layout)
12. [Open decisions](#12-open-decisions)
13. [Plan](#13-plan)

## 1. Decisions

| Decision | State |
| --- | --- |
| Debian 13 minimal is the base | Decided |
| systemd runs everything; apps are isolated with systemd-nspawn and unit sandboxing, as Photos' API is today | Decided |
| Every update can be rolled back, the OS's and the apps' | Decided |
| An update or upgrade of the OS must not affect the apps ([§2](#2-the-layers)) | Decided |
| Every app's image carries all its dependencies (its Python, `ffmpeg`, libraries, its own binaries such as Qdrant). Nothing on the host is there for an app, so a Debian fix on the host can never break an app ([§2](#2-the-layers)) | Decided |
| We fix the apps' dependencies with our own app releases, including security rebuilds; Debian's security fixes on the host are for the OS alone ([§2](#2-the-layers), [§6.3](#63-security-fixes-inside-app-images)) | Decided |
| SSH is off by default. An admin may turn on advanced access: key-only SSH as `admin`, from the home network only ([§7.1](#71-advanced-access)). Support gets in only through a support mode the admin turns on, for a limited time ([§7](#7-zoomriya-agent)) | Decided |
| The box's system controls are [Box Desktop](box-desktop.md), a desktop for admins on the box's own screen, in this public repository; using the box from elsewhere is Family Hub's and the apps' | Decided |
| Only components whose licences allow use in a product we sell; each is checked in [Licences](licences.md) before it goes into an image | Decided |
| Valkey (BSD-3-Clause, [Licences](licences.md)) is the shared key-value store | Leaning |
| Debian's security fixes for the OS install by themselves, so we don't release anything for them ([§6.1](#61-debian-security-fixes)) | Decided (OS-2) |
| The root filesystem stays writable (apt updates it) with Btrfs snapshots for rollback. A read-only A/B system is a later step, built from the same packages ([§6.7](#67-later-a-read-only-system)) | Decided (OS-1) |
| The OS version follows Debian's (ZOOMRIYA OS 13); each app keeps its own version ([§9](#9-names-and-versions)) | Leaning |
| The box is set up and runs over Ethernet; Wi-Fi stays off, shown turned off in Box Desktop on a box that has it ([§5.1](#51-wi-fi)) | Decided (OS-14) |
| One name per box, `<name>.zoomriya.app`, with a real certificate; `.local` names are for first boot only ([§5](#5-first-boot)) | Decided ([ZOOMRIYA Connect](https://github.com/zoomriya/zoomriya-connect/blob/main/docs/sharing-and-names.md)) |
| Hardening is our own list in `zoomriya-base`, checked in CI, with every difference from common advice written down ([§8](#8-hardening)) | Decided |

## 2. The layers

The system has four layers. Each has its own owner, update process and
rollback path, so changing one layer does not change or roll back another.

![The four layers: apps, shared services, OS and data, each with its owner, its update and its rollback](images/layers.svg)

Each app image contains everything that app needs to run: its Python
environment, `ffmpeg`, required libraries and any bundled programs, such as
Photos' Qdrant or the Myira model service's llama.cpp. From the host, an app
uses only the kernel, systemd, the network and the shared services' ports.
Keeping app dependencies out of the host means:

- **An OS update or upgrade can't change an app.** When the host takes a
  security fix, or moves from Debian 13 to 14, the images stay as they were.
- **Each kind of fix has one owner.** Debian's security fixes on the host
  patch the OS and the shared services, nothing else. The apps' dependencies
  are patched by our own app releases: a new release, or a security rebuild of
  the current one when Debian fixes a package inside an image ([§6.3](#63-security-fixes-inside-app-images)).
- **One copy of each dependency, where it's used.** `ffmpeg` is in Photos'
  image and nowhere else. If another app ever needs it, that app's image
  carries its own.

The same goes for ZOOMRIYA ID and our platform services in zoomriya-apps (the box
service, the Myira model service, backups, maps): each is an image. Backups'
image carries `restic` and `pg_dump`, the latter matching the host's PostgreSQL
major version.

**Current state.** Photos' API is the only app component currently running in
an image. The host still has some packages that will move into app images:

| Today on the host | Moves to |
| --- | --- |
| Photos' workers, in a venv on the host's `python3` | Photos' image, beside the API |
| `ffmpeg`, installed with `apt` for the workers | Photos' image (already there); removed from the host |
| Qdrant, a binary installed by `run_qdrant.sh` | Photos' image, as its own unit from the same image |
| ZOOMRIYA ID, the box service, the Myira model service, Vault, Chat, Tracker, Planner, Myira, in venvs under `/opt/zoomriya/<app>/venv` | One image each |
| `restic` and the maps tools | The backup and maps images |

Each app will follow the pattern used for Photos. Completing that migration is
necessary for this design.

**Shared services.** PostgreSQL, Valkey and Caddy remain on the host as Debian
packages. Apps connect to them over the network rather than linking against
them, so an update to a shared service does not change an app image. Debian's
security updates cover these services along with the OS. Their major versions
change only when the box moves to a new Debian release
([§6.6](#66-moving-to-debian-14)); PostgreSQL's database cluster is upgraded
as part of that move.

## 3. The disk

The box uses a fast SSD for the system and a separate drive for the family's
library. A future model may use two mirrored library drives (Solutions
Design, *Hardware and selling*).

**System SSD**

| Partition | Size | What |
| --- | --- | --- |
| EFI | 1 GB | Boot loader, kernels |
| recovery | 4 GB | A small ZOOMRIYA OS that can reinstall the system ([§4](#4-boot-recovery-and-factory-reset)) |
| system | the rest | Btrfs, with the subvolumes below |

**Subvolumes on the system partition**

**OS snapshots contain only the operating system.** App and family data live
in separate subvolumes, so restoring the OS does not roll back a photo,
message or database row.

| Subvolume | Mounted at | In OS snapshots | Notes |
| --- | --- | --- | --- |
| `@` | `/` | Yes | Debian, `zoomriya-base`, `zoomriya-agent`, `/etc` |
| `@machines` | `/var/lib/machines` | No | App images; the agent keeps the current and previous release of each |
| `@postgres` | `/var/lib/postgresql` | No | Every app's database. Snapshotted with a dump before an app migrates ([§6.2](#62-app-releases)) |
| `@valkey` | `/var/lib/valkey` | No | |
| `@data` | `/data` | No | `/data/zoomriya`, `/data/zoomriya-id`, `/data/models`, `/data/maps`, `/data/qdrant`, `/data/zoomriya-myira` |
| `@log` | `/var/log` | No | Kept across rollbacks, so the reason for a rollback is still there |
| `@snapshots` | `/.snapshots` | No | snapper's snapshots of `@` |

**A change still needed in the apps:** the box and maps services currently
store state in `/var/lib/zoomriya-box` and `/var/lib/zoomriya-maps`. Those
paths are on `@`, so an OS rollback would also roll back that state. The
services could move their data under `/data` or get separate subvolumes.
Moving it under `/data` is the simpler proposal and makes it available to
backups.

**Library drives.** One drive now, Btrfs, set up at the factory on the sold
box. Its scrub finds a damaged file from Btrfs's checksums, but with one copy
it can't repair it: the backups can. When a box has two library drives
(later), they're a Btrfs RAID 1 mirror, not `mdadm` (OS-4): the scrub then
repairs a bad copy from the good one, which `mdadm` can't tell apart. Photos' `storaged` keeps mounting drives
as today; it never formats them.

**Disk encryption is not planned** (OS-11), either on the system SSD or the
library drive. The box stays at home and starts without someone present.
Leaving the library drive unencrypted also means the family can still read
its photos on another computer if the box fails, without needing a recovery
key.

## 4. Boot, recovery and factory reset

**Secure Boot on.** Debian's signed shim, boot loader and kernel, so Secure
Boot works without our own keys. Custom signed images (UKIs) wait for the A/B
step ([§6.7](#67-later-a-read-only-system)).

**If a kernel fails to boot, the box falls back automatically.** Debian keeps
the previous kernel installed. With systemd-boot's boot counting, a new kernel gets three
tries; if it fails all three, the boot loader starts the previous one, and the
agent reports it in Box Desktop. `systemd-bless-boot` marks the new kernel good
once the box's health check passes (`zoomriya-boot-check`: `/` is `@` and
writable, `/data` is mounted, the firewall, the network, the agent and Box
Desktop are up); when it doesn't, the box restarts and the kernel uses up a
try. `panic=10` on the kernel command line makes a kernel that can't even find
its root restart too. Whether Debian 13's signed systemd-boot is usable here or
GRUB has to stay is OS-5.

**Recovery partition.** This partition holds a small ZOOMRIYA OS image that
is not updated in place. It starts if the main system still cannot boot after
using its available tries, or when someone requests a reset (below). It can
restore the system from a snapshot or reinstall it from the factory snapshot or
a USB stick. It does not touch
`@data`, `@postgres` or the library drives unless a full reset was asked for.

**Factory reset.** Two levels, both in Box Desktop's Control Panel for an
admin, and both available from the recovery partition when the admin can't
sign in:

| | What it does | What it keeps |
| --- | --- | --- |
| **Reset the system** | Restores `@` from the factory snapshot, then installs the latest security fixes; turns advanced access off and removes its keys ([§7.1](#71-advanced-access)) | Every app, account and photo |
| **Erase everything** | Also deletes the app images, the databases and `/data`, and wipes the library drives | Nothing. Asks twice, then needs the setup code from the sticker |

Both start from the factory restore point that the first start keeps. Erase
everything leaves a flag in the restored OS; at its first start, before
anything mounts them, `@data`, `@postgres`, `@valkey`, `@machines` and `@log`
are made new and empty and every other restore point is deleted. The setup
code stays on the EFI partition, so the box can be set up again.

A factory-reset button would be simplest, but an off-the-shelf mini PC has
none spare. How to reach the reset without signing in is OS-6.

## 5. First boot

Setup takes place at the box, using a screen and keyboard plugged into it.
Being there is how the owner confirms the box is theirs, so setup does not
happen over the network and does not depend on the apps or ZOOMRIYA Connect.

Before showing the setup screen, the box prepares sign-in automatically.
`zoomriya-services-init` makes PostgreSQL's cluster on `@postgres` (the image
carries none, so no two boxes share one), and tmpfiles.d gives Valkey its folders; then
`zoomriya-id-setup` runs ZOOMRIYA ID's `deploy/setup.sh`: its database role
and database, its settings in `/etc/zoomriya/id.env` and its folder, only
what's missing, with a mark written last. Both run again after Erase
everything or Reset the system, so the box comes back the same way. ZOOMRIYA
ID is in the image as the package `zoomriya-id`, built from its own
repository; it starts with no accounts.

Once those services are ready, `zoomriya-setup` runs the full-screen setup
app on tty1. It uses sway locked to a single GTK 4 app, with no sway keys or
bars, and renders in software so it can work without a graphics driver. The
setup continues until the box is configured:

1. **Welcome, and the language**, from Debian's languages (the `locales`
   package's list), each named in itself (Français, தமிழ்), with Noto fonts
   for every script. The agent's `language-set` adds the locale to
   `/etc/locale.gen`, builds only that one (`locale-gen`) and makes it the
   box's (`update-locale`). Once the region is chosen, the language becomes
   that region's own if there is one (Tamil in Sri Lanka: `ta_LK`), for its
   dates and numbers. The setup's own words are English until they're
   translated (gettext); Debian's programs follow the language already.
2. **The keyboard's layout** (the agent's `keyboard-set`), on the
   screen at once (`swaymsg`), so the admin's password is typed with it.
3. **The network.** The setup looks for an Ethernet port with a cable; without
   one it asks for the cable and goes on by itself once it's in. It shows the
   address the router gave (DHCP), the router and the name servers, and
   checks the internet. A fixed address is one tick away (`network-set`, kept
   with `network-keep`, undone by itself if not kept). Without the internet
   the setup carries on: the box catches up with updates and the clock later.
4. **The country or region**, chosen from tzdata's list with a search
   (`region-set`, kept in `/etc/zoomriya/region` as its two-letter code).
5. **The time zone**: the region's zones, or every zone with a tick, with a
   search (`time-zone-set`).
6. **The box's admin**: a username and a password, twice. The agent's
   `first-admin` makes it in ZOOMRIYA ID with ZOOMRIYA ID's own
   `app.first_admin`, run as ZOOMRIYA ID's account with the password on its
   standard input; the agent's audit log keeps the username only. There is no
   default account and no default password, and a second first admin is
   refused. The same username and password sign in to Box Desktop and to every
   ZOOMRIYA app added later.
7. **Done**: the language, the admin, the box's name on the home network,
   its address, its region and its time zone. Finish asks the agent's
   `setup-finish`, which needs an admin in ZOOMRIYA ID and writes `/var/lib/zoomriya-os/setup-done`; the screen
   goes to Box Desktop's sign-in, and `first-admin` and `setup-finish` are
   refused from then on.

The language, the region and the time zone are always asked: nothing is
chosen for the person, and no step's Next works until its fields are filled
in (the admin page says what's still missing).

Each confirmed step is saved as it happens. When setup starts, it asks the
agent for the current state (`setup-status`), so a power cut does not undo
completed steps. If an admin was already created, setup resumes at Done.
Setting up the box again requires Erase everything.

The setup app runs as its own account, `zoomriya-setup`, in `zoomriya-box`
(the agent's group), with a seat session for the screen, keyboard and mouse;
it can't sign in.

Added later, when they're on the box: the box's name `<name>.zoomriya.app` and
its certificate (ZOOMRIYA Connect); where the family lives (the box service)
and pairing the first phone (the apps). Remote use of the box is through
Family Hub and the apps.

**The setup code** is written at the factory to the EFI partition
(`zoomriya/setup-code`), matching the sticker; neither reset erases it there. A
box with none, a mini PC installed from the USB stick, makes one at its first
start and shows it on the screen. Erase everything asks for it. Avahi
announces the box as `zoomriya-<4 characters>.local` and `_zoomriya._tcp`.

### 5.1 Wi-Fi

The box uses the cable to the router, for setup and after it. **Wi-Fi stays
off** (OS-14).

- **Nothing turns it on.** The OS carries no Wi-Fi software (no `iwd`) and no
  Wi-Fi firmware, and its network settings match wired ports only, so a Wi-Fi
  chip in the box is never used. Bluetooth stays off too.
- **Box Desktop says so.** On a box with a Wi-Fi chip, Control Panel ›
  Network shows a Wi-Fi card, turned off and greyed out: "The box connects to
  the router by a cable." On a box without one, there's no Wi-Fi card at all.
  The agent's `network` already tells Ethernet and Wi-Fi apart.
- **Why the cable:** it's steadier for phones backing up at home and for the
  library, and ZOOMRIYA Connect needs it anyway: a steady address for port
  forwards, reliable mappings, and no Wi-Fi client isolation
  ([ZOOMRIYA Connect](https://github.com/zoomriya/zoomriya-connect/blob/main/docs/where-it-works.md)).

### 5.2 A fixed address

Each Ethernet port takes its address by DHCP, the default and what most homes
need. An admin can give a port a fixed address instead, in Control Panel ›
Network: the address, the subnet mask, the router and one to three name
servers. Most people never need it; it's for homes whose router can't reserve
an address for the box.

- **A wrong address can't shut the admin out.** Every change undoes itself
  after two minutes unless an admin presses Keep these settings, on the box's
  screen, where Box Desktop shows the new address. A second change before the first is kept goes back, if not
  kept, to the settings before both.
- **Checked before it's written:** IPv4, a mask from /8 to /30, not the
  network's or broadcast address, the router on the same network. The address
  is probed on the network before it's used (IPv4 duplicate address
  detection).
- **Where it lives:** `/etc/systemd/network/50-zoomriya-<port>.network`,
  which comes before the default DHCP file; going back to DHCP removes it. It's
  on `@`, so a restore point from before the change, and Reset the system, go
  back to DHCP.

## 6. Updates

| What | Who installs it | When | Rollback |
| --- | --- | --- | --- |
| Debian security fixes (host) | unattended-upgrades | By themselves, every day | snapper snapshot of `@` |
| Kernel | unattended-upgrades | Installed by itself; takes effect at the next restart, in the maintenance hour | Previous kernel, by boot counting |
| ZOOMRIYA OS's own packages (`zoomriya-base`, `zoomriya-agent`, `box-desktop-app`) | zoomriya-agent | Only when an admin presses Update; never checked or installed by themselves ([§6.8](#68-our-apt-repository)) | snapper snapshot of `@` |
| App release | zoomriya-agent | When an admin presses Update | The previous image, plus the database dump if the schema changed |
| App image security rebuild (same app code, newer Debian packages) | zoomriya-agent | By itself, in the maintenance hour | The previous image |
| Models, street maps | The apps, as today | When an admin asks | As today |
| Debian 13 → 14 | zoomriya-agent | Offered by us, accepted by an admin | snapper snapshot of `@` |

### 6.1 Debian security fixes

- **Only the security archive installs by itself.** Point releases and anything
  else wait for an image or a box upgrade we test first.

  ```
  // /etc/apt/apt.conf.d/52zoomriya-unattended
  Unattended-Upgrade::Origins-Pattern {
    "origin=Debian,codename=${distro_codename}-security,label=Debian-Security";
  };
  Unattended-Upgrade::MinimalSteps "true";
  Unattended-Upgrade::Automatic-Reboot "false";   // the agent restarts in the maintenance hour
  ```

- **Sources name Debian 13's codename, never `stable`.** With `stable`, every
  box would move itself to Debian 14 the day it's released.

  ```
  # /etc/apt/sources.list.d/debian.sources   (<codename>: Debian 13's)
  Suites: <codename> <codename>-updates <codename>-security
  ```

- **A snapshot before and after every apt run**, through snapper's apt hook.
  Box Desktop lists them as restore points with their date. The first start
  also keeps one named `factory`, which is never cleaned up.
- **Restoring one** makes a new `@` from it; the box runs on it from the next
  start, which removes the old `@`. A restore point of the OS as it was is
  taken first, so a restore can be undone too. Boot entries for kernels the
  restored OS doesn't have are removed, so the box never starts a kernel
  without its modules.
- **A restore holds the bad update back.** Restoring to a point before an
  update would otherwise only last until the next run, which installs the
  same fix again. So the agent compares the packages in the restored point
  with the ones it replaces, and holds each package the restore took back at
  its restored version (`apt-mark hold`). Each night it lets a hold go once
  Debian's security archive has a newer version than the one that was taken
  back, so a corrected fix still installs by itself (`zoomriya-agent
  release-holds`, just before the security fixes install). Box Desktop shows
  the held packages on Restore & Reset ("Updates on Hold") with Update now
  for each: it lifts the hold and installs the security fixes at once (Check
  now's job). The admin action is in the audit log. Holds
  are kept on `@`, so restoring further back or Reset the system clears them;
  a reset holds nothing.
- **An interrupted update fixes itself.** If power was cut during an update,
  `zoomriya-base` runs `dpkg --configure -a` at the next boot, before anything
  else updates.
- **The apps aren't restarted for it.** `needrestart` would restart services
  that use an updated library; the app units are excluded, since their images
  don't use the host's libraries.
- **Check now, never off.** An admin can start the same job at once with
  Check now on Control Panel › Security, for news of a serious hole or a box
  that was off overnight (`security-check-now`, `zoomriya-security-now.service`:
  the package lists from Debian's sources, the holds let go, then
  unattended-upgrades with the same settings and restore points). There's no
  switch to turn the security fixes off; only strict offline mode stops them.
- **Restarts happen in the maintenance hour.** When a kernel or `libc` fix needs
  one, Box Desktop says "The box will restart at 3:30 tonight to finish a
  security update", with Restart now and a different hour. The maintenance
  hour is a box-wide setting (Photos' own maintenance hour stays Photos'),
  03:30 unless an admin moves it: `zoomriya-maintenance.timer` runs then and
  restarts the box only when an update or a restore waits for it.
- **On the Network activity page.** Fetching updates from `deb.debian.org` and
  `security.debian.org` is an outbound connection, shown there with its switch
  like the others. Strict offline mode stops it, and System Status then says
  the box isn't getting security fixes.
- **Optional, later: our own mirror** of the security archive (`aptly`), so we
  can hold a bad Debian update back from every box at once. Without it, a bad
  update reaches every box the same day.

This is OS-2, option (c). Everything else installs only when an admin says
so; Debian's security fixes on the host are the exception.

### 6.2 App releases

When an administrator presses **Update**, the agent follows these steps.
Whether updates can also be scheduled is still an open decision (OS-12):

1. **Download** the release: one image per app and a signed manifest listing
   each image's digest. **Check the signature** against the public key in
   `zoomriya-base` and every digest. Nothing unsigned is imported.
2. **Check it fits:** the manifest's minimum ZOOMRIYA OS version, free space,
   and that the library drives are healthy.
3. **Back up the databases the release migrates** (`pg_dump` per app database
   into `@postgres`), because migrations can't always be undone.
4. **Import** the image (`machinectl import-tar`), next to the running one.
5. **Migrate:** `zoomriya-<app>-migrate@<new>`.
6. **Start the new release**, check its health (`/app/status`), then stop the
   old one and record the new one as active. This is how Photos' releases are
   promoted today.
7. **If the health check fails:** stop the new release, restore the database
   dump, start the old release. Box Desktop says "The update didn't work. Your
   box is back on 1.4.2", and the agent keeps the logs for support.
8. **Keep the previous release** of each app for **Restore previous version**,
   which runs steps 6 and 7 the other way.

### 6.3 Security fixes inside app images

An app image keeps the Debian packages it was built with, so it does not
receive the host's security fixes ([§6.1](#61-debian-security-fixes)). This is
particularly important for Photos' `ffmpeg`, which reads every video and
photo a family uploads. The plan has four parts: keep images small, track
available fixes, rebuild affected images, and install those rebuilds like
host security updates.

**1. Keep images small.** Every Debian package in an image is one more reason
to rebuild it. Images list only what their native code needs, as Photos'
already does (from its libraries' `DT_NEEDED` entries). Once Photos' workers
are in its image, `ffmpeg` comes off the host, so there's one copy to keep
current, not two.

**2. Notice fixes.** Every image build writes a package manifest (mkosi's
`ManifestFormat=json`) listing each Debian package and version in it, kept with
the release. A scheduled CI job, every 6 hours, compares each published
image's manifest with Debian 13's security archive and the Debian security
tracker's data.
Any package with a newer fixed version marks that image for a rebuild. The same
job runs `pip-audit` over each image's Python wheels (Pillow, onnxruntime,
OpenCV), since those carry their own copies of libraries Debian doesn't patch
for us.

**3. Rebuild automatically.** CI rebuilds the marked image from the
same git tag as the release it replaces: same app code, newer Debian packages.
It runs the app's tests and the image checks (`verify_image.sh`), signs it and
publishes it as a **security rebuild** of the same release, with a revision
number: Photos `1.4.2+r2` is 1.4.2's code with its second set of Debian fixes.
Box Desktop shows it as "Photos 1.4.2 (security update 2)". Target: published
within a day of Debian's fix.

**4. Install like the host's.** A security rebuild changes no app code, so it
carries the same risk as the host's own security fixes. The agent installs it
by itself in the maintenance hour, through [§6.2](#62-app-releases)'s steps: check the signature,
import, start, health check, roll back if it fails. No migrations run, because
the code is the same. The admin sees it afterwards in the update history, not
as a question. A release with new app code still waits for the Update button.
This is OS-3, option (c): the same exception as the host's security fixes in [§6.1](#61-debian-security-fixes).

**Limit the impact while a fix is on its way.** Fixes may take a day or two to
reach a box, so upload processing runs with limited access: inside the image,
as its own user, with `ProtectSystem=strict` and only
`/data/zoomriya` writable (Photos' units today). Running each `ffmpeg` call in a
transient unit with no network (`IPAddressDeny=any`), a system call filter and
a time and memory limit narrows it further; that's for zoomriya-apps to add.

### 6.4 Shared services

PostgreSQL, Valkey and Caddy get their security fixes through [§6.1](#61-debian-security-fixes). Each fix
restarts that one service (needrestart), which the apps already survive: they
reconnect to PostgreSQL and Valkey by themselves.

### 6.5 Models and maps

Models under `/data/models` and street maps under `/data/maps` continue to
update when an administrator asks, independently of app releases.

### 6.6 Moving to Debian 14

Debian releases about every two years. Debian 14 is expected around mid 2027;
Debian 13's security support continues about a year after that, then its LTS
until about mid 2030. Nothing moves by itself, because the sources name Debian
13's codename.

1. **When Debian 14 is released:** CI starts building and testing the host on it
   next to 13 ([§10](#10-building-and-testing)). The app images don't change.
2. **After its first point releases:** offer the upgrade in Box Desktop, to our
   own boxes first, then a few families, then everyone. "ZOOMRIYA OS 14 is
   ready. It takes about 20 minutes and keeps all your data."
3. **The upgrade:** check space and drive health; a snapper snapshot of `@`; a
   dump of every database; switch the sources to Debian 14's codename;
   `apt full-upgrade` with `--force-confdef --force-confold`, so no question
   stops it; `pg_upgradecluster` to the new PostgreSQL; restart; health check.
   If anything fails, roll back to the snapshot and the dumps, and the box is on
   13 as before.
4. **Before 13's LTS ends,** the upgrade installs on any box still on 13, with
   notice to the admin first.

### 6.7 Later: a read-only system

The root filesystem stays writable so Debian can install security fixes
automatically. In practice, `ProtectSystem=strict` already makes `/usr`,
`/etc` and `/boot` read-only to services. Since interactive logins are
disabled, only apt and the agent can change the system.

Once CI can rebuild, test and publish a whole system image whenever Debian
publishes a fix, the box can move to two read-only system partitions, A and B,
checked by dm-verity: the update writes the other one and the boot loader falls
back if it doesn't come up. Because everything we add to Debian is already a package
(`zoomriya-base`, `zoomriya-agent`), mkosi builds those images from the same
packages, and nothing changes for families.

### 6.8 Our apt repository

Our packages (`zoomriya-base`, `zoomriya-agent` and `box-desktop-app`) will
come from our signed apt repository at **`apt.zoomriya.com`** (OS-8). The box
will contact this repository only under the following conditions, so updates
do not become a way for the box to report home:

- **Only when an admin starts it.** The box never checks for our updates or
  installs them by itself: not at night, not with the security fixes. Its
  source is kept outside `sources.list.d`
  (`/etc/apt/zoomriya-updates/zoomriya.sources`), so apt and the nightly
  security fixes never read it; when an admin presses Update in Box Desktop,
  the agent checks and installs with that source added for that update
  alone, after a restore point. It's turned off (`Enabled: no`) until the
  repository and its signing key exist.

- **A name of our own,** never a hosting company's address, so the hosting can
  move (a CDN, another server) without changing a single box.
- **Plain signed files over HTTPS,** fetched by apt like Debian's. The box
  sends nothing that names it: no box id, serial, setup code, account or
  custom header, and no cookies. Our server sees only that an address
  downloaded the package list.
- **No logs kept** on our side, or only counts without addresses, and we say so
  on zoomriya.com.
- **Shown in Box Desktop** on Control Panel › Outbound connections, as
  "ZOOMRIYA updates: the box's own software, only when an admin presses
  Update". Strict offline mode turns it off.
- **Updates from a USB stick** for a box with it turned off: the same signed
  packages, downloaded from zoomriya.com on another computer, checked against
  the same key before they're installed.

## 7. zoomriya-agent

`zoomriya-agent` is the only helper the OS adds that runs as root. It is a
socket-activated Go service, like Photos' `storaged`, built as a static binary
using only the standard library. Its clients are the first-boot setup and Box
Desktop after an administrator signs in
([Box Desktop §6](box-desktop.md#6-how-it-works)). Only root and members of
the `zoomriya-box` group can access it; Family Hub and the apps do not call
it. The agent accepts a fixed set of requests with defined arguments, not
arbitrary commands:

| Request | What it does |
| --- | --- |
| `status` | OS version, last security update, a restart needed and when, recent restore points |
| `disk-health` | SMART for every drive, as "Healthy", "Check soon" or "Replace now" with the reason |
| `apps` | Each app's active and previous release |
| `app-update`, `app-restore` | [§6.2](#62-app-releases) |
| `os-update` | ZOOMRIYA OS's own packages from `apt.zoomriya.com`, only when an admin presses Update ([§6.8](#68-our-apt-repository)) |
| `os-upgrade` | [§6.6](#66-moving-to-debian-14) |
| `restart`, `maintenance-hour` | Restart now, or set the hour |
| `security-check-now` | The nightly security fixes, now ([§6.1](#61-debian-security-fixes)) |
| `restore-point-list`, `restore-point-restore` | snapper snapshots of `@` |
| `held`, `held-release` | The packages a restore held back, and letting one update again ([§6.1](#61-debian-security-fixes)) |
| `support-mode` | Turn on key-only SSH for support's key, for at most 24 hours, then off by itself |
| `factory-reset` | [§4](#4-boot-recovery-and-factory-reset) |
| `diagnostics` | One bundle: the journal, versions, disk health, each app's diagnostics; no family data |
| `storage`, `storage-check` | The library's space by what uses it, the mirror's state when there are two drives, a scrub now (for Box Desktop) |
| `resources` | CPU, memory, network, drive activity and temperatures, for the box and per app from its units' cgroups |
| `network`, `security`, `time` | The box's network, its outbound connections, the firewall and security checks (read only), the clock |
| `network-set`, `network-keep` | A port's fixed address or DHCP, undone after two minutes unless kept ([§5.2](#52-a-fixed-address)) |
| `notifications` | The last month's notifications from the watcher (`zoomriya-agent watch`, on a timer): drives, updates, restarts, the library's check, network settings undone |
| `keyboard`, `keyboard-set` | The layout of a keyboard plugged into the box, in Debian's `/etc/default/keyboard`, from a fixed list |
| `logs` | Box events, the audit log and admins' sign-ins, filtered, with no family content, a hundred entries at a time, newest first |
| `shutdown` | Shut the box down |
| `advanced-access` | Turn advanced access on or off; list, add and remove its keys ([§7.1](#71-advanced-access)) |

- Its socket is `0660`, group `zoomriya-box`.
- It runs the long jobs (updates, upgrades, resets) as separate units, such as
  `zoomriya-agent-update@<release>.service`, so they finish even when Box
  Desktop or the gateway restarts during them.
- Every request goes into the admin's audit log, with who asked.
- Updates, restarts and support mode are shown in Box Desktop's background
  jobs and Log Center while they run and after.

### 7.1 Advanced access

SSH for the box's admins, for those who want a command line, such as someone
running ZOOMRIYA OS on their own mini PC. Off by default.

- **An admin turns it on** in Box Desktop, Control Panel › Security › Advanced
  access, and enters their password again. It stays on until turned off, and
  Box Desktop shows "Advanced access is on" while it is.
- **Keys only.** No password logins and no root login. You sign in as `admin`,
  which has `sudo`.
- **From the home network only:** port 22 opens on the home interfaces
  (Ethernet) while it's on, and never through ZOOMRIYA
  Connect or from the internet.
- **Adding a key, from a USB stick:** put the public key in a folder
  `zoomriya/ssh/` on any stick (`zoomriya/ssh/anna-laptop.pub`) and plug it into
  the box. The box mounts it read-only (`nosuid,nodev,noexec`), reads only that
  folder and runs nothing from it. Box Desktop then shows "A USB stick has 1 SSH
  key: anna-laptop, SHA256:…" with **Add**, which asks for the password again
  (and offers to turn advanced access on, if it's off). Plugging in a stick
  alone never adds a key, so someone who can only reach the box can't let
  themselves in.
- **Or by pasting it** in the same section of Box Desktop, or uploading the
  `.pub` file, with a name. Either way the agent checks it's a valid public key
  (Ed25519, ECDSA, RSA of 3072 bits or more, or a hardware security key's
  `-sk` key) before adding it. `ssh-copy-id` doesn't work, because it needs a
  password login first.
- **Checking it's your box:** the section shows the box's host key fingerprint
  and the command, `ssh admin@<name>.zoomriya.app`, to compare on the first
  connection.
- **The key list** shows each key's name, fingerprint, when it was added and
  when it was last used, with Remove.
- **Kept outside the OS snapshot,** in `/data/zoomriya-os/ssh/authorized_keys`
  (sshd's `AuthorizedKeysFile`), so restoring the OS doesn't lock anyone out.
  Reset the system and Erase everything remove the keys and turn it off
  ([§4](#4-boot-recovery-and-factory-reset)).
- **Every change is audited:** turning it on or off, keys added and removed,
  each sign-in and each `sudo`.
- **What you change by hand is yours.** Box Desktop says so when turning it on:
  updates and restore points may undo hand changes, and support may ask to
  reset what was changed. The diagnostics bundle notes that advanced access was
  used, and when.
- **Support mode stays separate:** support's own key, turned on by an admin,
  off by itself within 24 hours.

## 8. Hardening

The hardening settings below live in `zoomriya-base` and are checked by our
tests on every build. When the design differs from common hardening advice,
the reason and the alternative measure are recorded in the final table.

**Already done by this design**

| What | Here |
| --- | --- |
| Separate `/tmp`, `/var`, `/var/log` with `nodev,nosuid,noexec` where possible | Subvolumes ([§3](#3-the-disk)); `/tmp` as tmpfs |
| SSH hardened | Off by default; advanced access and support mode are key-only, with no root or password login, and reachable from the home network only ([§7.1](#71-advanced-access)) |
| No unneeded services or packages | A minimal mkosi build installs only what's listed |
| Patches applied | [§6.1](#61-debian-security-fixes) |
| Mandatory access control | AppArmor (on by default in Debian), with profiles for the gateway, connectd and each app's units |
| Logs kept and capped | journald, persistent, 1 GB cap (already in Photos' `zoomriya-journald.conf`) |

**Done, and checked on every build** (`tests/hardening/check.sh`, run in the
VM test)

- **Firewall:** nftables, inbound denied except 443 (connectd); on the home
  network (Ethernet) mDNS, and SSH on 22 while advanced
  access or support mode is on ([§7.1](#71-advanced-access)); and DHCP. The
  first setup is on the box's own screen and listens on no port. connectd's short-block set (ZOOMRIYA
  Connect) lives in the same table. Checked: the rules, and that nothing else
  listens.
- **Always on.** The box never sleeps, hibernates or saves power by switching
  itself off: `zoomriya-base` turns sleep off (`sleep.conf.d`), has sleep keys,
  a lid and idleness do nothing (`logind.conf.d`), and masks the sleep targets
  so nothing can start them. Only an admin's Shut down or the power button stop
  it. Box Desktop shows it on Restarts › Power, on and greyed out (the agent's
  `status` reports `always_on`).
- **No opening ports by hand.** Box Desktop has no place to type a port. The
  OS opens what's above; later, each app declares the ports it needs, and the
  agent opens them, on the home network only, when the app is installed and
  closes them when it's removed. Box Desktop shows what's open in plain words
  (Security › Allowed Network Connections), read only. Someone who wants
  more uses advanced access ([§7.1](#71-advanced-access)), on their own.
- **sysctl:** no forwarding, `rp_filter`, no ICMP redirects, full ASLR, no
  setuid core dumps.
- **auditd:** changes to accounts, time and sudo, kernel modules loaded, and
  the agent's actions (the agent, restores, the maintenance hour); auditd's
  own cap on its log (5 files of 8 MB).
- **Unused kernel modules off:** `cramfs`, `freevxfs`, `hfs`, `hfsplus`, `udf`,
  `dccp`, `sctp`, `rds`, `tipc`. Checked: none of them loads.
- **Time:** systemd-timesyncd. Certificates and update signatures depend on the
  clock.

- **File integrity:** AIDE nightly over `/usr` and `/etc` against a baseline
  from the first start, refreshed after every apt run so it reports only what
  changed outside an update, into the journal for Log Center. Its database is
  on `@`, so it rolls back with the OS. Checked: a file dropped into `/etc` is
  reported.

**Still to add**

- **AppArmor profiles** for the gateway, connectd and each app's units, with
  them.

**Where we differ from common advice**, each with what stands in for it:

| Common advice | Why not | Instead |
| --- | --- | --- |
| No mDNS (Avahi) | First boot and the phone app find the box with it | Allowed on the home network interfaces only |
| Wireless interfaces off | None: the box uses the cable ([§5.1](#51-wi-fi)) | No Wi-Fi software or firmware, no Wi-Fi network settings; Bluetooth off |
| A boot loader password | Nobody would ever type it | Secure Boot; the boot menu's editor off |
| PAM password rules, lockout, password ageing | Families never log into Linux | ZOOMRIYA ID's rules: Argon2id, growing delays per account |
| USB storage off | Backups and the reinstall stick use it | Plugged-in drives mounted `nosuid,nodev,noexec` by `storaged` |

**CI checks** run in a VM for every image build. Each item in this section has
a check under `tests/hardening/` that verifies both the configuration and the
running system; a missing check or setting fails the build. Lynis is also used
to catch hardening issues outside our own checklist. We will share the
checklist and its documented exceptions with anyone who asks.

**Regulatory requirements.** The EU Cyber Resilience Act (vulnerability reporting from
September 2026, the rest from December 2027) and the UK's PSTI Act ask for no
default passwords ([§5](#5-first-boot)), security updates for a stated support period ([§6](#6-updates)), a
way to report vulnerabilities, and, for the CRA, a software bill of materials
([§10](#10-building-and-testing)). Canada and the US, where the first boxes go, have no equivalent law yet;
ETSI EN 303 645 is the checklist both laws borrow from.

## 9. Names and versions

**Versions**

| What | Version | Example | Changes when |
| --- | --- | --- | --- |
| ZOOMRIYA OS | Debian's major version | 13 | The box moves to the next Debian |
| Debian point release | Debian's | 13.2 | Debian, by itself |
| `zoomriya-base` | Debian major, then ours | 13.0.3 | We change a setting |
| Each app | Its own, semver | Photos 1.4.2 | Its release |
| Factory image | Build date | 2026.11.2 | We make a new installer |

An app release's manifest names the oldest ZOOMRIYA OS it runs on ("Photos 1.x
runs on ZOOMRIYA OS 13 and later"); the agent checks it before importing.

**`/usr/lib/os-release`**, replaced with `dpkg-divert` in `zoomriya-base` so
Debian's `base-files` updates don't put Debian's back:

```ini
NAME="ZOOMRIYA OS"
PRETTY_NAME="ZOOMRIYA OS 13"
ID=zoomriya
ID_LIKE=debian
VERSION="13"
VERSION_ID=13
VERSION_CODENAME=<codename>     # Debian 13's
IMAGE_ID=zoomriya
IMAGE_VERSION=2026.11.2
HOME_URL="https://zoomriya.com"
SUPPORT_URL="https://zoomriya.com/support"
```

`ID_LIKE=debian` and `VERSION_CODENAME` keep Debian's tools working;
`/etc/debian_version` stays as Debian ships it.

**Where the name shows**

| Where | What |
| --- | --- |
| Box Desktop, Control Panel › System Info | System OS: "ZOOMRIYA OS 13", and under Details: "System release 13.2", `zoomriya-base`, the factory image, the serial |
| Boot menu | `PRETTY_NAME` |
| The router's device list | The DHCP host name, `zoomriya-<4 characters>` |
| `/etc/issue`, `/etc/issue.net` | A warning banner, with the name |

**Debian's name and the GPL.** We don't call the product Debian or use Debian's
logo, and the screens don't name Debian at all: they say "the OS" ("the
OS's security fixes"), though a factual "based on Debian" would be allowed. Shipping a box with GPL software in it means
offering its source: each release publishes the source packages of what it
contains, alongside the software bill of materials, both from the same build.

## 10. Building and testing

The following images and packages are built with mkosi on Debian 13:

| Output | For |
| --- | --- |
| Disk image | Written to the system SSD at the factory |
| Installer (USB) | Bring-your-own mini PC, and reinstalling |
| Recovery image | The recovery partition |
| `zoomriya-base`, `zoomriya-agent`, `box-desktop-app` (`.deb`) | Our own signed apt repository, `apt.zoomriya.com` ([§6.8](#68-our-apt-repository)) |
| App images | Built in zoomriya-apps, as Photos' are today |

**The USB installer** (`mkosi/installer`, `zoomriya-installer`) is for
installing on a personal computer or reinstalling the system. The USB stick
contains a small read-only system and the disk image, and presents a
full-screen installer with five steps. Welcome checks for UEFI, at least 4 GB
of memory and a drive of 64 GB or more. It also checks for internet access,
but installation can continue without it because the image is on the stick;
checking for security fixes during installation still needs to be designed.
Drive lists the available disks and explains why the installer cannot use
the stick itself or a disk that is too small. Summary asks the user to
confirm which disk will be erased. Install copies the image while a slideshow
introduces the box. Done offers a restart or shutdown, so the owner can set
up the box later.

The install is two passes of `systemd-repart`: the image's partitions copied
as they are (`--copy-from`), then the system partition grown to the end of the
drive with the same definitions as the factory's, then its Btrfs grown to
fill it and a boot entry added (`efibootmgr`; the firmware's fallback path
works without it). The first setup then runs at the box's next start. Tested
in a VM: `scripts/run-installer-vm.sh`.

**Security rebuilds of app images** ([§6.3](#63-security-fixes-inside-app-images)):
Every six hours, a CI job compares each published image's package manifest
with Debian 13's security archive and runs `pip-audit` on its Python wheels.
If it finds a fix waiting, CI rebuilds, tests, signs and publishes the image.

**Continuous integration** runs for every change and whenever Debian
publishes a security update:

```
build ─► boot in a VM ─► first boot ─► app update and rollback ─► security
update and rollback ─► kernel that doesn't boot falls back ─► hardening checks ─►
sign ─► publish
```

As soon as Debian 14 and Debian testing are available, CI will also test the
same app images on those releases. This should reveal compatibility problems
well before families need to upgrade.

Every build records its software bill of materials and the source packages it
used.

## 11. Layout

The host image, `zoomriya-base`, `zoomriya-agent`, `box-desktop-app`,
`scripts/` and `tests/vm/` are in place as part of plan steps 1 and 4 and the
start of step 3. The other parts shown below are still planned.

```
docs/              this design
scripts/           setup, build, and boot or test in a VM
mkosi/
  host/            the system image (partitions, subvolumes, packages)
  installer/       the USB installer (zoomriya-installer, with the image)
  recovery/        the recovery image
packages/
  zoomriya-base/   os-release, apt sources and unattended-upgrades, snapper,
                   nftables, sysctl, auditd, AppArmor, journald, Avahi, banners,
                   the release signing key
  zoomriya-agent/  the root helper (Go, in src/) and its units
  box-desktop-app/ Box Desktop on the box's screen (GTK 4 in Python) ([Box Desktop](box-desktop.md))
tests/             VM tests: first boot, updates, rollback, fallback
  vm/              the checks run inside the booted image
  hardening/       one check per item in the hardening list (§8)
```

## 12. Open decisions

| # | Decision | Options | State |
| --- | --- | --- | --- |
| OS-1 | Writable root now, read-only A/B later ([§6.7](#67-later-a-read-only-system)) | (a) writable with snapshots, (b) A/B from the first box | Decided (a) |
| OS-2 | Debian updates on the host | (a) all install by themselves, (b) all wait for the admin, (c) security fixes by themselves, the rest wait | Decided (c), with a restore holding the bad update back ([§6.1](#61-debian-security-fixes)) |
| OS-3 | App image rebuilds for Debian security fixes | (a) wait for the admin's Update like any release, (b) install by themselves like the host's, (c) by themselves only when the app's code is unchanged | Decided (c), [§6.3](#63-security-fixes-inside-app-images) |
| OS-4 | The library mirror, when a box has two library drives (later; one for now) | (a) Btrfs RAID 1, (b) `mdadm` RAID 1 with ext4 | Decided (a) |
| OS-5 | Boot loader | (a) systemd-boot with boot counting, if Debian 13's signed build works with shim on the box, (b) GRUB with a fallback entry | Leaning (a); to test on the box |
| OS-6 | Reaching factory reset without signing in | (a) a USB stick with a reset file, (b) the recovery partition, chosen by holding a key at power-on, which needs a keyboard, (c) both | Leaning (c) |
| OS-7 | Our own mirror of Debian's security archive ([§6.1](#61-debian-security-fixes)) | (a) not at first, (b) from the first box | Decided (a) |
| OS-8 | Where releases and our apt repository are hosted | `zoomriya.net` (control plane and guest page today), `zoomriya.com` or GitHub | Decided for releases: the installer is downloaded from `zoomriya.com`, and GitHub gets only a tag with the source and release notes linking there. Our apt repository is `apt.zoomriya.com`, under the rules in [§6.8](#68-our-apt-repository) |
| OS-9 | This repository's licence | MPL-2.0 like ZOOMRIYA Connect, with `TRADEMARKS.md` | Decided |
| OS-10 | Box and maps state moving from `/var/lib` to `/data` ([§3](#3-the-disk)) | Move, or a subvolume each | Leaning move |
| OS-11 | Disk encryption ([§3](#3-the-disk)) | (a) LUKS by default, unlocked by the TPM with Secure Boot, a recovery key shown once at setup, (b) optional, (c) LUKS with a passphrase at every boot, (d) none | Decided (d): no encryption |
| OS-12 | Scheduling app updates ([§6.2](#62-app-releases)) | (a) only when an admin presses Update, (b) also on a schedule the admin chooses | Open |
| OS-13 | Box Desktop's own decisions: Family Hub's Update button, notifications, where each admin's settings live | [Box Desktop §10](box-desktop.md#10-open-decisions) | Open |
| OS-14 | Wi-Fi ([§5.1](#51-wi-fi)) | (a) the cable only: Wi-Fi stays off, (b) joined in Box Desktop after setup, (c) also a setup hotspot | Decided (a): Wi-Fi shown turned off on a box that has it, not shown on one that doesn't |

## 13. Plan

1. **Build the host image.** Create the mkosi disk image with its partitions
   and subvolumes, and configure `zoomriya-base` with the OS name, package
   sources, firewall, sysctl and journald settings. Boot it in a VM and on
   the box.
2. **Move every app into an image.** Finish moving the apps and Photos'
   workers into `systemd-nspawn` images in `zoomriya-apps`, and move the box
   and maps services' state under `/data`.
3. **Build out `zoomriya-agent`.** Add its status, restore-point, restart,
   maintenance-hour, disk-health, reset, storage, network and resource
   requests. The maintenance-hour restart is implemented and tested in a VM.
   Then add OS updates, app listing and app updates and restores, and finish
   Box Desktop's first tools ([Box Desktop §11](box-desktop.md#11-plan)).
4. **Complete security updates.** Automatic updates, snapper, restore points,
   boot counting, kernel health checks and the maintenance-hour restart have
   been tested in a VM, including a failed kernel boot. Restores now hold
   back the update they undo ([§6.1](#61-debian-security-fixes)); a VM test
   with an actual held package is still needed. The CI job to rebuild app
   images for Debian security fixes is also still to come
   ([§6.3](#63-security-fixes-inside-app-images)).
5. **Finish first boot.** The mDNS name, setup code and on-screen setup with
   ZOOMRIYA ID are in place. Add the ZOOMRIYA Connect name and certificate
   when that service is integrated.
6. **Complete recovery and reset.** The reset options are available from Box
   Desktop and have been tested in a VM. The recovery partition image and USB
   reset path remain to be built.
7. **Finish hardening.** Most settings and checks are in place. Add the
   AppArmor profiles for the gateway, connectd and app services, then report
   the Lynis hardening index for each run (67 on the first run).
8. **Test on the box itself.** Complete a clean install, a restore drill, and
   an update-and-rollback test.
9. **Later work.** Add our own security mirror and the read-only A/B system,
   then move to Debian 14.

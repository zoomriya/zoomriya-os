# ZOOMRIYA OS: design

November 2, 2026 · design only, nothing built yet

ZOOMRIYA OS is the system under the ZOOMRIYA Box: Debian 13, set up
so that a family never sees it. The family sees Family Hub. The box's admins
also get [Box Desktop](box-desktop.md), ZOOMRIYA OS's own desktop on the
box's screen, for storage, updates, restore points and reset, with an **Update** button and a
**Restore previous version** button, and nothing that needs a terminal.

It is not a new distribution and not a fork of Debian. It is Debian 13
installed a fixed way, plus one package of our own settings (`zoomriya-base`),
one small root helper (`zoomriya-agent`) and an image build. The apps and the
box service live in zoomriya-apps (private); sign-in is
[ZOOMRIYA ID](https://github.com/zoomriya/zoomriya-id), which the box sets up; remote access lives in
[zoomriya-connect](https://github.com/zoomriya/zoomriya-connect). This document covers
what sits underneath them.

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
| Debian's security fixes for the OS install by themselves, so we don't release anything for them ([§6.1](#61-debian-security-fixes)) | Leaning (OS-2) |
| The root filesystem stays writable (apt updates it) with Btrfs snapshots for rollback. A read-only A/B system is a later step, built from the same packages ([§6.7](#67-later-a-read-only-system)) | Leaning |
| The OS version follows Debian's (ZOOMRIYA OS 13); each app keeps its own version ([§9](#9-names-and-versions)) | Leaning |
| The box is set up over Ethernet and may also join the home Wi-Fi; ZOOMRIYA Connect (reaching the box away from home, and guest links) needs Ethernet ([§5.1](#51-wi-fi)) | Decided ([ZOOMRIYA Connect](https://github.com/zoomriya/zoomriya-connect)) |
| One name per box, `<name>.zoomriya.app`, with a real certificate; `.local` names are for first boot only ([§5](#5-first-boot)) | Decided ([ZOOMRIYA Connect](https://github.com/zoomriya/zoomriya-connect/blob/main/docs/sharing-and-names.md)) |
| Hardening is our own list in `zoomriya-base`, checked in CI, with every difference from common advice written down ([§8](#8-hardening)) | Decided |

## 2. The layers

Four layers, each with its own owner, its own update and its own rollback.
None of them updates or rolls back another.

![The four layers: apps, shared services, OS and data, each with its owner, its update and its rollback](images/layers.svg)

**The rule: each app's image carries everything the app runs.** Its Python
and venv, `ffmpeg`, the libraries its wheels need, and any program of its own,
such as Photos' Qdrant or the Myira model service's llama.cpp. From the host an
app needs only the kernel, systemd, the network and the shared services'
ports. Nothing is installed on the host for an app, so:

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

**Where this isn't true yet.** Today only Photos' API runs in an image, and the
host has packages that belong to the apps:

| Today on the host | Moves to |
| --- | --- |
| Photos' workers, in a venv on the host's `python3` | Photos' image, beside the API |
| `ffmpeg`, installed with `apt` for the workers | Photos' image (already there); removed from the host |
| Qdrant, a binary installed by `run_qdrant.sh` | Photos' image, as its own unit from the same image |
| ZOOMRIYA ID, the box service, the Myira model service, Vault, Chat, Tracker, Planner, Myira, in venvs under `/opt/zoomriya/<app>/venv` | One image each |
| `restic` and the maps tools | The backup and maps images |

Each app is to follow Photos' image pattern; this design depends on finishing
that.

**Shared services.** PostgreSQL, Valkey and Caddy stay on the host as Debian
packages. They are servers every app talks to over a port, never something an
app links against, so the rule above isn't broken: an app's image is the same
whichever PostgreSQL 17 security fix the host has. They get Debian's security
fixes with the OS. Their major versions only change when the whole box moves to
the next Debian ([§6.6](#66-moving-to-debian-14)), which is when PostgreSQL's cluster is upgraded too.

## 3. The disk

The box has a fast system SSD and two drives for the library (Solutions
Design, *Hardware and selling*).

**System SSD**

| Partition | Size | What |
| --- | --- | --- |
| EFI | 1 GB | Boot loader, kernels |
| recovery | 4 GB | A small ZOOMRIYA OS that can reinstall the system ([§4](#4-boot-recovery-and-factory-reset)) |
| system | the rest | Btrfs, with the subvolumes below |

**Subvolumes on the system partition**

The rule: **an OS snapshot holds only the OS**. Everything an app or a family
writes sits in its own subvolume, so rolling the OS back never rolls back a
photo, a message or a database row.

| Subvolume | Mounted at | In OS snapshots | Notes |
| --- | --- | --- | --- |
| `@` | `/` | Yes | Debian, `zoomriya-base`, `zoomriya-agent`, `/etc` |
| `@machines` | `/var/lib/machines` | No | App images; the agent keeps the current and previous release of each |
| `@postgres` | `/var/lib/postgresql` | No | Every app's database. Snapshotted with a dump before an app migrates ([§6.2](#62-app-releases)) |
| `@valkey` | `/var/lib/valkey` | No | |
| `@data` | `/data` | No | `/data/zoomriya`, `/data/zoomriya-id`, `/data/models`, `/data/maps`, `/data/qdrant`, `/data/zoomriya-myira` |
| `@log` | `/var/log` | No | Kept across rollbacks, so the reason for a rollback is still there |
| `@snapshots` | `/.snapshots` | No | snapper's snapshots of `@` |

**Change needed in the apps:** the box service and the maps service keep state
in `/var/lib/zoomriya-box` and `/var/lib/zoomriya-maps`, which are on `@` and
would roll back with the OS. Either they move under `/data`, or each gets its
own subvolume. Moving them under `/data` is simpler and puts them in the
backups' reach; that's the proposal.

**Library drives.** Two mirrored drives, set up at the factory on the sold box.
Whether the mirror is Btrfs RAID 1 or `mdadm` is open (OS-4); Btrfs is leaning,
because its checksums find a bad copy and repair it from the good one during a
scrub, which `mdadm` can't tell apart. Photos' `storaged` keeps mounting drives
as today; it never formats them.

**Disk encryption** is open (OS-11). The
layout allows LUKS under the system partition and the library drives, unlocked
by the TPM with Secure Boot (`systemd-cryptenroll`), with the recovery key shown
once at setup.

## 4. Boot, recovery and factory reset

**Secure Boot on.** Debian's signed shim, boot loader and kernel, so Secure
Boot works without our own keys. Custom signed images (UKIs) wait for the A/B
step ([§6.7](#67-later-a-read-only-system)).

**A kernel that doesn't boot falls back by itself.** Debian keeps the previous
kernel installed. With systemd-boot's boot counting, a new kernel gets three
tries; if it fails all three, the boot loader starts the previous one, and the
agent reports it in Box Desktop. `systemd-bless-boot` marks the new kernel good
once the box's health check passes (`zoomriya-boot-check`: `/` is `@` and
writable, `/data` is mounted, the firewall, the network, the agent and Box
Desktop are up); when it doesn't, the box restarts and the kernel uses up a
try. `panic=10` on the kernel command line makes a kernel that can't even find
its root restart too. Whether Debian 13's signed systemd-boot is usable here or
GRUB has to stay is OS-5.

**Recovery partition.** A small ZOOMRIYA OS image, never updated in place. It
starts when the main system fails to boot after its tries run out, or when the
reset is asked for (below). It can roll the system back to a snapshot, or
reinstall it from the factory snapshot or a USB stick, and it never touches
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

The first setup happens in front of the box, on a screen and keyboard plugged
into it. Being there is what proves the box is yours, so nothing of the setup
goes over the network, and it needs neither the apps nor ZOOMRIYA Connect.

Before the setup shows, the box gets its sign-in ready by itself.
`zoomriya-services-init` makes PostgreSQL's cluster on `@postgres` (the image
carries none, so no two boxes share one), and tmpfiles.d gives Valkey its folders; then
`zoomriya-id-setup` runs ZOOMRIYA ID's `deploy/setup.sh`: its database role
and database, its settings in `/etc/zoomriya/id.env` and its folder, only
what's missing, with a mark written last. Both run again after Erase
everything or Reset the system, so the box comes back the same way. ZOOMRIYA
ID is in the image as the package `zoomriya-id`, built from its own
repository; it starts with no accounts.

Then `zoomriya-setup` shows the setup app full screen on tty1 (sway, locked to
one app with no keys or bars of its own, and a GTK 4 app in Python, drawn in
software so any screen works), until the box is set up:

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

Each step is done on the box as soon as it's confirmed, and the setup asks the
agent where things stand when it starts (`setup-status`), so a power cut loses
nothing: an admin made before it takes the setup straight to Done. Setting the
box up again takes Erase everything.

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

The box may also join the home Wi-Fi, after setup. ZOOMRIYA Connect needs
the cable.

- **Joined from Box Desktop,** in Control Panel › Network › Wi-Fi, by an admin,
  after first boot. Wi-Fi is off until then. WPA2 and WPA3 personal networks
  only; open networks aren't offered.
- **On Wi-Fi alone, everything at home works:** Family Hub, the apps, Box
  Desktop and phones backing up at home, all at the box's name, which then
  points at its Wi-Fi address.
- **ZOOMRIYA Connect needs Ethernet:** reaching the box away from home (the
  phone app's and the desktop app's link) and guests' share links. Without
  the cable, ZOOMRIYA Connect pauses, and Box Desktop and Family Hub say "Your
  box can't be reached away from home until it's connected to the router by a
  cable." It starts again by itself when the cable is back. The reasons are
  ZOOMRIYA Connect's: a steady address for port forwards, reliable mappings,
  and no Wi-Fi client isolation
  ([ZOOMRIYA Connect](https://github.com/zoomriya/zoomriya-connect/blob/main/docs/where-it-works.md)).
- **With both connected, Ethernet carries everything** (a lower route metric),
  and Wi-Fi stays joined so the box is still reachable at home if the cable is
  pulled.
- **When phones on the Wi-Fi can't reach the box** (some routers keep Wi-Fi
  devices apart), Box Desktop says so and suggests the cable.
- **What it adds to the OS:** `iwd` with systemd-networkd, and the firmware for
  the box's own Wi-Fi chip only ([Licences](licences.md)). The Wi-Fi password is
  kept readable by root only, and never in logs or the diagnostics bundle.
  Bluetooth stays off.
- **Setting up over Wi-Fi** from the start, with no cable, would need the box to
  open its own setup hotspot; that's OS-14.

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
- **Wi-Fi** keeps DHCP.

## 6. Updates

| What | Who installs it | When | Rollback |
| --- | --- | --- | --- |
| Debian security fixes (host) | unattended-upgrades | By themselves, every day | snapper snapshot of `@` |
| Kernel | unattended-upgrades | Installed by itself; takes effect at the next restart, in the maintenance hour | Previous kernel, by boot counting |
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
- **An interrupted update fixes itself.** If power was cut during an update,
  `zoomriya-base` runs `dpkg --configure -a` at the next boot, before anything
  else updates.
- **The apps aren't restarted for it.** `needrestart` would restart services
  that use an updated library; the app units are excluded, since their images
  don't use the host's libraries.
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

What happens when an admin presses **Update** (whether updates can also be
scheduled is OS-12):

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

An image is frozen at the Debian packages it was built with, so the host's
security fixes ([§6.1](#61-debian-security-fixes)) never reach it. Photos' image carries `ffmpeg`, which
parses every video and photo a family uploads, so its fixes matter most. Four
parts handle it: keep images small, notice fixes, rebuild without anyone's
hands, and install them like the host's.

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

**3. Rebuild without anyone's hands.** CI rebuilds the marked image from the
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

**What a hole in `ffmpeg` can reach until then.** Fixes take a day or two to
reach a box, so the code that parses uploads runs as if it were already broken
into: inside the image, as its own user, with `ProtectSystem=strict` and only
`/data/zoomriya` writable (Photos' units today). Running each `ffmpeg` call in a
transient unit with no network (`IPAddressDeny=any`), a system call filter and
a time and memory limit narrows it further; that's for zoomriya-apps to add.

### 6.4 Shared services

PostgreSQL, Valkey and Caddy get their security fixes through [§6.1](#61-debian-security-fixes). Each fix
restarts that one service (needrestart), which the apps already survive: they
reconnect to PostgreSQL and Valkey by themselves.

### 6.5 Models and maps

Unchanged: the models under `/data/models` and the street maps under
`/data/maps` update when an admin asks, apart from any release.

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

A writable root is the price of letting Debian patch the box by itself. The
units' `ProtectSystem=strict` already makes `/usr`, `/etc` and `/boot` read-only
to every service, and nobody can log in, so only apt and the agent can change
the system.

Once CI can rebuild, test and publish a whole system image whenever Debian
publishes a fix, the box can move to two read-only system partitions, A and B,
checked by dm-verity: the update writes the other one and the boot loader falls
back if it doesn't come up. Because everything we add to Debian is already a package
(`zoomriya-base`, `zoomriya-agent`), mkosi builds those images from the same
packages, and nothing changes for families.

## 7. zoomriya-agent

The one root helper the OS adds, socket-activated like Photos' `storaged`, and
written in Go: one static binary, standard library only. Its clients are the
box's own screen: the first setup, and Box Desktop once an admin has signed
in ([Box Desktop §6](box-desktop.md#6-how-its-built)); it lets in only root
and the group `zoomriya-box`. Family Hub and the apps never ask it. The agent
accepts only these requests, with fixed
arguments, never a command line:

| Request | What it does |
| --- | --- |
| `status` | OS version, last security update, a restart needed and when, recent restore points |
| `disk-health` | SMART for every drive, as "Healthy", "Check soon" or "Replace now" with the reason |
| `apps` | Each app's active and previous release |
| `app-update`, `app-restore` | [§6.2](#62-app-releases) |
| `os-upgrade` | [§6.6](#66-moving-to-debian-14) |
| `restart`, `maintenance-hour` | Restart now, or set the hour |
| `restore-point-list`, `restore-point-restore` | snapper snapshots of `@` |
| `support-mode` | Turn on key-only SSH for support's key, for at most 24 hours, then off by itself |
| `factory-reset` | [§4](#4-boot-recovery-and-factory-reset) |
| `diagnostics` | One bundle: the journal, versions, disk health, each app's diagnostics; no family data |
| `storage`, `storage-check` | The library's space by what uses it, the mirror's state, a scrub now (for Box Desktop) |
| `resources` | CPU, memory, network, drive activity and temperatures, for the box and per app from its units' cgroups |
| `network`, `security`, `time` | The box's network, its outbound connections, the firewall and security checks (read only), the clock |
| `network-set`, `network-keep` | A port's fixed address or DHCP, undone after two minutes unless kept ([§5.2](#52-a-fixed-address)) |
| `notifications` | The last month's notifications from the watcher (`zoomriya-agent watch`, on a timer): drives, updates, restarts, the library's check, network settings undone |
| `keyboard`, `keyboard-set` | The layout of a keyboard plugged into the box, in Debian's `/etc/default/keyboard`, from a fixed list |
| `logs` | Box events, the audit log and admins' sign-ins, filtered, with no family content, a hundred entries at a time, newest first |
| `shutdown` | Shut the box down |
| `wifi` | List the networks in reach, join one, forget it ([§5.1](#51-wi-fi)); the password is passed once and never logged |
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
  (Ethernet, and Wi-Fi when joined) while it's on, and never through ZOOMRIYA
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

**Our own list.** The hardening below is our own, written as settings files in
`zoomriya-base`, and checked by our own tests on every build. Where we choose
not to follow common hardening advice, the reason and what stands in for it
are written down (the last table).

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
  network (Ethernet, and Wi-Fi when joined) mDNS, and SSH on 22 while advanced
  access or support mode is on ([§7.1](#71-advanced-access)); and DHCP. The
  first setup is on the box's own screen and listens on no port. connectd's short-block set (ZOOMRIYA
  Connect) lives in the same table. Checked: the rules, and that nothing else
  listens.
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
| Wireless interfaces off | A family may put the box where only Wi-Fi reaches ([§5.1](#51-wi-fi)) | Off until an admin joins a network; WPA2 or WPA3 only; firmware for the box's chip only; the same firewall rules as Ethernet; Bluetooth off |
| A boot loader password | Nobody would ever type it | Secure Boot; the boot menu's editor off |
| PAM password rules, lockout, password ageing | Families never log into Linux | ZOOMRIYA ID's rules: Argon2id, growing delays per account |
| USB storage off | Backups and the reinstall stick use it | Plugged-in drives mounted `nosuid,nodev,noexec` by `storaged` |

**Checked in CI** on every image build, booted in a VM: one check per item in
this section, in `tests/hardening/`, checking both the settings and what is
actually running, fails the build if any item is missing; Lynis catches
general hardening drift beyond our list. The list and its differences above
are what we hand to anyone who asks.

**The law.** The EU Cyber Resilience Act (vulnerability reporting from
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

**What we build**, all with mkosi, Debian 13:

| Output | For |
| --- | --- |
| Disk image | Written to the system SSD at the factory |
| Installer (USB) | Bring-your-own mini PC, and reinstalling |
| Recovery image | The recovery partition |
| `zoomriya-base`, `zoomriya-agent`, `box-desktop-app` (`.deb`) | Our own signed apt repository |
| App images | Built in zoomriya-apps, as Photos' are today |

**The USB installer** (`mkosi/installer`, `zoomriya-installer`), for a
bring-your-own computer and for reinstalling: a small read-only system on the
stick that carries the disk image and shows one full-screen app, five steps
down a side: Welcome checks the computer (UEFI, 4 GB of memory, a drive of 64
GB or more) and looks for the internet, which it never needs: without it, a
warning, and the install goes on, the image being on the stick (checking for
security fixes while installing is still to design); Drive lists every drive, the stick itself and drives too small
greyed out with the reason, and draws the chosen drive now and after
installing; Summary asks once more, with a tick for "Erase everything on …";
Install copies while a slideshow tells what the box does; Done offers Restart
now, or Power off for a box set up later by its owner (the factory's case).
The install is two passes of `systemd-repart`: the image's partitions copied
as they are (`--copy-from`), then the system partition grown to the end of the
drive with the same definitions as the factory's, then its Btrfs grown to
fill it and a boot entry added (`efibootmgr`; the firmware's fallback path
works without it). The first setup then runs at the box's next start. Tested
in a VM: `scripts/run-installer-vm.sh`.

**Security rebuilds of app images** ([§6.3](#63-security-fixes-inside-app-images)): a CI job every 6 hours compares
each published image's package manifest with Debian 13's security archive and
runs `pip-audit` over its wheels, then rebuilds, tests, signs and publishes any image
with a fix waiting.

**CI, for every change and whenever Debian publishes a security update:**

```
build ─► boot in a VM ─► first boot ─► app update and rollback ─► security
update and rollback ─► kernel that doesn't boot falls back ─► hardening checks ─►
sign ─► publish
```

The same app images are also tested on Debian 14 and on testing as soon as they
exist, so a problem with the next Debian shows up a year before families would
see it.

Every build records its software bill of materials and the source packages it
used.

## 11. Layout

`mkosi/host/`, `zoomriya-base`, `zoomriya-agent`, `box-desktop-app`,
`scripts/` and `tests/vm/` exist (plan steps 1, 4 and the start of 3); the
rest is planned.

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
| OS-1 | Writable root now, read-only A/B later ([§6.7](#67-later-a-read-only-system)) | (a) writable with snapshots, (b) A/B from the first box | Leaning (a) |
| OS-2 | Debian updates on the host | (a) all install by themselves, (b) all wait for the admin, (c) security fixes by themselves, the rest wait | Leaning (c) |
| OS-3 | App image rebuilds for Debian security fixes | (a) wait for the admin's Update like any release, (b) install by themselves like the host's, (c) by themselves only when the app's code is unchanged | Leaning (c), [§6.3](#63-security-fixes-inside-app-images) |
| OS-4 | The library mirror | (a) Btrfs RAID 1, (b) `mdadm` RAID 1 with ext4 | Leaning (a) |
| OS-5 | Boot loader | (a) systemd-boot with boot counting, if Debian 13's signed build works with shim on the box, (b) GRUB with a fallback entry | Leaning (a); to test on the box |
| OS-6 | Reaching factory reset without signing in | (a) a USB stick with a reset file, (b) the recovery partition, chosen by holding a key at power-on, which needs a keyboard, (c) both | Leaning (c) |
| OS-7 | Our own mirror of Debian's security archive ([§6.1](#61-debian-security-fixes)) | (a) not at first, (b) from the first box | Leaning (a) |
| OS-8 | Where releases and our apt repository are hosted | `zoomriya.net` (control plane and guest page today) or a new name | Open |
| OS-9 | This repository's licence | MPL-2.0 like ZOOMRIYA Connect, with `TRADEMARKS.md` | Decided |
| OS-10 | Box and maps state moving from `/var/lib` to `/data` ([§3](#3-the-disk)) | Move, or a subvolume each | Leaning move |
| OS-11 | Disk encryption ([§3](#3-the-disk)) | (a) LUKS by default, unlocked by the TPM with Secure Boot, a recovery key shown once at setup, (b) optional, (c) LUKS with a passphrase at every boot | Open |
| OS-12 | Scheduling app updates ([§6.2](#62-app-releases)) | (a) only when an admin presses Update, (b) also on a schedule the admin chooses | Open |
| OS-13 | Box Desktop's own decisions: Family Hub's Update button, notifications, where each admin's settings live | [Box Desktop §10](box-desktop.md#10-open-decisions) | Open |
| OS-14 | Setting up over Wi-Fi, with no cable ([§5.1](#51-wi-fi)) | (a) not at first: setup needs the cable, (b) the box opens a setup hotspot named on the sticker, protected by the setup code | Leaning (a) |

## 13. Plan

1. **The host image:** mkosi disk image with the partitions and subvolumes,
   `zoomriya-base` with os-release, sources, firewall, sysctl and journald;
   boots in a VM and on the box.
2. **Every app in an image:** finish moving the apps and Photos' workers into
   nspawn images (zoomriya-apps); move box and maps state under `/data`.
3. **zoomriya-agent** with `status`, `restore-point-list`,
   `restore-point-restore`, `restart`, `maintenance-hour`, `disk-health`,
   `factory-reset`, `storage`, `storage-check`, `network` and `resources`,
   and the maintenance hour's restart (done, and tested in a VM), then `apps`,
   `app-update`, `app-restore`, and Box Desktop's first tools
   ([Box Desktop §11](box-desktop.md#11-plan)).
4. **Security updates:** unattended-upgrades, snapper, restoring, boot
   counting, the health check that marks a new kernel good, and the
   maintenance hour (done, and tested in a VM across a kernel update, a
   restore and a kernel that can't start); the CI job that rebuilds app
   images when Debian fixes a package in them
   ([§6.3](#63-security-fixes-inside-app-images)).
5. **First boot:** mDNS name and setup code (done); the first setup on the
   box's screen with the admin in ZOOMRIYA ID (done, `zoomriya-setup`); then
   ZOOMRIYA Connect's name and certificate, when it's added.
6. **Recovery and reset:** the reset levels (done, from Box Desktop, and
   tested in a VM); the recovery partition's image, and the USB reset.
7. **Hardening:** done but for the profiles that come with the gateway,
   connectd and the apps; one check per item in `tests/hardening/`,
   and Lynis's hardening index reported on each run (67 at first).
8. **Proven on the box itself:** a clean install, a restore drill, and an
   update and rollback.
9. **Later:** our own security mirror, the read-only A/B system, Debian 14.

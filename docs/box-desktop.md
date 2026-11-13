# Box Desktop: design

Design and implementation status · November 13, 2026

Box Desktop is the administrator's desktop environment on the ZOOMRIYA Box.
It brings together tools for storage, updates, restore points, system
resources, logs and settings, including reset options. Tools open in windows
from a main menu, with widgets and a top bar for common actions. Box Desktop
also plays and records sound through the box's speakers and microphone.

Box Desktop is a web desktop. An admin opens it in a browser on the home
network, at `https://<name>.zoomriya.app:7443`, and later from away from home
too. The box's own screen shows the same desktop, full screen. The `box-desktop`
package is part of ZOOMRIYA OS and lives in this public repository. Family Hub,
the family's main entry point, is developed in the private `zoomriya-apps`
repository. Box Desktop communicates with
`zoomriya-agent` ([Design §7](design.md#7-zoomriya-agent)) and uses ZOOMRIYA
ID for sign-in ([§7](#7-signing-in)). It contains no code from
`zoomriya-apps`.

**Contents**

1. [Decisions](#1-decisions)
2. [Who sees it and where](#2-who-sees-it-and-where)
3. [The desktop](#3-the-desktop)
4. [The tools](#4-the-tools)
5. [Words](#5-words)
6. [How it works](#6-how-it-works)
7. [Signing in](#7-signing-in)
8. [Security](#8-security)
9. [What it doesn't do](#9-what-it-doesnt-do)
10. [Open decisions](#10-open-decisions)
11. [Plan](#11-plan)

## 1. Decisions

| Decision | State |
| --- | --- |
| The box's system controls are Box Desktop's, in this public repository, not Family Hub's | Decided |
| A web desktop: Svelte in the browser, a Go server on the box (`box-desktopd`), on its own port, 7443 (12 November 2026; it replaced the native GTK desktop, `box-desktop-app`, now removed) | Decided |
| The web desktop does everything the native one did, with the same tools, tabs, words, translations, look and behaviour ([§6.3](#63-the-same-as-the-native-desktop)) | Decided |
| The box's own screen shows the same web desktop in a full-screen WebKit window with no browser around it | Decided |
| HTTPS with a real certificate: ZOOMRIYA Connect's `<name>.zoomriya.app`, pointing at the box's home address. Without ZOOMRIYA Connect: `<name>.local` with the box's own certificate, trusted once | Decided |
| From the home network now; from away from home through ZOOMRIYA Connect's apps later | Decided |
| Can't sign in? (the setup code from the sticker lets an admin back in) on the box's own screen only: over the network, anyone who has seen the code could use it | Decided |
| A desktop with windows, top bar, main menu and widgets, with our own look, icons and names | Decided |
| Admins only; other members never see it | Decided |
| `zoomriya-agent` stays the only part that runs as root; Box Desktop runs as its own user and asks the agent ([§6](#6-how-it-works)) | Decided |
| Plain words first, the technical details one click away ([§5](#5-words)) | Decided |
| Myira in a panel, admins only: the OS's own Myira for the box, or the Myira app's answering in the same panel when it is installed; never a link from one to the other ([§4](#myira)) | Decided (13 November 2026), not built |
| The box's one model service is part of ZOOMRIYA OS (`zoomriya-model`), with the API the apps already use ([§4](#myira)) | Decided (13 November 2026), not built |
| Admins manage the family's accounts on the box as well (Control Panel › People), so a box with no Family Hub can still add someone. ZOOMRIYA ID keeps the accounts and checks every change, whichever screen asks ([§4](#control-panel)) | Decided |

## 2. Who sees it and where

| Where | Address | Certificate | When |
| --- | --- | --- | --- |
| A browser on the home network | `https://<name>.zoomriya.app:7443` | ZOOMRIYA Connect's, a real one | With ZOOMRIYA Connect |
| A browser on the home network | `https://<name>.local:7443` | The box's own, trusted once | Always |
| The box's own screen | `https://<name>.local:7444`, on 127.0.0.1 only, in a full-screen WebKit window | The box's own, trusted for its own name only | Once setup is done |
| Away from home | `https://<name>.zoomriya.app:7443`, through ZOOMRIYA Connect's desktop app | ZOOMRIYA Connect's | Later |

- **For admins only.** An admin signs in with their ZOOMRIYA ID. Anyone else is
  told Box Desktop is for the box's admins ([§7](#7-signing-in)). Other members
  use Family Hub and the apps.
- **Several at once.** Each browser has its own sign-in. Two admins can be
  signed in at once, and so can one admin on two computers.
- **First boot is apart from it.** The first setup runs on the same screen
  ([Design §5](design.md#5-first-boot)) and makes the admin in ZOOMRIYA ID;
  Box Desktop starts once that's done.

## 3. The desktop

![Box Desktop with Storage Manager in front of Control Panel, the shortcuts on the left and the widgets on the right](images/box-desktop.png)

*From the mockup; the names, sizes and times are examples.*

The taskbar runs along the top of the screen:

| Left | Middle | Right |
| --- | --- | --- |
| The main menu button, the ZOOMRIYA mark | One button per open window; click to bring it forward or minimise it | A restart that's needed; notifications; the sound's volume; the network cable, connected or not (it opens Network); the widgets; the clock |

The main menu shows each tool as a large icon, with a search box. Any
tool can be dragged to the desktop as a shortcut. Its foot shows who is signed
in (their initials and name) and the box, with Open Family Hub on a phone,
Lock screen, Restart the box and Sign out on the right. **Open Family Hub on
a phone** shows the box's address (`https://<box>/`, where Family Hub is) and
a QR code a phone's camera opens it from, the quickest way to get a family
member's phone onto it; System Info has it too.

**The launcher** along the bottom holds each tool's tile, its name when the
pointer is on it, and a dot under each one that's open. The six tools are
always there, in the same order. After a thin divider come the apps the admin
pinned (up to eight, each admin their own, kept with their settings): the pin
(Add to dock) beside an installed app in App Center, or **Remove from dock** from
the app's menu (right-click or press and hold its tile). An app removed from
the box leaves the launcher too. Until apps carry their own icons, an app's
tile is its first letters ("Ph", "FH") on a colour of its own.

**Frosted glass** is the launcher's, the top bar's, and the sign-in's and the
lock screen's cards: the background behind them shows through, blurred and a
little richer in colour, under a light tint of the theme's (white in
Daylight, dark in Midnight; the cards a little more solid, for their words
and password). The browser blurs what's behind each (CSS `backdrop-filter`):
the background, and a window under the launcher.

**An app opens in a window of its own**, like the tools': the app's pages,
from the box's own address (`https://<box>/<id>/`, as a phone opens it),
inside the desktop, with no address bar around them, so it reads as an app:

- **Signed in as the admin.** Signing in to Box Desktop also signs the
  browser in to ZOOMRIYA ID for the box's name (its own cookies, `HttpOnly`,
  passed on by `box-desktopd`; browsers don't keep cookies apart by port), so
  the apps open signed in. Signing out ends both.
- **The box's pages only.** The gateway lets only the box's own pages, and
  Box Desktop on the box's name at 7443 and 7444, show an app
  (`frame-ancestors`, in `zoomriya-apps`); other sites can't. Vault lets no
  page show it, by its own design: its window offers to open it in a tab of
  its own.
- **No way past a problem.** An app that isn't running shows a plain message
  with Open App Center.

The app's menu also has **Open on a phone or computer**: the app's address,
and a QR code a phone's camera opens it from; and **Manage in App Center**.

**A link off the box opens in the web window** (planned 12 November 2026,
not built). The app windows still follow the box's pages only; a link to
anywhere else (a result from Myira's `/web`, a site an app links to) opens in
a web window of its own, so the admin can read it at the box. It opens from a
link only: there is no address to type (BD-14).

| Piece | Plan |
| --- | --- |
| Engine | The same WebKitGTK as the app windows: nothing new to ship, its security fixes come with the OS's, and each page runs in WebKit's own sandbox |
| Its own program and account | A small program of its own (`box-web`), run as its own account `zoomriya-web`: not in `zoomriya-box`, so it can't reach the agent, and no access to `/data`. Box Desktop starts it with the link and nothing else |
| Session | Its own, in memory only (`NetworkSession.new_ephemeral()`), never the app windows': a site never sees the admin's ZOOMRIYA ID sign-in. Closing the window forgets everything |
| Privacy | Third-party cookies off, WebKit's tracking prevention on, trackers and ads blocked by WebKit's content filters (`UserContentFilterStore`) from a blocklist we ship and update with the OS (which list: BD-15) |
| Limits | HTTPS only (an `http://` link is tried as `https://`, else a plain message); nothing downloaded; camera, microphone, location and notifications refused; no saved passwords, no history, no developer tools, no other windows |
| Look | A thin bar: back, reload, the site's name with its full address beside it, **Open on a phone or computer** (the link as a QR code, as for an app) and close. Frosted glass like the app's menu |
| Internet | Declared in `/etc/zoomriya/outbound.d/box-web.json` as an optional connection to any host ([Design §8.2](design.md#82-internet-access)): switched off there, or with **Apps offline**, a link shows "The web window is switched off" and its QR code instead |
| No way past a problem | A certificate that isn't right, or a page that can't load, shows a plain message, never a "continue anyway" |

Not a headless browser: one with no screen fetches pages for a program to
read, which Myira's web search leaves out on purpose.
Not a full browser either: a larger program with its own updates and
settings, which wouldn't look like Box Desktop.

**Lock screen** keeps the admin signed in and their windows as they were, and
shows the box's background with the time, their initials and name, and a
password field. Their password (checked with ZOOMRIYA ID, as for "your password
again") brings the desktop back with Unlock, on the right; Switch account,
on the left of the same row, signs them out instead. Locking, signing out and the sign-in screen are only Box Desktop's:
the agent, ZOOMRIYA ID, the apps and the library are services of their own
and go on running whoever is signed in, or no one. The box never sleeps.

**When no one's using it,** the screen locks, then turns off, as Control
Panel › Display says: **Lock the screen after** (never, or 1 to 30 minutes;
5 at first) and **Turn the screen off after** (never, or 5 to 60 minutes; 10
at first). They're the box's own, for everyone at its screen, kept beside its
background (`box.json`). Any key, click, touch, scroll or move counts as use;
the first one after the screen went off turns it back on (the monitor's
power, through sway: `output * power off`). The sign-in screen turns off
too. Only the monitor sleeps; the box runs on.

**Search finds everything in Box Desktop** (the main menu's search box,
ready to type in as the menu opens): the tools; every section, also by the other words
people use for it ("ssh" finds Control Panel › Advanced access, "reboot"
Restarts, "wifi" Network); every setting in them ("Time Zone", "Serial
Number"); Support Center's questions; the desktop's actions (Restart the box,
Shut down, Lock screen, Sign out, Midnight, Daylight, the widgets); and last, always, the
system log searched for the same words. Words match in the box's language and
in English, with or without accents; Enter opens the first. The pages are
drawn only when opened, so the search has its own list (`search.py`), and a
test checks that each entry points at a real section. The desktop starts with
Control Panel, Storage Manager, App Center, Resource Monitor and Log Center.

![The main menu open over the desktop: a search box and every tool as an icon, with Family Hub last](images/box-desktop-menu.png)

Windows can be moved, resized, minimised, maximised and closed. Several can
stay open at once, but each tool opens only once; selecting it again brings
its window forward. Sections appear down the left side of each window. Box
Desktop remembers each admin's open windows and their positions, desktop
shortcuts and widgets, so their layout is ready at their next sign-in.

Widgets appear in a panel on the right, which can be hidden. They show box
health, CPU and memory, storage space, network, backups and updates. Admins
choose which widgets to show and in what order. Each widget opens its related
tool when clicked. Box events appear in Notifications and in full in Log
Center.

The agent's watcher (`zoomriya-agent watch`) supplies notifications. It runs
every 15 minutes, after security updates and after each library check, then
records events in the journal. These include changes in drive health,
installed security fixes, planned restarts, library check results and network
settings that were not kept and were undone, and advanced access turned on or
off. Backups may be added later. The bell in the top bar lists the latest notifications, newest
first, with a count of the ones since the admin last read them, and links to
Log Center › Notifications, which keeps the last month's. Each
admin turns kinds on or off in Control Panel › Notifications; their choices
are kept with their own settings ([§6](#6-how-it-works)). Family Hub's box
card shows the same few that matter to a family.

**Keyboard and screen readers.** Every window, menu and widget can be reached
and used with the keyboard: Tab between a window's parts, and these keys on
the desktop (sway binds none of its own, so they reach Box Desktop first):

| Keys | What they do |
| --- | --- |
| Super, pressed and let go on its own | Opens or closes the main menu, ready to search |
| Super+L | Locks the screen |
| Escape | Closes the sheet on top (a question, Add a picture); in a menu, closes it |
| Ctrl+W | Closes the window in front |
| Alt+Tab | Goes to the next window, bringing it back if it was minimised |

They work only on the desktop of a signed-in admin; the sign-in and lock
screens keep their own keys. Windows are labelled regions, and background
jobs and notifications are announced.

**Control Panel › Appearance** is where an admin chooses how Box Desktop
looks:

- **Style:** Daylight or Midnight, the apps' two themes, each shown as a
  small desktop. Midnight unless the admin switches.
- **Accent Colour:** blue (the apps' own), teal, green, yellow, orange, red,
  pink, purple or slate, for buttons, switches and what's selected. Each has
  a Daylight and a Midnight shade that read at 4.5:1 or more, as text on the
  page and under a button's words (`appearance.py`; a test checks every one).
- **Background:** the box's, one for everyone, shown on the sign-in screen,
  the desktop and the lock screen, so a change shows on all three at once.

Style and accent are each admin's own, kept with their settings
([§6](#6-how-it-works)) like their mouse and keyboard.

**The backgrounds** are our pictures, five plain colours, or a picture of the
family's own. The default is Smoke: soft waves of smoke in teal, violet and
pink along the bottom of a deep night-blue sky
([`backgrounds/wallpaper.jpg`](../packages/box-desktop/web/public/backgrounds/wallpaper.jpg)).
The sky stays dark and quiet where the windows and widgets sit, and the left
side stays dark so the shortcuts' labels read well. The others are Smoke at
dusk (the same on slate blue), Hills (dusk over the hills, with one house's
window lit) and Lakeside (a cabin by a lake at sunrise); the plain colours are
Night, Ocean, Forest, Plum and Graphite.

**Add Picture** lists the JPEG and PNG pictures on the USB sticks plugged
in, in each stick's top folder and the folders in it, with a preview of each.
Only pictures can be added, and the file on the stick never reaches Box
Desktop or the box's disk:

- The agent mounts the stick read-only, `noexec`, `nosuid`, `nodev`, and
  reads only regular files named `.jpg`, `.jpeg` or `.png`, of 40 MB or less;
  links, hidden files and anything else on the stick are left alone.
- It decodes each one with Go's own JPEG and PNG decoders, refusing a picture
  of more than 100 million pixels from its header, before decoding it. A
  script, a web page or anything else named `.jpg` doesn't decode, so it isn't
  listed and can't be added. SVG, which can carry scripts, isn't read at all.
- It then draws the picture into a new JPEG of its own, at most 3840 pixels
  across and turned the right way up (a phone's EXIF orientation). Anything
  else in the file, such as its metadata or data after the picture, is left
  behind.
- Box Desktop keeps only that new JPEG, and only if it is a JPEG, as
  `box.jpg` beside the admins' settings. Adding another replaces it.

![The empty desktop: shortcuts on the left, widgets on the right, and the wallpaper between](images/box-desktop-empty.png)

## 4. The tools

Each tool opens in a window, with its sections listed down the left side. The
last column shows where its data comes from. Requests marked *new* still need
to be added to the agent
([Design §7](design.md#7-zoomriya-agent)).

### Control Panel

Control Panel contains the box's settings, grouped into sections listed down
the left side of the window. Within each section, related settings appear in
groups of rows. Each row has a title, a short explanation and a control on
the right, such as a switch, list or slider. Status appears at the top where
it is useful, such as the Protected and Secure Boot indicators in Security.
Each row explains its purpose in plain language.

| Group | Section | What's in it | From |
| --- | --- | --- | --- |
| | Appearance | Style (Daylight or Midnight) and the accent colour, each admin's own; the box's background, for everyone and every screen (sign-in, desktop, lock): one of ours, a plain colour, or Add Picture from a USB stick. See [§3](#3-the-desktop) | Box Desktop's own, `pictures` |
| | Display | When no one's using it: lock the screen after, turn the screen off after (the box's own, [§3](#3-the-desktop)). The box's own screen, since every family's monitor is different: orientation, resolution (with its shape, 16:9), refresh rate, scale. A change shows at once and goes back by itself after 15 seconds unless Keep is pressed; kept, it's the box's, for whoever signs in, and the screen starts that way from then on (`/etc/zoomriya/display.conf`, which screen-session adds to sway's settings). Only on the box's screen | `display-set` |
| | Sound | Which speakers and microphone (when there's more than one), their volume and mute, a test sound, and what the microphone hears. Only when the box has a sound card | PipeWire, Box Desktop's own |
| | Keyboard & Mouse | Each admin's own, like the theme. Keyboard: the layout (English, French, German…) of a keyboard plugged into the box; the box uses the last one an admin saved, and each admin's own when they sign in on the box. Mouse, for one plugged into the box: the primary button (Left or Right), pointer speed, acceleration, scroll direction (traditional or natural); inside Box Desktop on another computer, the mouse follows that computer | `keyboard`, `keyboard-set`; preferences |
| | Language & Region | The box's language and its country or region (the first setup's), with a note that ZOOMRIYA's own screens are in English for now | `setup-status`, `language-set`, `region-set` |
| | Date and time | The date and time now, the time zone, whether the clock is set from the internet | `setup-status`, `time-zone-set` |
| | Restarts | The maintenance hour, a planned restart, Restart now, Shut down | `maintenance-hour`, `restart`, `shutdown` *new* |
| Updates | Update and restore | Debian's security fixes (installed by themselves, with the last one's date), ZOOMRIYA OS 2 when it's offered ([Design §6.6](design.md#66-moving-to-debian-14)), the update history | `status`, `os-upgrade` |
| | Restore & Reset | From the mildest fix to the most drastic: the restore points (the OS's snapshots by date, and Restore to this point); the updates a restore held back, with Update now ([Design §6.1](design.md#61-debian-security-fixes)); Reset the system; and apart at the bottom, Erase everything ([Design §4](design.md#4-boot-recovery-and-factory-reset)). Each asks for the admin's password again; Erase everything also for the setup code | `restore-point-list`, `restore-point-restore`, `held`, `held-release`, `factory-reset` |
| Family | People | Everyone with a ZOOMRIYA ID on the box: admins first, then by name, with Admin, Child, Teen or Off beside the name, a child's parents or guardians, and "Needs a parent or guardian" when none is left. **Add a person**: their name, a username, adult or under 18 (a child also needs a birth year and one or two parents or guardians from the adults), then the person types their own password at the box, twice. **Edit**: their name (and a child's birth year and parents); **New password** (the person types it; they're signed out everywhere; your own needs your current one first); **Make admin** or **Remove admin** for another adult; **Turn off** (signed out everywhere, nothing deleted) or **Turn on**. See below | ZOOMRIYA ID's `/users`, Box Desktop's own |
| Network | This box on the network | The box's name, each port's address, the router, the cable's speed, and whether ZOOMRIYA Connect is on (it needs the cable); each Ethernet port by DHCP or a fixed address, with Keep these settings ([Design §5.2](design.md#52-a-fixed-address)) | `network`, `network-set`, `network-keep` |
| | Wi-Fi | Only on a box with a Wi-Fi chip: Wi-Fi turned off and greyed out, "The box connects to the router by a cable" ([Design §5.1](design.md#51-wi-fi), OS-14) | `network` |
| | Remote access | A link to Family Hub's Remote access page, where ZOOMRIYA Connect's settings are | none |
| | Outbound connections | What the box connects to and why, with the switches ([Design §6.1](design.md#61-debian-security-fixes)) | `network` *new* |
| Security | Security & Privacy | Security updates (automatic, the last installed, Check now to install them at once; never off); the firewall, what it lets in and why in plain words, read only (no opening ports by hand, [Design §8](design.md#8-hardening)); the last file integrity check and what changed, AppArmor, Secure Boot; Privacy: that the box reports nothing about the family, every host on the internet it connects to and why, and under it **Sent in the Last 7 Days**: what actually went out, by app and where, with how often and when last, kept current every 15 seconds while the page is open | `security`, `security-check-now`, `outbound` |
| | Notifications | Each kind of notification on or off, for this admin, and See past notifications (Log Center) | none (Box Desktop's own) |
| | Advanced access | SSH for admins: on (the password again) or off; the keys, each with its fingerprint, when it was added and last used, and Remove; Add from a USB stick or Paste a key (the password again); the command and the box's host key fingerprint; what hand changes mean ([Design §7.1](design.md#71-advanced-access)) | `advanced-access` |
| Box | System Info | The ZOOMRIYA mark and the box's name; Family Hub's address with Open on a phone (its QR code); its name, model and serial, System OS (os-release's `PRETTY_NAME`, "ZOOMRIYA OS 1 Alpha", and the factory image), security updates until when (os-release's `SUPPORT_END`), the processor, memory, disk capacity, temperature, how long it's run, the time zone, who's signed in; and under Details: Debian's point release, `zoomriya-base`, the factory image, the kernel. Last in the list | `status`, `resources`, `storage`, `setup-status` |

**People** is ZOOMRIYA ID's: Box Desktop asks ZOOMRIYA ID on the box
directly (`GET`/`POST /users`, `PATCH /users/{id}`) with the signed-in
admin's token, not the agent, since the accounts aren't the OS's. Family Hub's
Family page manages the same people from elsewhere, so the two always agree.
The rules are ZOOMRIYA ID's, so they hold on either screen:

- **Everyone chooses their own password.** On the box, the person types it
  when they're added and when it's reset, so the admin never needs to know
  it, and it's theirs from the start (`"temporary": false`). One an admin
  sets in Family Hub is temporary. ZOOMRIYA ID won't sign anyone in with it
  until they choose their own.
- **The box always has an admin.** No one can remove their own admin role or
  turn off their own account, and ZOOMRIYA ID refuses any change that would
  leave no active admin.
- **A child is never an admin**, and every child has one or two parents or
  guardians who are adults with accounts that are on.
- **Changing someone's role or password, or turning them off, signs them out
  everywhere**, since their tokens carry the old role.
- **Making or removing an admin and turning an account off ask for the
  password again** ([§7](#7-signing-in)).
- **Each change goes in Log Center's Admin actions**, with the admin who made
  it and never a password ([§8](#8-security)).

A child's own settings (which apps, quiet hours, moving between child and
teen) stay with their parents in Family Hub. Admins don't manage them unless
they're that child's parent or guardian. Accounts are turned off, never
deleted, so a person's photos and files stay theirs.

### Storage Manager

| Section | What's in it | From |
| --- | --- | --- |
| Overview | The library's space used and free, the mirror's state when there are two drives, each drive's health in one line | `storage` *new*, `disk-health` |
| Drives | Each drive: its place, model, size, hours on, temperature, health ("Healthy", "Check soon", "Replace now" with the reason), and its SMART details under Details | `disk-health` |
| Space | What takes the space: photos and videos, each app's data, the databases, models and maps, restore points | `storage` *new* |
| Backups | The box's backup (zoomriya-apps' backup service: restic, every app's data and the accounts): the last one and how it went ("Backed up", "The last backup failed", with each app that failed and why, in the backup's own words), Back up now, Nightly backup on or off (with the next one's time), and where they go (a folder, or a kind and a host, never a password). Not set up: says to set it up in Family Hub's settings, which also choose the apps left out. Bringing a backup back is for later (below) | `backups` |
| Checks | The library's last and next check (a Btrfs scrub, OS-4), Check now, what it found, and what it repaired when there are two drives | `storage` *new*, `storage-check` *new* |
| Replace a drive | With two drives (later): step by step, with pictures of the box: which drive, when it's safe to pull it, and the rebuild's progress | `storage` *new* |

Box Desktop never formats, mounts or unmounts a drive itself; that stays with
Photos' `storaged`, and the Replace a drive steps call it through the agent.

### App Center

| Section | What's in it | From |
| --- | --- | --- |
| Installed | What's being installed and its step, or how the last install went; each app with Running, Starting, Stopped or Not working beside its name, then its version and when it was installed; Restore <previous version>; a pin, pressed while the app is on this admin's launcher ([§3](#3-the-desktop)), named Add to dock or In dock; Remove (keep its data, or delete it too) | `apps` |
| Add from USB | Look on USB stick: each package in `zoomriya/apps/` on the sticks plugged in, as new, an update, older, or already installed, with Install or Update; a package not signed by ZOOMRIYA shows "Not a ZOOMRIYA app. It can't be installed." | `apps` |

For now apps come only from a USB stick, and only ZOOMRIYA's: each package is
checked against ZOOMRIYA's key before anything changes ([Design §6.9](design.md#69-app-center-apps-from-a-usb-stick)).
Installing, restoring and removing ask for the password again. An app's own
settings stay in the app. Later: a catalogue to download from, and Updates and
History sections.

### Resource Monitor

| Section | What's in it | From |
| --- | --- | --- |
| Overview | CPU, memory, network and drive activity, live, with the last hour and the last week | `resources` *new* |
| By app | The same for each app and shared service, from its systemd units' cgroups, so "Photos is using most of the CPU" can be seen at once, with what it's doing ("Finding faces") where the app says | `resources` *new* |
| Temperature | The CPU's and drives' temperatures and the fan | `resources` *new* |

### Log Center

| Section | What's in it | From |
| --- | --- | --- |
| Box events | Updates, restarts, drive health, backups, advanced access, in plain words | `logs` *new* |
| Notifications | The last month's notifications, newest first, with their kind, including the kinds an admin turned off on their bell; Control Panel › Notifications and the bell link here | `notifications` |
| Admin actions | The audit log: who asked the agent for what, and when ([Design §7](design.md#7-zoomriya-agent)) | `logs` *new* |
| Sign-ins | Admins' sign-ins to Box Desktop, with the device | `logs` *new* |
| Family activity | ZOOMRIYA ID's activity list (90 days): sign-ins and wrong tries in Family Hub and the apps, second steps, passwords, accounts, AI keys, and what apps report (new devices, share links, setting changes); the same as Family Hub's Activity | ZOOMRIYA ID `GET /users/activity`, as the signed-in admin |
| System log | The whole journal, for support: errors, warnings or everything, and a search | `logs` |
| Diagnostics | The diagnostics bundle. The same page as Support Center's | `diagnostics` |

Logs are shown filtered and in plain words first; the journal's own lines are
under Details, for support. No family content is ever in them.

### Support Center

| Section | What's in it | From |
| --- | --- | --- |
| Diagnostics | Make a bundle: what's wrong, in the admin's words; the system log and the network details in or left out. The bundles on the box, newest first, each with what its checks found, **Review** (the note, the checks, each file, readable), **Save to USB** and Delete. How to send one: save it to a stick, upload it at zoomriya.com/support from a computer, give support the code ([Design §7.2](design.md#72-getting-help-the-diagnostics-bundle)) | `diagnostics` |
| Contact us | zoomriya.com/support, to open on a phone or computer; what to tell support and how to send a bundle; the box's model, serial number, ZOOMRIYA OS version, address, and security updates until when ([Design §6.6](design.md#66-moving-to-debian-14)) | `status` |
| Help | Common questions, each with a button to the tool and section that answers it | none |

Every other tool's window has a **?** on its bar that opens Help. Help is
built into the box, in its language, because the box works without the
internet; zoomriya.com/support has the same answers and more.

ZOOMRIYA support never signs in to the box; the bundle is how it finds what's
wrong. While advanced access is on, the top bar's pill says so when nothing
more urgent is shown, and Box health shows it.

### Myira

Planned 13 November 2026, not built. Myira, the box's assistant, in a panel of
Box Desktop, for admins only. **One panel, whichever Myira is on the box:** the
OS has its own Myira for the box, and when the Myira app (one of the ZOOMRIYA
apps) is installed, that Myira answers in the same panel instead. The admin
never sees two assistants, and the panel never sends them somewhere else.

```
Box Desktop: the Myira panel (this repository)
      │  the assistant contract: ask, stream, chats, cards
      ├─▶ the Myira app installed and on for this admin ──▶ the Myira app answers
      │                                                       everything it answers on /myira,
      │                                                       plus the box, through the OS's tools
      └─▶ otherwise ──────────────────────────────────────▶ the OS's own Myira: the box only
```

| Piece | Plan |
| --- | --- |
| Where | A **Myira** button on the top bar, left of notifications; a panel slides in from the right, in frosted glass, above the windows. Help's **?** in each tool opens it too, with that tool named |
| Who answers | The Myira app, when it is installed and on for this admin (the app's own switch: Myira's skills follow the apps). Otherwise the OS's own Myira. `box-desktopd` decides for each question, so installing or removing the app changes the panel straight away |
| The OS's own Myira | Answers questions about the box: storage, drives, backups, restore points, updates, people, network, ZOOMRIYA Connect, sound and the screen. How-tos come word for word from Support Center's Help, each with a button to the tool and section; the box's state from the OS's tools (below). Written for this repository; it holds no other Myira's code or words |
| A question that isn't about the box | Without the Myira app, one fixed reply: "For now I can help with the box. With the ZOOMRIYA apps installed, I can answer much more." No link, no other app named. With the app, the question is simply answered |
| The OS's tools (its skill) | Served by `box-desktopd` in the apps' tool shape (name, description, input schema, whether it changes something): reading the box's health, storage, backups, updates, network and ZOOMRIYA Connect; changing what the tools change without the password again (an update's time, a backup now, the volume), each on a card the admin confirms. Always on for an admin |
| Never through Myira | What asks for the password again (§7): Erase everything, Reset the system, Restore to this point, Shut down, advanced access, SSH keys, making or removing an admin, turning an account off. Myira opens that tool's section instead |
| Tools are called by the panel | Whichever Myira answers, it can only pick one of the OS's tools; the panel calls it through `box-desktopd` as the admin and sends back what came of it. The Myira app never reaches into the OS, and the OS's tools are never offered anywhere but this panel (not on `/myira`, not to AI apps through `/mcp`) |
| Chats | The OS's own Myira keeps none: the panel's chat is gone when it is closed. With the Myira app, the panel shows the admin's own Myira chats, the same as on `/myira`, kept by the app |
| The model | The box's one model service, part of ZOOMRIYA OS (`zoomriya-model`): llama.cpp's `llama-server` (MIT) and a small Go front for sign-in and choosing the model, on `127.0.0.1:8070`, loopback only, with the API the apps already use (`/myira/api/v1/complete`, `/complete/stream`, `/describe`, `/model`). It replaces the apps' own model service, so the box still loads one model, shared. The model files (Gemma 4, Apache-2.0, [Licences](licences.md)) are in `/data/models` |
| No model yet | The panel says "Myira needs her model to answer (3.2 GB)." with **Download**, through the model's declared connection ([Design §8.2](design.md#82-internet-access)), or from a USB stick |
| Words | Every reply and button in `po/<lang>.po`, in every language the box has, changed on the fly like the rest of Box Desktop |

**The assistant contract.** Published in this repository so the Myira app can
answer in the panel without sharing any of its code with the OS:

| Request | What it does |
| --- | --- |
| `POST /assistant/ask` | A question (and the open chat's id) with the OS's tools offered; answers as a stream, one JSON object per line: `text`, `choices` (buttons), `source` (a page's name and date), `tool` (a tool picked, with its input), `card` (a change to confirm), `done` or `error` |
| `POST /assistant/ask/tool` | What came of a tool, so the answer goes on |
| `GET /assistant/chats`, `GET /assistant/chats/{id}`, `DELETE /assistant/chats/{id}` | The admin's chats, for a provider that keeps them |

`box-desktopd` asks it on loopback as the signed-in admin, with their ZOOMRIYA
ID token; the OS's own Myira answers the same contract inside `box-desktopd`.

## 5. Words

Storage tools often assume that people know terms such as "volume" and "RAID
group." Box Desktop is intended for whoever manages the family's box, so it
uses these principles:

- **Plain words first.** "Library drives", "Healthy", "Check soon", "Restore
  to this point", "The box will restart at 3:30 tonight". The technical names
  (Btrfs, SMART, snapper, nftables, unit names) are under each section's
  Details, for support and for those who want them.
- **Every warning says what to do:** "Drive 2 should be replaced soon. Your
  photos are safe on drive 1. Order a 4 TB or larger drive", with a link.
- **Every risky button says what it changes and what it keeps**, as the
  factory reset table in [Design §4](design.md#4-boot-recovery-and-factory-reset) does.

## 6. How it works

```
a browser on the home network              the box's own screen (tty1)
  https://<name>.zoomriya.app:7443           sway, one window full screen
  https://<name>.local:7443                    └ box-desktop-screen: WebKit, no browser around it
        │                                              │ https://<name>.local:7444 (127.0.0.1)
        └──────────────┐            ┌──────────────────┘
                       ▼            ▼
             box-desktopd (Go)     account box-desktop: groups zoomriya-box,
                │                  zoomriya-tls and audio; never root
                ├ the desktop itself   Svelte, built into the program
                ├ ZOOMRIYA ID          http://127.0.0.1:8050, sign-in and People
                ├ zoomriya-agent       Unix socket, its fixed requests only
                ├ PipeWire             the box's speakers and microphone
                └ sway                 the box's screen, keyboard and mouse
```

### 6.1 The pieces

| Piece | What it is |
| --- | --- |
| `box-desktopd` | A Go program, `box-desktop.service`. It serves the desktop and its API over HTTPS on TCP 7443 (the home network) and on 127.0.0.1:7444 (the box's screen only), with the same certificates. It asks the agent and ZOOMRIYA ID on the admin's behalf |
| The desktop | Svelte 5 and TypeScript, built with Vite and put into `box-desktopd` when it's built. No code from `zoomriya-apps`: only its colours, through `export-tokens.mjs` |
| `box-desktop-screen` | The box's screen (`box-desktop-screen.service` on tty1): a WebKitGTK window full screen under sway, with no address bar, tabs or menus. It opens `https://<name>.local:7444` (the box names itself 127.0.0.1 in `/etc/hosts`), trusts exactly the box's own certificate for that name, and goes nowhere else. Same account as `box-desktopd`, so the sound and the screen's settings are its own |
| The certificate | `box-desktopd` loads every bundle in `certs/`, as the gateway does, and picks one by the name the browser asks for: ZOOMRIYA Connect's `<name>.zoomriya.app`, else the box's own `<name>.local`. It loads them again when they change |
| Live updates | One server-sent event stream per signed-in page: the agent's `watch` (notifications, a restart that's needed, the drives), the widgets' numbers and the sound's level |
| Its words | The same `po/<lang>.po` files as the native desktop's, the same English. `scripts/i18n.py` turns them into JSON for the browser; the language changes on the fly |
| Each admin's settings | As before, in `/data/zoomriya/admin` (BD-4), the same files and format, so an admin's settings carry over from the native desktop |

### 6.2 What happens where

The box's hardware stays the box's, whichever browser asks. A change made from
a laptop changes the box, just as it did at the box's screen.

| Setting | Changes | Done by |
| --- | --- | --- |
| Volume, speakers, microphone, Test sound, the mic's level | The box's speakers and microphone | `box-desktopd`, through PipeWire |
| Display (resolution, scale, rotation, Keep these settings) | The box's screen | `box-desktopd` through sway; the agent's `display-set` saves it |
| Keyboard & Mouse | The box's screen, while that admin is signed in there | `box-desktop-screen`, when that admin signs in |
| Lock screen, and locking when no one's using it | The browser it's pressed in; the box's screen also locks itself and turns off | The page itself; `box-desktop-screen` turns the screen off |
| Restart the box, Shut down, everything in the tools | The box | The agent |

### 6.3 The same as the native desktop

The web desktop does everything the native one did, with the same words and
translations (its `po/` began as the native desktop's), and was checked
section by section before the native code was removed on 12 November 2026.
Its look goes a little further than the native one's: real blur behind the
glass, Inter's optical sizes, and smoother motion.

| Native | Web |
| --- | --- |
| Sign-in, the temporary password, the second step and setting one up, Can't sign in? | The same cards and words |
| Top bar, main menu and search, widgets, launcher, windows that move, minimise and maximise, shortcuts dragged to the desktop | The same, drawn in the browser |
| Frosted glass (`glass.py`) | CSS `backdrop-filter`, the same tints, over everything behind it |
| The keys (Super for the menu, Super+L to lock) | The same keys on the box's screen. In a browser, the browser keeps Super, so the menu has Ctrl+Space |
| Control Panel, Storage Manager, App Center, Resource Monitor, Log Center, Support Center, each tab | The same tabs, the same agent requests |
| Graphs (`graphs.py`) | SVG, the same look |
| Icons and colour tiles (`icons.py`, `tiles.py`) | The same drawings as SVG |
| QR codes (Open Family Hub on a phone) | Drawn in the page as SVG (`qrcode-generator`); the second step's is ZOOMRIYA ID's own, as before |
| From a USB stick (apps, SSH keys, a picture, diagnostics) | The same, from a stick in the box (choosing a file on the computer instead: BD-16) |
| An app's window (`appview.py`) | A window inside the desktop showing the app's page from the box's own address, signed in (§3). Vault opens in a tab of its own |
| A link off the box (planned) | In a browser, a new tab. On the box's screen, the web window as planned (BD-14, BD-15) |

### 6.4 Settings and packaging

- **Each admin has their own settings**, stored in a record under
  `/data/zoomriya/admin`, named by a hash of their member
  id and readable only by Box Desktop's account. It stores their theme and
  accent colour, keyboard and mouse settings, notification preferences and
  desktop layout. The box's background is beside them, the same for
  everyone: `box.json` says which, and `box.jpg` is the family's own picture.
  Settings are written to a new file and renamed into place, then checked
  when read; if a file is damaged, Box Desktop uses the defaults. The data
  lives on `@data`, so an OS restore or Reset the system does not roll it
  back. Erase everything does (BD-4).
- **Box Desktop is packaged with the OS** as a `.deb` from our apt repository,
  alongside `zoomriya-base` and `zoomriya-agent`. It updates with ZOOMRIYA OS's own
  releases, not with the apps.
- **The visual design is ours:** the icons, colours, artwork and names, and the
  ZOOMRIYA logo. Its two themes are the apps' own: Midnight, the default, and
  Daylight, chosen by each admin in Control Panel › Appearance and kept with
  their settings, with their accent colour.
  Every library, font and icon goes through [Licences](licences.md) first.
- **Seen without a box:** `scripts/preview-desktop.sh` runs `box-desktopd`
  with sample data and opens it in a browser.

## 7. Signing in

Box Desktop does not keep its own accounts. Like the other apps,
[ZOOMRIYA ID](https://github.com/zoomriya/zoomriya-id) handles sign-in. Box
Desktop sends the username and password to the local ZOOMRIYA ID service
(`/id/api/v1/auth/login`), checks the account (`/auth/me`) and allows access
only to an administrator. The access token lasts 15 minutes. `box-desktopd`
keeps the token and the login's session cookie in memory, never in the
browser, and renews the token with it (`/auth/refresh`) when People needs it,
so People works for as long as the admin stays signed in. The browser holds
only Box Desktop's own session cookie ([§8](#8-security)). Signing out also
ends the ZOOMRIYA ID session. A restart of `box-desktopd` signs everyone out.

**A temporary password** (an admin set it for them in Family Hub) gets no
token: ZOOMRIYA ID answers with `new_password_needed`. The sign-in card then
asks for the admin's own password, twice, with Not now to go back, and signs
in with both (`/auth/login/new-password`), which replaces the temporary one.
Successful sign-ins and refusals are recorded in the journal and shown in Log
Center's Sign-ins.

**Risky actions ask for the password again**: Erase everything, Reset the
system, Restore to this point, Shut down, turning on advanced access, adding an
SSH key, making or removing an admin, and turning an account off. Box Desktop
checks the password with a fresh ZOOMRIYA ID sign-in, then drops that session
without signing out, since signing out would end every session of that
admin's, including this desktop's and their phone's. Erase everything also
needs the setup code from the sticker.

## 8. Security

| Guard | How |
| --- | --- |
| Admins only | Checked with ZOOMRIYA ID at every sign-in |
| The home network only | TCP 7443 answers addresses on the home network only, like advanced access. The router never forwards it; ZOOMRIYA Connect's outside check warns if someone does. Away from home is through ZOOMRIYA Connect's apps, later |
| HTTPS | TLS 1.3 on 7443, with HSTS. Port 7444 listens on 127.0.0.1 only, for the box's screen |
| Its session cookie | `__Host-box-desktop`: `Secure`, `HttpOnly`, `SameSite=Strict`, random, kept in memory. Signed out after 12 hours, or 30 minutes with the page closed |
| Requests from elsewhere | Every change needs an exact `Origin` (the box's own names on 7443, or on 7444 at the box) and a token sent with the page. `zoomriya.app` isn't on the Public Suffix List, so a cookie alone isn't trusted |
| Content Security Policy | Scripts and styles from the box only, no inline scripts. Box Desktop can't be shown inside another site's page (`frame-ancestors 'none'`) |
| Wrong passwords | ZOOMRIYA ID's limits, wherever the password was typed (BD-10) |
| Risky actions | The password again ([§7](#7-signing-in)), checked by `box-desktopd`, not the page |
| Its own account | `box-desktop` (each admin's settings moved to it from the native desktop's `box-desktop-app` on upgrade). In groups `zoomriya-box` (the agent's socket), `zoomriya-tls` (the certificates) and `audio`, never root. The unit locks it down further (`ProtectSystem=strict`, no capabilities, a system call filter) |
| The agent's requests | Each one on `box-desktopd`'s list ([`agent.go`](../packages/box-desktop/src/agent.go)); the page can't name who asked, nor ask for the first setup's |
| The agent's fixed requests | `box-desktopd` passes on only the requests on its own list, with the admin's username as `by`. It adds no way to run a command; a bug in it can do no more than an admin pressing its buttons |
| The audit log | The agent's own lines, and People's changes, which `box-desktopd` writes to the journal itself (`SYSLOG_IDENTIFIER=box-desktop`, `ZOOMRIYA_AUDIT=1`), with the browser's address. Log Center's Admin actions shows both |

## 9. What it doesn't do

- **No terminal.** A command line is only through advanced access's SSH, from
  the home network ([Design §7.1](design.md#71-advanced-access)).
- **No files.** Photos and Vault keep the family's files; Box Desktop never
  shows their content, only how much space they take.
- **No app settings, and no children's settings.** An app's settings stay
  in the app. A child's apps and quiet hours stay with their parents in
  Family Hub. People manages the accounts themselves.
- **No deleting people.** An account is turned off, so their photos and
  files stay theirs.
- **No apps from others,** and no way to run anything that isn't one of our
  signed app images.
- **No content from the family in logs, notifications or the diagnostics
  bundle.**

## 10. Open decisions

| # | Decision | Options | State |
| --- | --- | --- | --- |
| BD-1 | Its name | Box Desktop: the package `box-desktop` | Decided |
| BD-2 | Family Hub's own Update button | (a) keep a short Update / Restore on Family Hub's box card, (b) only the box's health there, with updates on the box's screen | Open |
| BD-3 | Notifications reaching an admin's phone | (a) Family Hub's notifications show the important ones, (b) Box Desktop only | Leaning (a) |
| BD-4 | Where each admin's settings live | (a) Box Desktop's state folder on `@`, which an OS restore takes back to the restore point's, (b) under `/data`, which a restore doesn't touch ([Design §3](design.md#3-the-disk), OS-10) | Decided (b): `/data/zoomriya/admin` |
| BD-5 | Bringing a backup back | (a) Storage Manager › Backups lists the backups by date and app, and restores one app or all through the agent (zoomriya-apps' `restore.sh`: stops the app, restores, starts it), the password again; (b) Support Center only, by hand | Open |
| BD-6 | A UPS | Plug a USB UPS into the box; its battery and time left in Box health; shut down cleanly when it runs low (Debian's NUT); a notification on mains power lost and back | Open |
| BD-7 | Problems reaching an admin away from the screen | A failing drive or a failed backup today shows on the box's screen and in Family Hub's box card: (a) a push to each admin's phone through Family Hub (with BD-3), (b) email as well | Open |
| BD-8 | Shared folders for the family's computers (SMB) | Laptops' own backups (Time Machine, File History) and a shared folder; against "Photos and Vault keep the files" (§9), so a product decision first | Open |
| BD-9 | More for the drives | Drives sleeping when idle, scheduled SMART tests, and adding a second drive to make a mirror (beside Replace a drive) | Open |
| BD-10 | Repeated wrong passwords | Sign-ins shows them; ZOOMRIYA ID limits tries. Block a device that keeps guessing, and say so in Box Desktop | Open |
| BD-11 | Apps' updates | From a USB stick only today; the catalogue (§11) brings them online, with security fixes first | Open |
| BD-12 | A touch screen with no keyboard | Nothing pops up by itself: GTK asks for an on-screen keyboard, but the box's screen (sway and Box Desktop) has none. An on-screen keyboard program that sway's input-method support can show, or Box Desktop's own for the sign-in and lock screens | Open |
| BD-13 | Screenshots for support | Save the screen to a USB stick, or add it to a diagnostics bundle | Open |
| BD-14 | The web window's address bar (§3) | (a) links only: it opens from a link, nothing to type, (b) also an address to type, making the box's screen a browser | Leaning (a): a general browser is a large target on the screen where an admin is signed in |
| BD-16 | Files from the computer | In a browser on a computer, choose a picture, an app (`.zapp`) or an SSH key from that computer, as well as from a USB stick in the box | Open |
| BD-17 | Myira's model on a new box ([§4](#myira)) | (a) the first setup offers it, (b) the first time the panel is opened, (c) in the OS image (3.2 GB more to download and flash) | Leaning (b): the OS works without it, and a box that never uses Myira never downloads it |
| BD-18 | An emergency in the OS's own Myira ([§4](#myira)) | Someone types they're hurt or in danger into the box-only Myira: (a) the fixed reply as for any other question, (b) a fixed line of its own with the emergency number of where the family lives | Open |
| BD-15 | The web window's blocklist (§3) | A tracker and ad list turned into WebKit's content-blocker rules. The well-known lists are GPL-3.0 or CC BY-SA, so their terms are checked first; or a list of our own | Open |

## 11. Development plan

The web desktop is built in this order. Each step is done when it matches
the native desktop ([§6.3](#63-the-same-as-the-native-desktop)).

1. **The server and sign-in.** Done (12 November 2026): `box-desktopd` in
   `packages/box-desktop/src`, with the certificates by name, the two ports,
   sessions, ZOOMRIYA ID's sign-in (the temporary password, the second step
   and setting one up), Can't sign in? at the box, the password again, the
   agent's requests, People, the event stream, each admin's settings (the
   same files), the firewall's 7443, and `scripts/i18n.py json` for the page's
   words. Its own tests, and the VM test's. Next: the sound and the screen's
   settings move into it with steps 3 and 4.
2. **The desktop.** Done: the top bar, the main menu and search, widgets,
   the launcher, windows, shortcuts, frosted glass, the lock screen (held by
   `box-desktopd` until the password, even if the page is opened again), both
   themes and the accents.
3. **The box's screen.** Done: `box-desktop-screen` under sway on tty1, the
   keys, turning the screen off, and each admin's keyboard and mouse there.
4. **Control Panel.** Done, every page.
5. **Storage Manager, Resource Monitor and Log Center.** Done.
6. **App Center and Support Center**, and apps in windows. Done (the gateway's
   `frame-ancestors`, in `zoomriya-apps`).
7. **Parity check, then the native desktop is removed.** Done
   (12 November 2026): `box-desktop` replaces `box-desktop-app` on upgrade;
   the VM test checks both ports and the box's screen. `scripts/preview-desktop.sh`
   shows it with the sample box; the page's own tests (`npm test`) and
   `box-desktopd`'s run with every build.
8. **Away from home** with ZOOMRIYA Connect's apps.
9. **Myira** ([§4](#myira)): the model service (`zoomriya-model`) in the OS
   with the apps' API, then the panel and the OS's own Myira with Help's words
   and the OS's tools, then the assistant contract, which the Myira app
   answers.

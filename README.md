# ZOOMRIYA OS

Behind the scenes, the ZOOMRIYA Box runs on a rock-solid Debian 13
system, carefully designed to stay completely out of your family's way.
Instead of scary command lines or tech jargon, your family sees the clean
Family Hub dashboard, and the box's admins get Box Desktop, where updating or
restoring your system is as simple as clicking a button.

![Box Desktop: Storage Manager and Control Panel open on the desktop, with the box's health, resources, space and updates as widgets on the right](docs/images/box-desktop.png)

**Status:** early. The system image builds and boots in a VM, takes Debian's
security fixes by itself, keeps restore points and restores them, and falls
back to the previous kernel when a new one doesn't start (plan steps 1 and 4).
The agent and Box Desktop have their first tools (step 3); the rest is design.
The screenshot is from the [Box Desktop](docs/box-desktop.md) mockup.

## What it is

- **Debian 13 minimal,** not a fork: Debian installed a fixed way, plus
  our settings package (`zoomriya-base`), a small root helper
  (`zoomriya-agent`) and an image build with mkosi.
- **systemd runs everything.** Each app is a versioned
  systemd-nspawn image, so updating the OS never changes an app.
- **Everything rolls back.** The OS through Btrfs snapshots, a kernel that
  doesn't boot through boot counting, an app through its previous image.
- **Debian's security fixes install by themselves;** app updates install when
  an admin presses Update.
- **Box Desktop, on the box's own screen, for its admins:** storage, updates,
  restore points, resources, logs and the box's settings, as windows on a
  desktop.
- **SSH off by default.** An admin can turn on advanced access: key-only SSH
  from the home network, with keys added from a USB stick or in Box Desktop.
  Support gets in only when an admin turns on support mode, for at most a day.
- **Hardened:** a default-deny firewall, AppArmor, auditd, sandboxed services
  and a minimal install, with every exception written down.
- **Only what we may use in a product we sell:** every component's licence is
  checked in [Licences](docs/licences.md).

## Building

On a Debian 13 machine, as your own user (no root needed after setup), with
[ZOOMRIYA ID](https://github.com/zoomriya/zoomriya-id) checked out beside this
repository (or `ZOOMRIYA_ID_SRC` naming it): the image carries it.

```sh
scripts/setup.sh            # installs what's missing (asks for sudo), checks KVM
scripts/build-image.sh      # builds build/image/zoomriya-os.raw
scripts/test-vm.sh          # boots it in a VM and runs tests/vm/check.sh
scripts/run-vm.sh           # boots it to look around: root / rootroot, this VM only
scripts/build-installer.sh  # the USB installer, carrying that image: build/installer/zoomriya-installer.raw
scripts/run-installer-vm.sh # installs it in a VM, to an empty 64 GB drive (--target: start what it installed)
scripts/preview-desktop.sh  # Box Desktop in a window here, with sample data
```

The image has no password of its own: `run-vm.sh` gives that VM root's
password, `rootroot`, when it boots. Each boot starts from the image as built.

## Read more

| Document | What's in it |
| --- | --- |
| [Design](docs/design.md) | The layers, the disk, boot and recovery, first boot, updates, the agent, hardening, names and versions, building, open decisions and the plan |
| [Installing](docs/install.md) | Putting ZOOMRIYA OS on a computer of your own, from a USB stick: what you need, building the stick, installing, the first setup |
| [Box Desktop](docs/box-desktop.md) | The admins' desktop on the box's screen: the desktop, its tools, how it's built and signs in |
| [Licences](docs/licences.md) | Every component's licence and whether we may use it in a product we sell |

## Licence

[MPL-2.0](LICENSE). Changes to these files stay open; you may combine them
with your own code under its own terms. The ZOOMRIYA name and logo are not
covered by the licence: see [TRADEMARKS.md](TRADEMARKS.md). The software ZOOMRIYA
OS is built from keeps its own licences: see [Licences](docs/licences.md).

## Related

Sign-in is [ZOOMRIYA ID](https://github.com/zoomriya/zoomriya-id); the apps and
the box service are in zoomriya-apps (private); remote access is in
[zoomriya-connect](https://github.com/zoomriya/zoomriya-connect).

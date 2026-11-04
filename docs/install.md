# Installing ZOOMRIYA OS

How to put ZOOMRIYA OS on a computer of your own, so it becomes a ZOOMRIYA Box.

**Early days.** The installer is tested in a VM (`scripts/run-installer-vm.sh`),
not yet on real computers, and there's no download yet: for now you build the
installer yourself. Expect rough edges, and use a computer whose drive you can
erase.

## What you need

- **A 64-bit PC (x86-64) that starts in UEFI mode,** with at least **4 GB of
  memory**.
- **A drive of 64 GB or more** for ZOOMRIYA OS. **Everything on it is
  erased.**
- **A USB stick of 2 GB or more** for the installer. It's erased too.
- **A keyboard, a mouse and a screen** plugged into the computer, for
  installing and the first setup.
- **An Ethernet cable to your router.** The first setup waits for it. The
  internet itself can come later: installing doesn't need it.
- To build the installer: a computer running Debian 13, as in the
  [README](../README.md#building).

## ZOOMRIYA ID comes with it

ZOOMRIYA OS includes [ZOOMRIYA ID](https://github.com/zoomriya/zoomriya-id),
the sign-in every ZOOMRIYA app shares. You don't install it separately: it's
built into the image, sets itself up the first time the box starts, and the
first setup makes your admin account in it. That's the account you sign in to
Box Desktop with.

To build, ZOOMRIYA ID's repository has to be next to this one (or named by
`ZOOMRIYA_ID_SRC`), because the image carries it.

## 1. Build the installer

```sh
git clone https://github.com/zoomriya/zoomriya-os.git
git clone https://github.com/zoomriya/zoomriya-id.git
cd zoomriya-os
scripts/setup.sh            # installs what's missing (asks for sudo)
scripts/build-image.sh      # ZOOMRIYA OS: build/image/zoomriya-os.raw
scripts/build-installer.sh  # the USB installer, carrying it: build/installer/zoomriya-installer.raw
```

You can try it in a VM first, with an empty 64 GB drive to install to:
`scripts/run-installer-vm.sh`, then `scripts/run-installer-vm.sh --target` to
start what it installed.

## 2. Write it to the USB stick

The installer is a disk image: write it to the stick as a whole, don't copy
the file onto it.

On Linux, find the stick's name with `lsblk` (by its size; for example
`/dev/sdb`), then:

```sh
sudo dd if=build/installer/zoomriya-installer.raw of=/dev/sdX bs=4M conv=fsync status=progress
```

Replace `/dev/sdX` with the stick, and check twice: `dd` erases whatever drive
it's given. On another system, use any tool that writes a disk image to a USB
stick.

## 3. Install

1. Plug the stick into the computer and start it from the stick. Most
   computers show a boot menu if you press a key while they start (often F12,
   F11, F8 or Esc; the screen usually says which).
2. **Welcome** checks the computer: started in UEFI mode, enough memory, and a
   drive big enough. If it started in the old BIOS mode instead, change that
   in the computer's firmware settings and start again. It also looks for the
   internet, through a cable to the router: without it you get a warning, and
   it installs anyway, since ZOOMRIYA OS is on the stick; the box gets its
   security fixes once it's online.
3. **Drive:** choose where ZOOMRIYA OS goes. Each drive shows what's on it now
   and what it will hold afterwards; the stick itself, and drives that are too
   small, can't be chosen.
4. **Summary:** tick that everything on the drive is erased, then **Erase and
   install**.
5. **Install** takes a few minutes. When it's **Done**, choose **Restart now**,
   or **Power off** to move the computer to where it will live, and take the
   stick out.

If the stick doesn't start with Secure Boot on, turn Secure Boot off in the
computer's firmware settings for now.

## 4. First start

The computer starts from its drive into the first setup, on its own screen:
the language, the keyboard, the network (plug in the cable), the country or
region, the time zone, and the box's admin, with a username and password of
your choice. When it's done, the screen shows Box Desktop's sign-in: sign in
with that admin.

From then on, ZOOMRIYA OS installs its security fixes by itself in the
maintenance hour (3:30 at night, until you move it in Control Panel ›
Restarts), and keeps restore points before and after each update.

## Starting again

To set the box up again from scratch: Control Panel › Restore & Reset ›
**Erase everything**, with your password and the box's setup code. Or install
again from the stick.

A ZOOMRIYA Box has its setup code on a sticker. A computer you installed
yourself makes its own the first time it starts; for now it's only written to
the system log, and showing it in the first setup is still to come.

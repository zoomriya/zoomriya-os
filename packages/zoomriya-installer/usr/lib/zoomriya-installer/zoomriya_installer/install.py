"""Writing ZOOMRIYA OS to the chosen drive (design §10).

The drive gets the same partitions as the factory's (mkosi/host/mkosi.repart),
in two passes of systemd-repart: the first copies the image's partitions from
the stick block for block, as they are; the second, with repart.d/, grows the
system partition to the end of the drive. The system's Btrfs then grows to
fill it, and the firmware gets a boot entry for the drive.
"""

from __future__ import annotations

import json
import os
import socket
import subprocess
import tempfile
import threading
import time
from collections.abc import Callable
from pathlib import Path

HERE = Path(__file__).resolve().parent
IMAGE = os.environ.get("ZOOMRIYA_INSTALLER_IMAGE", "/usr/share/zoomriya-installer/zoomriya-os.raw")
DEFINITIONS = HERE.parent / "repart.d"
LOG = Path(os.environ.get("RUNTIME_DIRECTORY", tempfile.gettempdir())) / "install.log"
LOADER = r"\EFI\systemd\systemd-bootx64.efi"

#: What the person sees, and each stage's share of the progress bar.
STAGES = [
    ("Getting the drive ready", 0.04),
    ("Copying ZOOMRIYA OS", 0.86),
    ("Making room for your files", 0.05),
    ("Making the drive start", 0.05),
]

Progress = Callable[[int, float], None]


class InstallError(Exception):
    """A step that failed, worded for the person; the details are in LOG."""


def run(*command: str, problem: str) -> str:
    with LOG.open("a") as log:
        log.write(f"$ {' '.join(command)}\n")
        done = subprocess.run(command, capture_output=True, text=True)
        log.write(done.stdout + done.stderr)
    if done.returncode != 0:
        raise InstallError(problem)
    return done.stdout


def sectors_written(disk: str) -> int:
    """The drive's own count of 512-byte sectors written since it appeared."""
    try:
        return int((Path("/sys/class/block") / Path(disk).name / "stat").read_text().split()[6])
    except (OSError, IndexError, ValueError):
        return 0


#: The partitions' numbers, in mkosi/host/mkosi.repart's order.
SYSTEM_PARTITION = 3


def partition(disk: str, number: int) -> str:
    """The drive's partition by its number, from the kernel's own list: udev
    may not have read the new partitions' labels yet."""
    for _ in range(10):
        for entry in (Path("/sys/class/block") / Path(disk).name).iterdir():
            try:
                if (entry / "partition").read_text().strip() == str(number):
                    return f"/dev/{entry.name}"
            except OSError:
                continue
        time.sleep(1)
    raise InstallError("The new partitions can't be found on the drive.")


def install(disk: str, progress: Progress) -> None:
    LOG.write_text(f"Installing {IMAGE} to {disk}\n")

    progress(0, 0.0)
    # Old filesystems' signatures, so nothing left over is found on the new partitions.
    tree = json.loads(run("lsblk", "-J", "-o", "PATH,TYPE", disk, problem="The drive went away."))
    for dev in tree["blockdevices"]:
        for child in dev.get("children") or []:
            run("wipefs", "--all", "--force", child["path"], problem="The drive's old partitions couldn't be cleared.")
    run("wipefs", "--all", "--force", disk, problem="The drive couldn't be cleared.")
    run("udevadm", "settle", problem="The drive didn't settle.")
    progress(0, 1.0)

    # The copy's progress, read from what the drive has written: every block
    # of the image's partitions is copied, empty or not.
    total = max(os.stat(IMAGE).st_size // 512, 1)
    start = sectors_written(disk)
    copying = threading.Event()

    def watch() -> None:
        while not copying.wait(1):
            progress(1, min((sectors_written(disk) - start) / total, 0.99))

    threading.Thread(target=watch, daemon=True).start()
    try:
        # No definitions of its own: the image's partitions, copied as they are.
        with tempfile.TemporaryDirectory(prefix="zoomriya-none-") as none:
            run("systemd-repart", "--dry-run=no", "--empty=force", "--seed=random",
                f"--definitions={none}", f"--copy-from={IMAGE}", disk,
                problem="ZOOMRIYA OS couldn't be copied to the drive.")
    finally:
        copying.set()
    progress(1, 1.0)

    progress(2, 0.0)
    run("systemd-repart", "--dry-run=no", f"--definitions={DEFINITIONS}", disk,
        problem="The system partition couldn't grow to fill the drive.")
    run("udevadm", "settle", problem="The drive didn't settle.")
    system = partition(disk, SYSTEM_PARTITION)
    with tempfile.TemporaryDirectory(prefix="zoomriya-system-") as mnt:
        run("mount", "-t", "btrfs", system, mnt, problem="The new system partition couldn't be opened.")
        try:
            run("btrfs", "filesystem", "resize", "max", mnt,
                problem="The system partition couldn't grow to fill the drive.")
        finally:
            run("umount", mnt, problem="The new system partition couldn't be closed.")
    progress(2, 1.0)

    progress(3, 0.0)
    # The firmware also finds \EFI\BOOT\BOOTX64.EFI by itself, so a firmware
    # that won't take an entry still starts the drive.
    if Path("/sys/firmware/efi/efivars").is_dir():
        try:
            run("efibootmgr", "--quiet", "--create", "--disk", disk, "--part", "1",
                "--label", "ZOOMRIYA OS", "--loader", LOADER, problem="")
        except InstallError:
            pass
    run("sync", problem="The drive couldn't finish writing.")
    progress(3, 1.0)


def overall(stage: int, fraction: float) -> float:
    """A stage's progress as a share of the whole install."""
    return sum(share for _, share in STAGES[:stage]) + STAGES[stage][1] * fraction


#: Where the installer looks for the internet: where the box's security fixes
#: come from.
INTERNET_CHECK = ("deb.debian.org", 443)


def online() -> bool:
    """The internet reached, through the stick's own network (a cable to the
    router). Installing doesn't need it: ZOOMRIYA OS is on the stick."""
    try:
        socket.create_connection(INTERNET_CHECK, timeout=5).close()
        return True
    except OSError:
        return False


def restart() -> None:
    subprocess.run(["systemctl", "reboot"], check=False)


def power_off() -> None:
    subprocess.run(["systemctl", "poweroff"], check=False)


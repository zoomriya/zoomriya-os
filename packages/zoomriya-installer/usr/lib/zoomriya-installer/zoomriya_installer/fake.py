"""Drives and an install to try the installer on a development machine.

    ZOOMRIYA_INSTALLER_FAKE=1 python3 -m zoomriya_installer
    ZOOMRIYA_INSTALLER_FAKE=fail python3 -m zoomriya_installer   # the copy fails

Nothing is written anywhere.
"""

from __future__ import annotations

import os
import time

from .disks import GB, Disk, from_lsblk
from .install import STAGES, InstallError, Progress

LSBLK = {"blockdevices": [
    {"name": "nvme0n1", "path": "/dev/nvme0n1", "type": "disk", "size": 512110190592,
     "model": "Samsung SSD 980 PRO 500GB", "tran": "nvme", "rm": False, "ro": False,
     "children": [
         {"name": "nvme0n1p1", "path": "/dev/nvme0n1p1", "type": "part", "size": 104857600,
          "fstype": "vfat", "partlabel": "EFI system partition"},
         {"name": "nvme0n1p2", "path": "/dev/nvme0n1p2", "type": "part", "size": 16777216},
         {"name": "nvme0n1p3", "path": "/dev/nvme0n1p3", "type": "part", "size": 510886920192,
          "fstype": "ntfs", "label": "Windows"},
         {"name": "nvme0n1p4", "path": "/dev/nvme0n1p4", "type": "part", "size": 1101004800,
          "fstype": "ntfs", "label": "Recovery"},
     ]},
    {"name": "sda", "path": "/dev/sda", "type": "disk", "size": 2000398934016,
     "vendor": "ATA", "model": "WDC WD20EFZX-68A", "tran": "sata", "rm": False, "ro": False, "rota": True},
    {"name": "mmcblk0", "path": "/dev/mmcblk0", "type": "disk", "size": 31268536320,
     "model": "DA4032", "tran": "mmc", "rm": False, "ro": False},
    {"name": "sdb", "path": "/dev/sdb", "type": "disk", "size": 32 * GB,
     "vendor": "SanDisk", "model": "Ultra Fit", "tran": "usb", "rm": True, "ro": False,
     "children": [
         {"name": "sdb1", "path": "/dev/sdb1", "type": "part", "size": 512 * 1024**2,
          "fstype": "vfat", "partlabel": "esp"},
         {"name": "sdb2", "path": "/dev/sdb2", "type": "part", "size": 3 * GB,
          "fstype": "erofs", "partlabel": "zoomriya-installer", "mountpoints": [None]},
     ]},
]}


def list_disks() -> list[Disk]:
    time.sleep(0.4)
    return from_lsblk(LSBLK)


def install(disk: str, progress: Progress) -> None:
    """The install played out in about a minute."""
    for stage, (_, share) in enumerate(STAGES):
        steps = max(int(share * 60), 3)
        for step in range(steps + 1):
            progress(stage, step / steps)
            time.sleep(0.5)
            if stage == 1 and step == steps // 2 and os.environ.get("ZOOMRIYA_INSTALLER_FAKE") == "fail":
                raise InstallError("ZOOMRIYA OS couldn't be copied to the drive.")


def restart() -> None:
    raise SystemExit(0)


def power_off() -> None:
    raise SystemExit(0)

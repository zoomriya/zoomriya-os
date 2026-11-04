"""The drives the installer may write to, from lsblk (design §10).

Every drive is listed, so the person sees the one they expect; those that
can't be used say why: the stick the installer runs from, a read-only drive,
one too small for the system.
"""

from __future__ import annotations

import json
import subprocess
from dataclasses import dataclass, field
from typing import Any

GB = 1000**3
#: The OS, the app images, every app's database, the models and the maps all
#: sit on the system drive (design §3).
MIN_BYTES = 64 * GB
#: mkosi/host/mkosi.repart, in order; the system partition takes the rest.
LAYOUT = [("EFI", 1 * 1024**3), ("Recovery", 4 * 1024**3)]
#: The stick's own partition (mkosi/installer/mkosi.repart).
INSTALLER_LABEL = "zoomriya-installer"
LSBLK_COLUMNS = "NAME,PATH,TYPE,SIZE,MODEL,VENDOR,TRAN,RM,RO,ROTA,FSTYPE,LABEL,PARTLABEL,MOUNTPOINTS"


@dataclass
class Part:
    name: str
    size: int
    what: str


@dataclass
class Disk:
    path: str
    name: str
    model: str
    size: int
    bus: str
    removable: bool
    #: A hard disk, not an SSD.
    spinning: bool = False
    parts: list[Part] = field(default_factory=list)
    #: Why it can't be chosen; empty when it can.
    reason: str = ""

    @property
    def usable(self) -> bool:
        return not self.reason

    @property
    def title(self) -> str:
        return self.model or f"Drive {self.name}"

    @property
    def kind(self) -> str:
        if self.bus in ("USB", "SD card"):
            return self.bus
        return f"{'Hard disk' if self.spinning else 'SSD'} · {self.bus}"


def size_text(size: int) -> str:
    """In the drive makers' units, as on the drive's label: 512 GB, 2 TB."""
    for unit, scale in (("TB", 1000**4), ("GB", GB), ("MB", 1000**2)):
        if size >= scale:
            value = size / scale
            return f"{value:.1f} {unit}" if value < 10 and round(value, 1) != int(value) else f"{value:.0f} {unit}"
    return f"{size} bytes"


def bus_name(tran: str | None, removable: bool) -> str:
    names = {"nvme": "NVMe", "sata": "SATA", "ata": "SATA", "usb": "USB", "mmc": "SD card",
             "virtio": "Virtual", "scsi": "SCSI", "sas": "SAS"}
    if tran in names:
        return names[tran]
    return "Removable" if removable else "Internal"


def vendor(dev: dict[str, Any]) -> str:
    name = (dev.get("vendor") or "").strip()
    # "ATA" is the bus, not the maker.
    return "" if name == "ATA" else name


def part_what(part: dict[str, Any]) -> str:
    """A partition, as a person would know it: its label, else its kind."""
    kinds = {"vfat": "FAT", "ntfs": "Windows (NTFS)", "ext4": "Linux (ext4)",
             "btrfs": "Linux (Btrfs)", "xfs": "Linux (XFS)", "swap": "Swap",
             "exfat": "exFAT", "crypto_LUKS": "Encrypted", "linux_raid_member": "RAID member"}
    name = part.get("label") or part.get("partlabel")
    kind = kinds.get(part.get("fstype") or "", part.get("fstype") or "Unknown")
    return f"{name} · {kind}" if name else kind


def from_lsblk(tree: dict[str, Any]) -> list[Disk]:
    disks = []
    for dev in tree.get("blockdevices", []):
        if dev.get("type") != "disk" or dev["name"].startswith(("loop", "zram", "ram", "sr")):
            continue
        removable = bool(dev.get("rm"))
        children = dev.get("children") or []
        disk = Disk(
            path=dev["path"],
            name=dev["name"],
            model=" ".join(filter(None, [vendor(dev), (dev.get("model") or "").strip()])),
            size=int(dev.get("size") or 0),
            bus=bus_name(dev.get("tran"), removable),
            removable=removable,
            spinning=bool(dev.get("rota")),
            parts=[Part(c["name"], int(c.get("size") or 0), part_what(c))
                   for c in children if c.get("type") == "part"],
        )
        mounted = any(m for c in [dev, *children] for m in (c.get("mountpoints") or []) if m)
        if any(c.get("partlabel") == INSTALLER_LABEL for c in children):
            disk.reason = "The installer is running from this drive."
        elif dev.get("ro"):
            disk.reason = "This drive is read-only."
        elif disk.size < MIN_BYTES:
            disk.reason = f"Too small: ZOOMRIYA OS needs at least {size_text(MIN_BYTES)}."
        elif mounted:
            disk.reason = "This drive is in use."
        disks.append(disk)
    # The likely choice first: an internal SSD, the smallest one, since the
    # big hard disks are the library's.
    disks.sort(key=lambda d: (not d.usable, d.removable, d.spinning, d.size))
    return disks


def layout_after(size: int) -> list[tuple[str, int]]:
    """The partitions the drive will have once installed."""
    used = sum(part for _, part in LAYOUT)
    return [*LAYOUT, ("ZOOMRIYA OS", max(size - used, 0))]


def list_disks() -> list[Disk]:
    tree = json.loads(subprocess.run(["lsblk", "-J", "-b", "-o", LSBLK_COLUMNS],
                                     capture_output=True, text=True, check=True).stdout)
    return from_lsblk(tree)

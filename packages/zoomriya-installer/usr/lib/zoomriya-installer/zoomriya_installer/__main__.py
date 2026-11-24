"""python3 -m zoomriya_installer: the installer's server, for its screen.

    ZOOMRIYA_INSTALLER_FAKE=1 python3 -m zoomriya_installer     # without writing anything
    python3 -m zoomriya_installer --list                        # the drives, as text
    python3 -m zoomriya_installer --install /dev/vdb            # no screen (the VM test)
"""

import os
import sys
from types import SimpleNamespace


def backend() -> SimpleNamespace:
    if os.environ.get("ZOOMRIYA_INSTALLER_FAKE"):
        from . import fake as chosen
    else:
        from . import disks, install

        chosen = SimpleNamespace(list_disks=disks.list_disks, install=install.install,
                                 online=install.online, restart=install.restart,
                                 power_off=install.power_off)
    return SimpleNamespace(list_disks=chosen.list_disks, install=chosen.install,
                           online=chosen.online, restart=chosen.restart,
                           power_off=chosen.power_off)


def main() -> int:
    args = sys.argv[1:]
    if args[:1] in (["--list"], ["--install"]):
        from .disks import size_text
        from .install import STAGES, InstallError, overall

        disks = backend().list_disks()
        if args[0] == "--list":
            for disk in disks:
                print(f"{disk.path}\t{size_text(disk.size)}\t{disk.bus}\t{disk.title}\t{disk.reason}")
            return 0
        disk = next((d for d in disks if d.path == args[1:2][0]), None) if args[1:] else None
        if disk is None or not disk.usable:
            print(disk.reason if disk else "Name a drive from --list.", file=sys.stderr)
            return 2

        def progress(stage: int, fraction: float) -> None:
            print(f"{overall(stage, fraction):4.0%} {STAGES[stage][0]}", flush=True)

        try:
            backend().install(disk.path, progress)
        except InstallError as exc:
            print(exc, file=sys.stderr)
            return 1
        return 0

    from .server import serve

    serve(backend())
    return 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
"""Build a validated ZIP with only distributable files, using the stdlib."""
import hashlib
import os
from pathlib import Path
import stat
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
INCLUDE = ("System", "Roms", "Imgs", "Emus", "Apps", "sources", "tools",
           "BRICK_PRO.md", "README.md", "LICENSE.txt", "do_release.sh")


def allowed(path):
    relative = path.relative_to(ROOT)
    return not (relative.parts[:2] == ("System", "backups")
                or "__pycache__" in relative.parts or path.name == ".DS_Store"
                or path.name.startswith("._") or path.suffix in (".pyc", ".log"))


def main():
    fd, temporary = tempfile.mkstemp(prefix=".release-", suffix=".zip", dir=ROOT)
    os.close(fd)
    try:
        with zipfile.ZipFile(temporary, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
            for name in INCLUDE:
                root = ROOT / name
                paths = sorted(root.rglob("*")) if root.is_dir() else [root]
                for path in paths:
                    if not allowed(path) or path.is_dir():
                        continue
                    relative = str(path.relative_to(ROOT))
                    if path.is_symlink():
                        info = zipfile.ZipInfo(relative)
                        info.create_system = 3
                        info.external_attr = (stat.S_IFLNK | 0o777) << 16
                        archive.writestr(info, os.readlink(path))
                    else:
                        archive.write(path, relative)
        with zipfile.ZipFile(temporary) as archive:
            failed = archive.testzip()
            if failed is not None:
                raise RuntimeError(f"ZIP integrity check failed: {failed}")
            if any(name.startswith("tests/") for name in archive.namelist()):
                raise RuntimeError("release ZIP must not contain tests")
            for name in ("System/bin/bash", "System/bin/bash.real", "System/bin/setterm",
                         "Emus/PORTS/launch_balanced.sh", "Apps/ROMSearch/launch.sh"):
                assert archive.getinfo(name).external_attr >> 16 & 0o111, name
            for name in ("Apps/ROMSearch/app.py", "Apps/ROMSearch/rom_sources.py",
                         "Apps/ROMSearch/config.json", "Apps/ROMSearch/icon.png"):
                archive.getinfo(name)
            count = len(archive.namelist())
        output = ROOT / "TRIMUI_EX.zip"
        os.replace(temporary, output)
        sha = hashlib.sha256(output.read_bytes()).hexdigest()
        print(f"Validated {count} files; {output.stat().st_size} bytes")
        print(f"{sha}  {output.name}")
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


if __name__ == "__main__":
    main()

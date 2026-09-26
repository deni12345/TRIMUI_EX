#!/usr/bin/env python3
"""Build device and corresponding-source ZIPs from this checkout."""
import hashlib
import os
from pathlib import Path
import stat
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
DEVICE_DIRS = ("System", "Roms", "Imgs", "Emus", "Apps")
ROM_SEARCH_FILES = {
    "bin/7zzs", "bin/7zip-LICENSE.txt", "bin/azuretls-LICENSE.txt",
    "bin/purego-LICENSE.txt", "bin/ximage-LICENSE.txt",
    "bin/xnet-LICENSE.txt", "config.json", "icon.png", "launch.sh",
    "romsearch",
}
PYTHON_PAYLOAD = "System/updates/update_001/python.zip"
REQUIRED_DEVICE_FILES = (
    "System/bin/bash", "System/bin/bash.real", "System/bin/setterm",
    "System/bin/ex_install_brickpro.sh", PYTHON_PAYLOAD,
    "Emus/PORTS/launch_balanced.sh", "Apps/ROMSearch/launch.sh",
    "Apps/ROMSearch/bin/7zzs", "Apps/ROMSearch/romsearch",
)
EXECUTABLE_FILES = (
    "System/bin/bash", "System/bin/bash.real", "System/bin/setterm",
    "Emus/PORTS/launch_balanced.sh", "Apps/ROMSearch/launch.sh",
    "Apps/ROMSearch/bin/7zzs", "Apps/ROMSearch/romsearch",
)


def device_file(path):
    relative = path.relative_to(ROOT)
    if (relative.parts[:2] == ("System", "backups")
            or "__pycache__" in relative.parts
            or path.name == ".DS_Store" or path.name.startswith("._")
            or path.suffix in (".pyc", ".log") or path.name == ".gitkeep"):
        return False
    if relative.parts[:2] == ("Apps", "ROMSearch"):
        return str(Path(*relative.parts[2:])) in ROM_SEARCH_FILES
    return True


def python_file(name):
    # Every cached .pyc in this payload has a matching .py source file.
    return ("__pycache__" not in name.split("/")
            and not name.startswith("lib/python3.11/test/"))


def add_file(archive, path, name):
    if path.is_symlink():
        info = zipfile.ZipInfo(name)
        info.create_system = 3
        info.external_attr = (stat.S_IFLNK | 0o777) << 16
        archive.writestr(info, os.readlink(path))
    else:
        archive.write(path, name)


def build_python_payload(output):
    with zipfile.ZipFile(ROOT / PYTHON_PAYLOAD) as source, zipfile.ZipFile(
            output, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as target:
        if source.testzip() is not None:
            raise RuntimeError("source Python payload is corrupt")
        for entry in source.infolist():
            if python_file(entry.filename):
                target.writestr(entry, source.read(entry))
    with zipfile.ZipFile(output) as archive:
        if archive.testzip() is not None:
            raise RuntimeError("repacked Python payload is corrupt")
        for name in ("bin/python3", "lib/python3.11/os.py",
                     "lib/python3.11/ssl.py", "lib/python3.11/site-packages/pip/__init__.py"):
            archive.getinfo(name)


def validate_device(output):
    with zipfile.ZipFile(output) as archive:
        if archive.testzip() is not None:
            raise RuntimeError("device ZIP is corrupt")
        names = set(archive.namelist())
        for name in REQUIRED_DEVICE_FILES:
            archive.getinfo(name)
        for name in EXECUTABLE_FILES:
            assert archive.getinfo(name).external_attr >> 16 & 0o111, name
        if any(name.startswith(("sources/", "tools/")) or name.endswith(".go")
               or name.endswith("_test.go") or "__pycache__" in name
               for name in names):
            raise RuntimeError("development files in device ZIP")
        if any(name.startswith("Apps/ROMSearch/") and name.split("Apps/ROMSearch/")[1]
               not in ROM_SEARCH_FILES for name in names):
            raise RuntimeError("unexpected ROM Search file in device ZIP")
        return len(names)


def describe(path, count):
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    print(f"Validated {count} entries; {path.stat().st_size} bytes")
    print(f"{digest.hexdigest()}  {path.name}")


def main():
    with tempfile.TemporaryDirectory(prefix=".release-", dir=ROOT) as staging:
        staging = Path(staging)
        python_payload = staging / "python.zip"
        device_zip = staging / "TRIMUI_EX.zip"
        source_zip = staging / "TRIMUI_EX-sources.zip"
        build_python_payload(python_payload)
        with zipfile.ZipFile(device_zip, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
            for directory in DEVICE_DIRS:
                for path in sorted((ROOT / directory).rglob("*")):
                    if path.is_dir() or not device_file(path):
                        continue
                    name = path.relative_to(ROOT).as_posix()
                    add_file(archive, python_payload if name == PYTHON_PAYLOAD else path, name)
            for name in ("Imgs/PORTS/", "Roms/PORTS/"):
                archive.writestr(name, b"")
            add_file(archive, ROOT / "LICENSE.txt", "LICENSE.txt")
        count = validate_device(device_zip)
        with zipfile.ZipFile(source_zip, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
            for path in sorted((ROOT / "sources").rglob("*")):
                if path.is_file():
                    add_file(archive, path, path.relative_to(ROOT).as_posix())
            add_file(archive, ROOT / "LICENSE.txt", "LICENSE.txt")
        with zipfile.ZipFile(source_zip) as archive:
            if archive.testzip() is not None or not any(
                    n.startswith("sources/bash/") for n in archive.namelist()) or not any(
                    n.startswith("sources/util-linux/") for n in archive.namelist()):
                raise RuntimeError("corresponding-source ZIP is incomplete")
            source_count = len(archive.namelist())
        for staged in (device_zip, source_zip):
            os.replace(staged, ROOT / staged.name)
        describe(ROOT / device_zip.name, count)
        describe(ROOT / source_zip.name, source_count)


if __name__ == "__main__":
    main()

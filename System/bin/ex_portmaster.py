#!/usr/bin/env python3
"""Apply narrow, reversible PortMaster adaptations; never edit game scripts."""
import argparse
import ast
import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

BEGIN = "# TRIMUI_EX_BRICK_PRO_BEGIN"
END = "# TRIMUI_EX_BRICK_PRO_END"


def remove_block(text):
    if BEGIN not in text:
        return text
    if text.count(BEGIN) != 1 or text.count(END) != 1:
        raise ValueError("ambiguous TRIMUI_EX block")
    start = text.index(BEGIN)
    stop = text.index(END, start) + len(END)
    return text[:start] + text[stop:].lstrip("\n")


def transform(relative, original, fragments):
    base = remove_block(original)
    if relative.endswith("hardware.py"):
        tree = ast.parse(base)
        names = {n.name for n in tree.body if isinstance(n, ast.FunctionDef)}
        assignments = {t.id for n in tree.body if isinstance(n, ast.Assign)
                       for t in n.targets if isinstance(t, ast.Name)}
        if "new_device_info" not in names or not {"DEVICES", "HW_INFO"} <= assignments:
            raise ValueError("unsupported PortMaster hardware interface")
        # Append at EOF without accumulating blank lines on repeated runs.
        result = base.rstrip() + "\n\n" + fragments["hardware.py.inc"]
        ast.parse(result)
        return result
    if relative.endswith("platform.py"):
        tree = ast.parse(base)
        trimui = next((node for node in tree.body if isinstance(node, ast.ClassDef)
                       and node.name == "PlatformTrimUI"), None)
        if trimui is None or not any(isinstance(node, ast.FunctionDef)
                                     and node.name == "add_port_script" for node in trimui.body):
            raise ValueError("unsupported PortMaster TrimUI install interface")
        old = "target_file = ROM_SCRIPT_DIR / (port_script.name)\n            if not os.path.samefile(port_script, target_file):"
        fixed = "target_file = ROM_SCRIPT_DIR / (port_script.name)\n            if not target_file.exists() or not os.path.samefile(port_script, target_file):"
        if base.count(old) == 1 and fixed not in base:
            result = base.replace(old, fixed)
        elif base.count(fixed) == 1 and old not in base:
            result = base
        else:
            raise ValueError("unsupported PortMaster TrimUI launcher copy interface")
        ast.parse(result)
        return result
    if relative == "device_info.txt":
        # Stock TrimUI lacks lscpu; the model adapter supplies DEVICE_CPU below.
        base = base.replace("DEVICE_CPU=$(lscpu |", "DEVICE_CPU=$(command -v lscpu >/dev/null 2>&1 && lscpu |")
        anchor = "# GLIBC\n"
        if base.count(anchor) != 1:
            raise ValueError("unsupported PortMaster shell device interface")
        return base.replace(anchor, fragments["device_info.sh.inc"] + anchor)
    if relative == "control.txt":
        if "get_controls()" not in base or "device_info.txt" not in base:
            raise ValueError("unsupported PortMaster controls interface")
        return base.rstrip() + "\n\n" + fragments["control.sh.inc"]
    if relative == "../launch.sh":
        first, separator, rest = base.partition("\n")
        if not first.startswith("#!") or not separator:
            raise ValueError("PortMaster launcher has no shebang")
        shim = BEGIN + '''
# The stock frontend may explicitly invoke sh, regardless of the shebang.
if [ -z "${BASH_VERSION:-}" ]; then
    exec /mnt/SDCARD/System/bin/bash "$0" "$@"
fi
/mnt/SDCARD/System/bin/ex_portmaster.sh || exit 1
''' + END + "\n"
        return first + "\n" + shim + rest
    raise ValueError(relative)


def atomic_write(path, content):
    fd, tmp = tempfile.mkstemp(prefix=".ex-", dir=path.parent)
    try:
        with os.fdopen(fd, "w") as stream:
            stream.write(content)
            stream.flush()
            os.fsync(stream.fileno())
        os.chmod(tmp, path.stat().st_mode & 0o777)
        os.replace(tmp, path)
    finally:
        if os.path.exists(tmp):
            os.unlink(tmp)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--portmaster", type=Path, default=Path("/mnt/SDCARD/Apps/PortMaster/PortMaster"))
    parser.add_argument("--system", type=Path, default=Path("/mnt/SDCARD/System"))
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    fragments = {p.name: p.read_text() for p in (args.system / "lib/trimui-ex").glob("*.inc")}
    changes = []
    # Validate the entire set before making any writes; refuse unfamiliar layouts.
    for relative in ("pylibs/harbourmaster/hardware.py", "pylibs/harbourmaster/platform.py",
                     "device_info.txt", "control.txt", "../launch.sh"):
        path = args.portmaster / relative
        original = path.read_text()
        result = transform(relative, original, fragments)
        if not relative.endswith(".py"):
            subprocess.run([str(args.system / "bin/bash"), "-n"], input=result, text=True, check=True)
        if original != result:
            changes.append((path, original, result))
    if args.check:
        print(f"PortMaster interface validated; {len(changes)} files need adaptation")
        return
    for path, original, result in changes:
        # Content-addressed backups retain upstream versions across self-updates.
        digest = hashlib.sha256(original.encode()).hexdigest()
        backup = args.system / "backups/portmaster" / (path.name + "." + digest)
        backup.parent.mkdir(parents=True, exist_ok=True)
        if not backup.exists():
            shutil.copy2(path, backup)
        atomic_write(path, result)
        print(f"Adapted {path}")


if __name__ == "__main__":
    main()

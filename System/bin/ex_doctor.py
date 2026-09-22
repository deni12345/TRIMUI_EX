#!/usr/bin/env python3
"""Read-only diagnostics for the stock Brick Pro PortMaster environment."""
import ctypes
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import ssl
import urllib.request


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--https", action="store_true", help="also check the PortMaster release endpoint over verified HTTPS")
    args = parser.parse_args()
    report = {"model": Path("/etc/model").read_text().strip(),
              "firmware": Path("/etc/version").read_text().strip(),
              "bash": subprocess.check_output(["/bin/bash", "-c", 'printf "%s" "$BASH_VERSION"'], text=True),
              "openssl": ssl.OPENSSL_VERSION}
    # A normal SSH environment masked a startup SIGSEGV in the original static
    # Bash integration. Both public entry points must work without SHELL/HOME.
    report["bash_without_login_environment"] = {
        path: subprocess.check_output([path, "-c", 'printf "%s" "$BASH_VERSION"'],
                                      env={}, text=True)
        for path in ("/bin/bash", "/mnt/SDCARD/System/bin/bash")
    }
    pm = Path("/mnt/SDCARD/Apps/PortMaster/PortMaster")
    sys.path[:0] = [str(pm / "pylibs"), str(pm / "exlibs")]
    from harbourmaster.hardware import device_info
    report["portmaster"] = device_info()
    report["trusted_certificates"] = len(ssl.create_default_context().get_ca_certs())
    if args.https:
        url = "https://github.com/PortsMaster/PortMaster-GUI/releases/latest/download/version.json"
        with urllib.request.urlopen(url, timeout=25) as response:
            report["https_status"] = response.status
    report["libraries"] = {}
    for name in ("libSDL2-2.0.so.0", "libSDL-1.2.so.0", "libEGL.so.1", "libGLESv2.so.2", "libasound.so.2"):
        try:
            ctypes.CDLL(name)
            report["libraries"][name] = "loadable"
        except OSError as error:
            report["libraries"][name] = str(error)
    sdl = ctypes.CDLL("/usr/trimui/lib/libSDL2-2.0.so.0")
    sdl.SDL_GetError.restype = ctypes.c_char_p
    sdl.SDL_JoystickOpen.restype = ctypes.c_void_p
    sdl.SDL_JoystickNameForIndex.restype = ctypes.c_char_p
    for name in ("SDL_JoystickNumAxes", "SDL_JoystickNumButtons", "SDL_JoystickNumHats", "SDL_JoystickClose"):
        getattr(sdl, name).argtypes = [ctypes.c_void_p]
    if sdl.SDL_Init(0x200 | 0x2000) != 0:
        raise RuntimeError(sdl.SDL_GetError().decode())
    try:
        report["controllers"] = []
        for index in range(sdl.SDL_NumJoysticks()):
            joystick = sdl.SDL_JoystickOpen(index)
            if not joystick:
                raise RuntimeError(sdl.SDL_GetError().decode())
            try:
                report["controllers"].append({"name": sdl.SDL_JoystickNameForIndex(index).decode(),
                    "axes": sdl.SDL_JoystickNumAxes(joystick), "buttons": sdl.SDL_JoystickNumButtons(joystick),
                    "hats": sdl.SDL_JoystickNumHats(joystick), "gamecontroller": bool(sdl.SDL_IsGameController(index))})
            finally:
                sdl.SDL_JoystickClose(joystick)
    finally:
        sdl.SDL_Quit()
    print(json.dumps(report, indent=2))
    if report["model"] == "TG4040":
        assert report["portmaster"]["device"] == "trimui-brick-pro"
        assert report["portmaster"]["analogsticks"] == 2
        assert any(c["gamecontroller"] and c["axes"] >= 4 for c in report["controllers"])
    assert report["bash"], "Bash is not installed"
    assert report["trusted_certificates"] > 0, "Python has no trusted CA certificates"
    assert all(value == "loadable" for value in report["libraries"].values())


if __name__ == "__main__":
    main()

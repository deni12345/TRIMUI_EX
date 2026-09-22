# TrimUI Brick Pro support

This branch targets **TG4040 with stock firmware**. The development device ran
firmware 1.1.2, kernel 4.9.191, glibc 2.33 and PortMaster 2026.06.23-0015.
TG3040 is the original Brick; it has the same display resolution but different
controls. Do not flash a TG3040 or TG5040 firmware image onto a TG4040.

## What changes

- PortMaster's Python and shell detection use `/etc/model`, reporting Brick Pro,
  1024×768, two analog sticks and AArch64. Existing game scripts are not rewritten.
- The launcher runs genuine GNU Bash, preserves arguments and exit codes, and
  restores the previous CPU limits/governor after normal exit or a handled signal.
  Frequency choices are constrained to the kernel's supported values; it does
  not overclock the device.
- Vendor SDL is preferred, while the supplied user-space libraries provide the
  OpenSSL version required by Python. A trusted CA bundle is configured without
  disabling certificate checks. Stock EGL/GLES drivers are used.
- The vendor Xbox-compatible controller mapping supplies both sticks and triggers.
  The old launch splash and in-place game-script delay insertion are not used on
  TG4040. The absent `oga_events` restart is handled only for that specific call.
- `/usr/bin/retroarch` starts stock `ra64.trimui` under the process name expected
  by input helpers, using SDL2 input and without saving over the shared config.
- Genuine `setterm` supplies the input helper's terminal-control dependency.
- The TG4040 startup path bypasses legacy updates that replace BusyBox.

## Install or update

Back up the device's internal firmware and existing SD software first. This is
an SD compatibility layer with a small internal integration; it does **not**
require reflashing the firmware or formatting the SD card.

Extract `TRIMUI_EX.zip` onto the SD card, preserving the existing game data and
PortMaster installation. Do not merely place the ZIP at the SD root: the legacy
automatic ZIP updater is intentionally bypassed on TG4040. Then run:

```sh
sh /mnt/SDCARD/System/bin/ex_install_brickpro.sh
```

The existing `System/starts/ex_init.sh` also calls the installer at startup.
For a fresh SD setup, put the official `trimui.portmaster.zip` at its root; the
installer can unpack it and the repository's Python payload. Fresh installations
and firmware versions other than the development device's configuration have
not been hardware-tested.

The installer saves original internal files once in
`/etc/trimui-ex-brickpro-backup`. Its internal changes are:

| Path | Purpose |
| --- | --- |
| `/usr/local/bin/trimui-ex-bash` | Genuine Bash stored internally |
| `/bin/bash` | Link to that interpreter; `/bin/sh` is preserved |
| `/usr/bin/retroarch` | Link to the SD wrapper, only if previously absent |
| `/usr/local/lib/trimui-ex/retroarch` | Named link to the stock SD binary |
| `/roms/ports/PortMaster/control.txt` | Updated fallback for ordinary port scripts |

PortMaster adapters validate known interfaces before writing and retain
content-addressed originals in `System/backups/portmaster`. They are idempotent
and reapplied at game launch, GUI launch and startup. If an upstream update
changes the expected interfaces, installation/launch fails with an explicit
error instead of silently corrupting the new files. A PortMaster update that
replaces its GUI launcher requires a restart through the stock startup hook or
running the installer again. Arbitrary future PortMaster versions are not guaranteed.

## Validation

Local regression checks:

```sh
python3 -m unittest discover -s tests -v
```

On-device checks, including a real certificate-verified HTTPS request:

```sh
. /mnt/SDCARD/System/etc/ex_config
python3 /mnt/SDCARD/System/bin/ex_doctor.py --https
```

Hardware checks cover Bash, model/capabilities, controller enumeration, library
loading, HTTPS and bounded launches of PortMaster, 2048 and Alien Blaster.
They do not establish that every PortMaster game works or validate physical
buttons, stick calibration, rumble, Bluetooth, suspend/resume, or long play sessions.
The firmware exposes no ARMHF loader on the tested device; games that require an
unavailable architecture/runtime still need an appropriate build/runtime.

## Rollback and firmware recovery

Keep the pre-change SD archive and raw firmware backup outside the device.
Restore the affected SD files from that archive first, including
`System/etc/ex_config`, `System/bin/ex_update.sh`, `System/starts/ex_init.sh`,
`Emus/PORTS/`, and the four adapted PortMaster files (`launch.sh`, `control.txt`,
`device_info.txt`, `pylibs/harbourmaster/hardware.py`). Remove added compatibility
files using the deployment manifest, checking for subsequent edits first. This
prevents the new startup hook from reinstalling the integration.

The internal backup uses the keys `bash`, `retroarch`, `control`, `bash_binary`
and `retroarch_binary` for the paths in the table. A `.absent` file means that
path did not exist before installation; otherwise the key itself holds the
original file/symlink. Restore an original via a temporary adjacent path and
rename it over the installed path, preserving symlinks. Only remove paths marked
absent when they still contain this integration's files. Do not restore game saves
over newer progress merely to undo the launcher changes.

The raw internal eMMC image is **not** an Allwinner `.awimg` update package.
It must not be fed to the stock updater or written over a mounted live root disk.
The live backup's writable partitions are not atomic snapshots; a full recovery
requires the correct TG4040 recovery environment and storage layout. Firmware
restoration has not been tested. Kernel, bootloader and vendor GPU libraries are
not modified by this integration.

## Binary sources and packaging

New Bash and setterm binaries have package/binary hashes and licensing information
under `System/share/licenses/`. Exact corresponding source and Debian build
patches are under `sources/` and included in the release ZIP.

Run `sh do_release.sh` to build and validate `TRIMUI_EX.zip`. Backups, logs and
Python caches are excluded from the package.

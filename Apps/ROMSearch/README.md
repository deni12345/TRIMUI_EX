# ROM Search for TrimUI Brick Pro

The app appears under Apps after `Apps/ROMSearch` is copied to the SD card.
It uses the Python and SDL libraries in TRIMUI_EX. Use the D-pad and A to type,
Start to search, X to switch sites, and A on a result to download. Select exits.
Downloaded archives are saved to the corresponding existing folder under
`/mnt/SDCARD/Roms`. The app also fetches the game's image and writes a
matching PNG under `/mnt/SDCARD/Imgs/<system>/` for the stock menu. Pressing A
on an already downloaded game can retry a missing thumbnail. Existing ROMs and
images are not overwritten; archives are not extracted.

Both sources were searched and their small sample ZIP files downloaded to a
temporary directory on a TG4040 on 2026-09-23. Both samples passed ZIP format
checks. Controller input and live results were also checked on the device.
Thumbnail download and PNG conversion passed for one game from each source.
The game's runtime compatibility with a particular emulator has not been
verified. These sites may change at any time. The app only supports folder
mappings in `rom_sources.py` and only downloads files you are permitted to use.

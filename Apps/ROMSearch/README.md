# ROM Search for TrimUI Brick Pro

ROM Search now runs as a Go arm64 executable. It starts with a list of PSP games from RomsGames, so a search is optional. L/R switches installed emulators, X switches CoolROM, RomsFun, and RomsGames, left/right changes catalog pages, and A installs the selected game. Y opens the on-screen keyboard; enter a title and press START to search. SELECT exits. The selected emulator filters every source's results before installation.

The app reads each emulator's configured `rompath` and saves games under the matching `Roms/<system>/` folder. Cartridge and disc archives are extracted to playable files, keeping companion disc tracks together. Arcade ZIP sets remain zipped. A matching PNG thumbnail is saved under `Imgs/<system>/`. Unclear or unsupported archive contents are rejected. Existing games are kept.

Downloads use four HTTP byte ranges when the source supports them, with a single-stream fallback. When there is room, downloads and extraction stage on the device's internal UDISK storage; finished files are copied once to the SD card. The bundled arm64 7-Zip extractor is used for ZIP, 7z, and RAR.

Build from this directory with:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o romsearch .
```

The Go UI loads the device's SDL2 and SDL2_gfx libraries at runtime. Source sites can change or block automated requests. RomsGames' current download endpoint sometimes returns HTTP 500; the app reports that error and does not install an invalid file. Only download files you are permitted to use.

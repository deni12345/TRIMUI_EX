# ROM Search for TrimUI Brick Pro

ROM Search runs as a Go arm64 executable. It opens with a mixed game grid, showing cover art and each game's emulator badge. The initial RomsGames featured list is bundled as metadata, so it appears without crawling the website. Covers are fetched in the background and cached on the device's internal storage. The search field is visible above the grid. D-pad or left stick moves between games, L/R moves a screen at a time, A installs the selected game, and X switches CoolROM, RomsFun, and RomsGames. Y opens a QWERTY keyboard; move with the D-pad, type with A, erase with B, and press START or the on-screen GO key to search. The keyboard also has a 123 page, space bar, and delete key. SELECT exits. Loaded lists stay cached for instant revisits during that app session.

An install runs in a separate background process. A progress bar remains visible while downloading, and you can continue browsing or leave the app. Reopening ROM Search shows the running download or its result. One install can run at a time.

The app reads each emulator's configured `rompath` and saves games under the matching `Roms/<system>/` folder. Cartridge and disc archives are extracted to playable files, keeping companion disc tracks together. Arcade ZIP sets remain zipped. A matching PNG thumbnail is saved under `Imgs/<system>/`. Unclear or unsupported archive contents are rejected. Existing games are kept.

Downloads use four HTTP byte ranges when the source supports them, with a single-stream fallback. When there is room, downloads and extraction stage on the device's internal UDISK storage; finished files are copied once to the SD card. The bundled arm64 7-Zip extractor is used for ZIP, 7z, and RAR.

Build from this directory with:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o romsearch .
```

The Go UI loads the device's SDL2 and SDL2_gfx libraries at runtime. Source sites can change or block automated requests. RomsGames' current download endpoint sometimes returns HTTP 500; the app reports that error and does not install an invalid file. Only download files you are permitted to use.

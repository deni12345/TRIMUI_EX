# ROM Search for TrimUI Brick Pro

ROM Search runs as a Go arm64 executable. It opens with a mixed game grid, showing cover art and each game's emulator badge. The initial RomsGames featured list is bundled as metadata, so it appears without crawling the website. When browsing reaches the end of loaded games, the app fetches another emulator catalog page in the background and adds its games to the grid. The `+` after the page number means more games may be available. Covers are fetched in the background and cached on the device's internal storage. The search field is visible above the grid. D-pad or left stick moves between games, L/R moves a screen at a time, A installs the selected game, and X switches CoolROM, RomsFun, and RomsGames. SELECT opens and closes the search keyboard; move with the D-pad, type with A, erase with B, and choose the on-screen GO key to search. Y also opens or closes the keyboard. The keyboard has a 123 page, space bar, and delete key. START exits. Loaded lists stay cached for instant revisits during that app session.

An install runs in a separate background process. A progress bar remains visible while downloading, and you can continue browsing or leave the app. Reopening ROM Search shows the running download or its result. One install can run at a time.

The app reads each emulator's configured `rompath` and saves games under the matching `Roms/<system>/` folder. Cartridge and disc archives are extracted to playable files, keeping companion disc tracks together. Arcade ZIP sets remain zipped. A matching PNG thumbnail is saved under `Imgs/<system>/`. Unclear or unsupported archive contents are rejected. Existing games are kept.

Downloads use four HTTP byte ranges when the source supports them, with a single-stream fallback. When there is room, the compressed download is staged on the device's internal UDISK storage. Playable files are extracted directly to the SD card's emulator folder, avoiding an extra full-size copy. The bundled arm64 7-Zip extractor is used for ZIP, 7z, and RAR.

RomsFun pages and file links use a browser-compatible Go TLS and HTTP/2 client. The app resolves each signed download link on the device, then streams the file from its approved host. RomsFun WebP covers are converted to PNG for the game grid and installed thumbnail.

Build from this directory with:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o romsearch .
```

The Go UI loads the device's SDL2 and SDL2_gfx libraries at runtime. Source sites can change or block automated requests. RomsGames' current download endpoint sometimes returns HTTP 500; the app reports that error and does not install an invalid file. Only download files you are permitted to use.

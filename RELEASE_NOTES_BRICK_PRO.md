# Brick Pro (TG4040) compatibility release

This release adds PortMaster support for the TrimUI Brick Pro on stock firmware.
The tested device used firmware 1.1.2 and PortMaster 2026.06.23-0015.

The compatibility layer supplies real Bash, required runtime libraries, model
and controller detection, and a launcher that returns CPU settings after a game
exits. Tested ready-to-play ports launched from the stock Games menu and
returned to MainUI. It also fixes PortMaster's first-time install error when
the launcher destination in `Roms/PORTS` does not exist yet. Existing game
scripts are not rewritten.

The tested 2048 black-screen-and-exit failure was caused by MainUI omitting
`SHELL`: the static Bash interpreter crashed before the game started. Both Bash
entry points now initialize `SHELL` before invoking that interpreter. This is
separate from the newly downloaded launcher fix.

Download `TRIMUI_EX.zip` and follow the Brick Pro instructions in
[README.md](README.md). Extract the ZIP onto the SD card; do not flash a
TG3040 or TG5040 firmware image. The release ZIP excludes tests; the source
repository retains them.

The tests and hardware checks cover the compatibility layer and selected
ready-to-play ports. Individual ports may require other game data or runtimes.

## Lean SD card package update

The device ZIP now contains runtime files only. It omits ROM Search Go sources,
build scripts, repository documentation, and Python test and bytecode cache
files. The Python installer still contains Python modules, pip and SSL support.
Corresponding Bash and setterm sources are supplied as the separate
`TRIMUI_EX-sources.zip` release asset. ROM Search includes the current
controller, catalog pagination and direct extraction updates.

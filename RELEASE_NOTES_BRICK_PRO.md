# Brick Pro (TG4040) compatibility release

This release adds PortMaster support for the TrimUI Brick Pro on stock firmware.
The tested device used firmware 1.1.2 and PortMaster 2026.06.23-0015.

The compatibility layer supplies real Bash, required runtime libraries, model
and controller detection, and a launcher that returns CPU settings after a game
exits. Tested ready-to-play ports launched from the stock Games menu and
returned to MainUI. It also fixes PortMaster's first-time install error when
the launcher destination in `Roms/PORTS` does not exist yet. Existing game
scripts are not rewritten.

Download `TRIMUI_EX.zip` and follow the Brick Pro instructions in
[README.md](README.md). Extract the ZIP onto the SD card; do not flash a
TG3040 or TG5040 firmware image. The release ZIP excludes tests; the source
repository retains them.

The tests and hardware checks cover the compatibility layer and selected
ready-to-play ports. Individual ports may require other game data or runtimes.

#!/bin/sh
# PortMaster ports expect /usr/bin/retroarch. Stock TrimUI ships it on the SD.
ra_dir=/mnt/SDCARD/RetroArch
[ -x "$ra_dir/ra64.trimui" ] || { echo "Stock RetroArch not found" >&2; exit 1; }
cd "$ra_dir" || exit 1
exec /usr/local/lib/trimui-ex/retroarch --config "$ra_dir/retroarch.cfg" \
    --appendconfig /mnt/SDCARD/System/etc/retroarch-ports.cfg "$@"

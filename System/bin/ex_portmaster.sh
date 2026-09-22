#!/bin/sh
. "${EX_SYSTEM_PATH:-/mnt/SDCARD/System}/etc/ex_config" || exit 1
[ "$EX_MODEL" = TG4040 ] || exit 0
"$EX_SYSTEM_PATH/bin/python3" "$EX_SYSTEM_PATH/bin/ex_portmaster.py" --system "$EX_SYSTEM_PATH"

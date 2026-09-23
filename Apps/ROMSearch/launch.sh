#!/bin/sh
: > /tmp/rom-search.log
exec >> /tmp/rom-search.log 2>&1
. /mnt/SDCARD/System/etc/ex_config || exit 1
cd /mnt/SDCARD/Apps/ROMSearch || exit 1
exec /mnt/SDCARD/System/bin/python3 app.py

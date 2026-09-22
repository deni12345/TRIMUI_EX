#!/bin/sh
export EX_CPU_PROFILE='Battery Saver'
exec "$(dirname "$0")/launch.sh" "$@"

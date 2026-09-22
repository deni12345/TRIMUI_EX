#!/bin/sh
export EX_CPU_PROFILE='High Performance'
exec "$(dirname "$0")/launch.sh" "$@"

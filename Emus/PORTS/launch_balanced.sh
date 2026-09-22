#!/bin/sh
export EX_CPU_PROFILE='Balanced'
exec "$(dirname "$0")/launch.sh" "$@"

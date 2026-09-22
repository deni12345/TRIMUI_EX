#!/bin/sh
. "${EX_SYSTEM_PATH:-/mnt/SDCARD/System}/etc/ex_config" || exit 1
EMU_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd) || exit 1
[ "$#" -gt 0 ] && [ -f "$1" ] || { echo "Port script not found: ${1:-<none>}" >&2; exit 1; }
case "$1" in
    /*) port=$1 ;;
    *) port="$PWD/$1" ;;
esac
shift
# Old TRIMUI_EX aliases /bin/bash to ash. Never silently run Bash ports with it.
if ! "$EX_BASH" -c 'test -n "$BASH_VERSION"' 2>/dev/null; then
    echo "TRIMUI_EX requires genuine Bash at $EX_BASH. Run the installer." >&2
    exit 1
fi
"$EX_SYSTEM_PATH/bin/ex_portmaster.sh" || exit 1
cpu_state=$("$EMU_DIR/cpufreq.sh" save)
restore_cpu() {
    if [ -n "$cpu_state" ]; then
        # State contains exactly three validated numeric/governor fields.
        "$EMU_DIR/cpufreq.sh" restore $cpu_state
    fi
}
trap restore_cpu EXIT
trap 'exit 130' INT
trap 'exit 143' TERM HUP
"$EMU_DIR/cpufreq.sh" "${EX_CPU_PROFILE:-Balanced}" || exit 1
cd "${EX_PORTS_PATH:-/mnt/SDCARD/Roms/PORTS}" || exit 1
"$EX_BASH" "$port" "$@"
exit $?

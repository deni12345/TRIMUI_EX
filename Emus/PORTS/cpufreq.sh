#!/bin/sh
# Restrict requests to the frequencies and governors offered by the kernel.
cpu=${EX_CPUFREQ_PATH:-/sys/devices/system/cpu/cpu0/cpufreq}
[ -d "$cpu" ] || exit 0
read -r old_min < "$cpu/scaling_min_freq" || exit 1
read -r old_max < "$cpu/scaling_max_freq" || exit 1
read -r old_gov < "$cpu/scaling_governor" || exit 1
case "$1" in
    save) printf '%s %s %s\n' "$old_min" "$old_max" "$old_gov"; exit 0 ;;
    restore) [ "$#" -eq 4 ] || exit 1; minimum=$2; maximum=$3; governor=$4 ;;
    'High Performance') minimum=1008000; maximum=1800000; governor=performance ;;
    Balanced) minimum=600000; maximum=1608000; governor=ondemand ;;
    'Battery Saver') minimum=408000; maximum=1200000; governor=conservative ;;
    *) echo "Unknown CPU profile: $1" >&2; exit 1 ;;
esac
case "$minimum:$maximum" in *[!0-9:]*|:*|*:) exit 1 ;; esac
governors=$(cat "$cpu/scaling_available_governors") || exit 1
case " $governors " in *" $governor "*) ;; *) governor=$old_gov ;; esac
frequencies=$(cat "$cpu/scaling_available_frequencies") || exit 1
pick_frequency() {
    printf '%s\n' "$frequencies" | awk -v target="$1" '
        { for (i=1; i<=NF; i++) if ($i ~ /^[0-9]+$/) {
            if (low == 0 || $i < low) low=$i;
            if ($i <= target && $i > best) best=$i;
        }} END { if (low) print (best ? best : low); else exit 1 }'
}
minimum=$(pick_frequency "$minimum") || exit 1
maximum=$(pick_frequency "$maximum") || exit 1
[ "$minimum" -le "$maximum" ] || exit 1
# Expand bounds before narrowing, avoiding min > max transient writes.
if [ "$minimum" -gt "$old_max" ]; then
    printf '%s\n' "$maximum" > "$cpu/scaling_max_freq" || exit 1
    printf '%s\n' "$minimum" > "$cpu/scaling_min_freq" || exit 1
else
    printf '%s\n' "$minimum" > "$cpu/scaling_min_freq" || exit 1
    printf '%s\n' "$maximum" > "$cpu/scaling_max_freq" || exit 1
fi
printf '%s\n' "$governor" > "$cpu/scaling_governor"

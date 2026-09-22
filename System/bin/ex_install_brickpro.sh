#!/bin/sh
set -eu
. "${EX_SYSTEM_PATH:-/mnt/SDCARD/System}/etc/ex_config"
[ "$EX_MODEL" = TG4040 ] || { echo 'This installer requires TG4040.' >&2; exit 1; }
[ "$(uname -m)" = aarch64 ] || exit 1
# Validate the menu's missing-login-variable case, not the SSH environment.
env -i EX_SYSTEM_PATH="$EX_SYSTEM_PATH" "$EX_BASH" -c 'test -n "$BASH_VERSION"'

# Extract the existing Python payload only when this is a fresh SD install.
if [ ! -x "$EX_SYSTEM_PATH/bin/python3" ]; then
    unzip -o "$EX_SYSTEM_PATH/updates/update_001/python.zip" -d "$EX_SYSTEM_PATH"
fi
"$EX_SYSTEM_PATH/bin/python3" --version
pm=/mnt/SDCARD/Apps/PortMaster/PortMaster
if [ ! -f "$pm/control.txt" ]; then
    [ -f /mnt/SDCARD/trimui.portmaster.zip ] || {
        echo 'PortMaster is missing; place trimui.portmaster.zip at the SD root.' >&2
        exit 1
    }
    unzip -o /mnt/SDCARD/trimui.portmaster.zip -d /mnt/SDCARD
fi
# Validate all adapters before touching system paths.
"$EX_SYSTEM_PATH/bin/python3" "$EX_SYSTEM_PATH/bin/ex_portmaster.py" --system "$EX_SYSTEM_PATH" --check

backup=/etc/trimui-ex-brickpro-backup
mkdir -p "$backup" /usr/local/bin /usr/local/lib/trimui-ex /roms/ports/PortMaster
save_original() {
    key=$1
    path=$2
    if [ ! -e "$backup/$key.saved" ]; then
        if [ -e "$path" ] || [ -L "$path" ]; then
            cp -a "$path" "$backup/$key"
        else
            touch "$backup/$key.absent"
        fi
        touch "$backup/$key.saved"
    fi
}
save_original bash /bin/bash
save_original retroarch /usr/bin/retroarch
save_original control /roms/ports/PortMaster/control.txt
save_original bash_binary /usr/local/bin/trimui-ex-bash
save_original retroarch_binary /usr/local/lib/trimui-ex/retroarch

# Keep the shebang interpreter on internal storage so removing the SD never
# leaves /bin/bash pointing at an unavailable card. /bin/sh and busybox stay stock.
if ! cmp -s "$EX_SYSTEM_PATH/bin/bash.real" /usr/local/bin/trimui-ex-bash; then
    cp "$EX_SYSTEM_PATH/bin/bash.real" /usr/local/bin/trimui-ex-bash.new
    chmod 755 /usr/local/bin/trimui-ex-bash.new
    mv /usr/local/bin/trimui-ex-bash.new /usr/local/bin/trimui-ex-bash
fi
if ! cmp -s "$EX_SYSTEM_PATH/lib/trimui-ex/bash-internal.sh" /bin/bash; then
    cp "$EX_SYSTEM_PATH/lib/trimui-ex/bash-internal.sh" /bin/bash.trimui-ex-new
    chmod 755 /bin/bash.trimui-ex-new
    mv -f /bin/bash.trimui-ex-new /bin/bash
fi
if [ ! -e /usr/bin/retroarch ]; then
    ln -s "$EX_SYSTEM_PATH/bin/ex_retroarch.sh" /usr/bin/retroarch
fi
# gptokeyb and port cleanup scripts look for a process named "retroarch".
# Executing this link gives the vendor binary that name without copying it.
if [ ! -e /usr/local/lib/trimui-ex/retroarch ]; then
    ln -s /mnt/SDCARD/RetroArch/ra64.trimui /usr/local/lib/trimui-ex/retroarch
fi
# Forward the standard fallback path to the current SD control file, so a
# PortMaster self-update cannot leave game launchers using an obsolete copy.
"$EX_SYSTEM_PATH/bin/ex_portmaster.sh"
fallback="$EX_SYSTEM_PATH/lib/trimui-ex/fallback-control.sh"
if ! cmp -s "$fallback" /roms/ports/PortMaster/control.txt; then
    cp "$fallback" /roms/ports/PortMaster/control.txt.new
    mv /roms/ports/PortMaster/control.txt.new /roms/ports/PortMaster/control.txt
fi
echo 'TRIMUI_EX Brick Pro integration ready.'

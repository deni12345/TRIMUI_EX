# TRIMUI_EX_BRICK_PRO_BEGIN
if [ "${EX_MODEL:-}" = TG4040 ]; then
    systemctl() {
        # Port exit scripts restart this input service on other distributions.
        # Stock TrimUI has no oga_events service to restart.
        if [ "$#" -eq 2 ] && [ "$1" = restart ] && [ "$2" = oga_events ]; then
            return 0
        fi
        command systemctl "$@"
    }
    get_controls() {
        # Stock TG4040 exposes its pad with the Xbox 360 GUID. Preserve the
        # vendor mapping including both analog sticks, triggers and stick clicks.
        sdl_controllerconfig=$(grep -m 1 '^030000005e0400008e02000014010000,' /usr/trimui/gamecontrollerdb.txt)
        if [ -n "$sdl_controllerconfig" ]; then
            export SDL_GAMECONTROLLERCONFIG="$sdl_controllerconfig"
        fi
        export sdl_controllerconfig
    }
fi
# TRIMUI_EX_BRICK_PRO_END

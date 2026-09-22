# TRIMUI_EX_BRICK_PRO_BEGIN
if [ "$CFW_NAME" = TrimUI ] && [ "$(tr -d '\000\r\n' < /etc/model 2>/dev/null)" = TG4040 ]; then
    DEVICE_NAME="TrimUI Brick Pro"
    DEVICE_CPU=a133plus
    ANALOG_STICKS=2
    DISPLAY_WIDTH=1024
    DISPLAY_HEIGHT=768
    ASPECT_X=4
    ASPECT_Y=3
fi
# TRIMUI_EX_BRICK_PRO_END

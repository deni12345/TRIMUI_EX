# TRIMUI_EX_BRICK_PRO_BEGIN
# Keep detection independent of resolution: TG3040 and TG4040 share a panel.
DEVICES.setdefault("TrimUI Brick Pro", {
    "device": "trimui-brick-pro", "manufacturer": "TrimUI", "cfw": ["TrimUI"],
})
HW_INFO.setdefault("trimui-brick-pro", {
    "resolution": (1024, 768), "analogsticks": 2, "cpu": "a133plus",
    "capabilities": ["power"], "ram": 1024,
})
_ex_original_new_device_info = new_device_info

def new_device_info():
    info = _ex_original_new_device_info()
    if info.get("name") == "TrimUI" and safe_cat("/etc/model").strip().upper() == "TG4040":
        info["device"] = "trimui-brick-pro"
    return info
# TRIMUI_EX_BRICK_PRO_END

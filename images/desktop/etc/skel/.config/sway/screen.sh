#!/usr/bin/bash
if grep -q open /proc/acpi/button/lid/LID/state; then
    echo "enable Notebook display"
    swaymsg output ePD-1 enable
    swaymsg output eDP-1 pos 0 0 res 1920x1080 scale 1
else
    echo "disable Notebook display"
    swaymsg output eDP-1 disable
fi

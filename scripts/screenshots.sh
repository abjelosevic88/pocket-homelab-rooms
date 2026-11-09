#!/bin/sh
# README screenshots from a demo instance, with headless Chrome in Docker.
#
#   make build && DATA_DIR=/tmp/rooms-demo DEMO_DATA=true AUTH_MODE=none HOST=127.0.0.1 PORT=18265 ./pocket-rooms &
#   sh scripts/screenshots.sh            # → docs/screenshots/*.png
set -eu
BASE=${BASE:-http://127.0.0.1:18265}
OUT=$(cd "$(dirname "$0")/.." && pwd)/docs/screenshots
mkdir -p "$OUT"
shot() { # name path width height [dark]
  extra=""
  [ "${5:-}" = dark ] && extra="--force-dark-mode --blink-settings=preferredColorScheme=0"
  docker run --rm --network host -v "$OUT:/out" zenika/alpine-chrome --headless --no-sandbox --hide-scrollbars $extra \
    --virtual-time-budget=6000 --screenshot="/out/$1.png" --window-size="$3,$4" "$BASE$2" >/dev/null 2>&1
  echo "$1"
}
shot desktop-plans / 1440 950
shot desktop-editor /plans/1 1440 950
shot desktop-option-b '/plans/1?layout=2' 1440 950
shot desktop-3d '/plans/2?tab=3d' 1440 950
shot desktop-sizes '/plans/1?tab=sizes' 1440 1100
shot desktop-fit '/fit?piece=12&route=1' 1440 1150
shot desktop-way-in '/plans/1?tab=way' 1440 1000
shot desktop-furniture /furniture 1440 900
shot desktop-editor-dark /plans/2 1440 950 dark
shot phone-plans / 390 844
shot phone-editor /plans/1 390 844
shot phone-3d '/plans/1?tab=3d' 390 844

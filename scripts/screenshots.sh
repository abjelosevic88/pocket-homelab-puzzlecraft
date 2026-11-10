#!/bin/sh
# README screenshots from a demo instance, with headless Chrome in Docker.
#
#   make build && DATA_DIR=/tmp/puzzlecraft-demo DEMO_DATA=true AUTH_MODE=none HOST=127.0.0.1 PORT=18266 ./pocket-puzzlecraft &
#   sh scripts/screenshots.sh            # → docs/screenshots/*.png
set -eu
BASE=${BASE:-http://127.0.0.1:18266}
OUT=$(cd "$(dirname "$0")/.." && pwd)/docs/screenshots
mkdir -p "$OUT"
ADV=$(curl -s "$BASE/api/v1/adventures/1" | sed -n 's/.*"share_token":"\([0-9a-f]*\)".*/\1/p')
PZ=$(curl -s "$BASE/api/v1/puzzles/2" | sed -n 's/.*"share_token":"\([0-9a-f]*\)".*/\1/p')
shot() { # name path width height [dark]
  extra=""
  [ "${5:-}" = dark ] && extra="--force-dark-mode --blink-settings=preferredColorScheme=0"
  docker run --rm --network host -v "$OUT:/out" zenika/alpine-chrome --headless --no-sandbox --hide-scrollbars $extra \
    --virtual-time-budget=8000 --screenshot="/out/$1.png" --window-size="$3,$4" "$BASE$2" >/dev/null 2>&1
  echo "$1"
}
shot desktop-today / 1440 950
shot desktop-crossword /puzzles/1 1440 950
shot desktop-wordsearch '/puzzles/4?tab=play' 1440 950
shot desktop-sudoku '/puzzles/6?tab=play' 1440 950
shot desktop-adventure /adventures/1 1440 950
shot desktop-live '/adventures/1?tab=live' 1440 800
shot desktop-new /new 1440 950
shot desktop-share '/puzzles/2?tab=share' 1440 950
shot desktop-quiz /puzzles/14 1440 950
shot desktop-logic /puzzles/17 1440 950
shot desktop-today-dark / 1440 950 dark
shot phone-today / 390 844
shot phone-play "/p/$PZ" 390 844
shot phone-escape "/a/$ADV" 390 844

#!/bin/bash
# Spike harness (#37): (re)starts Goose desktop on a virtual display with a
# remote debugging port, so Playwright can attach over CDP; launching the
# packaged app through Playwright's Electron support times out.
#   GOOSE_DIR  the unpacked goose .deb (usr/lib/goose/Goose inside it)
#   GOOSE_HOME the HOME Goose runs with; its config is written on first start
#   GOOSE_MODE approve, smart_approve or unset (Goose's default, auto)
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
: "${GOOSE_DIR:?set GOOSE_DIR to the unpacked goose .deb}" "${GOOSE_HOME:?set GOOSE_HOME}"
config=$GOOSE_HOME/.config/goose/config.yaml
mkdir -p "$(dirname "$config")"
sed "s|@HARNESS@|$here|" "$here/goose-config.yaml" > "$config"
if [ -n "${GOOSE_MODE:-}" ]; then sed -i "1i GOOSE_MODE: $GOOSE_MODE" "$config"; fi
for pid in $(pgrep -x Goose || true); do kill "$pid"; done
# The old instance holds the debugging port until it has exited.
for _ in $(seq 1 30); do pgrep -x Goose > /dev/null || break; sleep 1; done
pgrep -f "Xvfb :91" > /dev/null || { Xvfb :91 -screen 0 1400x1000x24 > /dev/null 2>&1 & sleep 1; }
DISPLAY=:91 HOME=$GOOSE_HOME GOOSE_DISABLE_KEYRING=1 OPENAI_API_KEY=spike \
  nohup "$GOOSE_DIR/usr/lib/goose/Goose" --no-sandbox --disable-gpu --remote-debugging-port=9333 > "$GOOSE_HOME/goose-desktop.out" 2>&1 &
for _ in $(seq 1 60); do curl -s localhost:9333/json/version > /dev/null && break; sleep 1; done
curl -s localhost:9333/json/version | head -3

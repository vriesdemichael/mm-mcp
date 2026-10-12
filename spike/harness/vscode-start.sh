#!/bin/bash
# Spike harness (#37): (re)starts VS Code on a virtual display with a remote
# debugging port, the scripted model's extension and the spike server.
#   VSCODE_DIR  the unpacked VS Code linux-x64 archive (code inside it)
#   VSCODE_DATA the user data directory; settings and mcp.json are written there
# --password-store=basic: without an OS keyring VS Code shows a native dialog
# no CDP client can see, and the MCP server waits on it and never starts.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
: "${VSCODE_DIR:?set VSCODE_DIR to the unpacked VS Code}" "${VSCODE_DATA:?set VSCODE_DATA}"
mkdir -p "$VSCODE_DATA/User" "$VSCODE_DATA/workspace"
cp "$here/vscode-settings.json" "$VSCODE_DATA/User/settings.json"
if [ "${AUTO_APPROVE:-true}" = false ]; then sed -i 's/"chat.tools.autoApprove": true/"chat.tools.autoApprove": false/' "$VSCODE_DATA/User/settings.json"; fi
printf '{ "servers": { "mmmcpspike": { "type": "stdio", "command": "%s/mm-mcp-spike.sh", "args": [] } } }\n' "$here" > "$VSCODE_DATA/User/mcp.json"
for pid in $(pgrep -x code || true); do kill "$pid"; done
# The old instance holds the debugging port until it has exited.
for _ in $(seq 1 30); do pgrep -x code > /dev/null || break; sleep 1; done
pgrep -f "Xvfb :92" > /dev/null || { Xvfb :92 -screen 0 1600x1000x24 > /dev/null 2>&1 & sleep 1; }
DISPLAY=:92 nohup "$VSCODE_DIR/code" --no-sandbox --disable-gpu --password-store=basic --remote-debugging-port=9334 \
  --user-data-dir "$VSCODE_DATA" --extensions-dir "$VSCODE_DATA/extensions" --extensionDevelopmentPath="$here/vscode-model" \
  "$VSCODE_DATA/workspace" > "$VSCODE_DATA/vscode.out" 2>&1 &
for _ in $(seq 1 60); do curl -s localhost:9334/json/version > /dev/null && break; sleep 1; done
curl -s localhost:9334/json/version | head -3

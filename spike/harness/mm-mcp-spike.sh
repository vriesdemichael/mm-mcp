#!/bin/bash
# Spike harness (#37): starts the spike build of mm-mcp over stdio for a client
# under test. Clients such as Goose desktop do not pass a configured environment
# on, so the connection comes from .tmp/spike.env (MM_URL, MM_TOKEN), which is
# never committed. Every request the client sends is logged to SPIKE_LOG.
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
set -a
# shellcheck disable=SC1091
source "$root/.tmp/spike.env"
set +a
export MM_URL=${MM_URL:-http://localhost:8065}
export MM_MCP_ALLOW_WRITES=false
export SPIKE_LOG=${SPIKE_LOG:-$root/.tmp/spike-server.log}
exec "$root/.tmp/mm-mcp-spike" serve

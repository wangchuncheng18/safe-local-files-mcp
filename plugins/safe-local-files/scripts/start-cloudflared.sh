#!/usr/bin/env sh
set -eu

DATA_DIR=${XDG_CONFIG_HOME:-"$HOME/.config"}/safe-local-files
CONFIG_PATH=${SLFM_CONFIG:-"$DATA_DIR/config.json"}
TOKEN_FILE=${CLOUDFLARED_TOKEN_FILE:-"$DATA_DIR/cloudflare-tunnel-token"}

command -v cloudflared >/dev/null 2>&1 || { echo 'cloudflared is not installed' >&2; exit 1; }
test -r "$TOKEN_FILE" || { echo "Tunnel token file is missing: $TOKEN_FILE" >&2; exit 1; }
test -r "$CONFIG_PATH" || { echo "MCP config is missing: $CONFIG_PATH" >&2; exit 1; }
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PLUGIN_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
BIN=${BINARY_PATH:-"$PLUGIN_ROOT/bin/safe-local-files"}
test -x "$BIN" || { echo "MCP binary is missing: $BIN" >&2; exit 1; }
"$BIN" validate --config "$CONFIG_PATH" --require-cloudflare-read-only
"$(dirname "$0")/start.sh" "$CONFIG_PATH"
exec cloudflared tunnel run --token-file "$TOKEN_FILE"

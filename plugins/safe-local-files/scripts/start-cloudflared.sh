#!/usr/bin/env sh
set -eu

DATA_DIR=${XDG_CONFIG_HOME:-"$HOME/.config"}/safe-local-files
CONFIG_PATH=${SLFM_CONFIG:-"$DATA_DIR/config.json"}
TOKEN_FILE=${CLOUDFLARED_TOKEN_FILE:-"$DATA_DIR/cloudflare-tunnel-token"}

command -v cloudflared >/dev/null 2>&1 || { echo 'cloudflared is not installed' >&2; exit 1; }
test -r "$TOKEN_FILE" || { echo "Tunnel token file is missing: $TOKEN_FILE" >&2; exit 1; }
test -r "$CONFIG_PATH" || { echo "MCP config is missing: $CONFIG_PATH" >&2; exit 1; }
"$(dirname "$0")/start.sh" "$CONFIG_PATH"
exec cloudflared tunnel run --token-file "$TOKEN_FILE"

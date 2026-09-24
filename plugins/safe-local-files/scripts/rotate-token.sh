#!/usr/bin/env sh
set -eu

PLUGIN_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
DATA_DIR=${XDG_CONFIG_HOME:-"$HOME/.config"}/safe-local-files
TOKEN_FILE="$DATA_DIR/token"
mkdir -p "$DATA_DIR"
umask 077
if [ "${1:-}" = "--restart-http" ]; then
  "$PLUGIN_ROOT/scripts/stop.sh"
fi
TEMP_FILE="$DATA_DIR/token.new.$$"
trap 'rm -f "$TEMP_FILE"' EXIT HUP INT TERM
"$PLUGIN_ROOT/bin/safe-local-files" token > "$TEMP_FILE"
chmod 600 "$TEMP_FILE"
mv -f "$TEMP_FILE" "$TOKEN_FILE"
SAFE_LOCAL_FILES_TOKEN=$(cat "$TOKEN_FILE")
export SAFE_LOCAL_FILES_TOKEN
if command -v launchctl >/dev/null 2>&1; then
  launchctl setenv SAFE_LOCAL_FILES_TOKEN "$SAFE_LOCAL_FILES_TOKEN"
elif command -v systemctl >/dev/null 2>&1; then
  systemctl --user set-environment SAFE_LOCAL_FILES_TOKEN="$SAFE_LOCAL_FILES_TOKEN" 2>/dev/null || true
fi
if [ "${1:-}" = "--restart-http" ]; then
  "$PLUGIN_ROOT/scripts/start.sh"
fi
echo "HTTP token rotated. Restart HTTP clients to pick up the new value. The token was not printed."

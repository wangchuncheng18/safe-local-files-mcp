#!/usr/bin/env sh
set -eu

DATA_DIR=${XDG_CONFIG_HOME:-"$HOME/.config"}/safe-local-files
PID_FILE="$DATA_DIR/server.pid"
if [ ! -f "$PID_FILE" ]; then
  echo "Safe Local Files is not running."
  exit 0
fi
PID=$(cat "$PID_FILE")
if kill -0 "$PID" 2>/dev/null; then
  kill "$PID"
fi
rm -f "$PID_FILE"
echo "Safe Local Files stopped."

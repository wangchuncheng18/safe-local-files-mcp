#!/usr/bin/env sh
set -eu

PLUGIN_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
BIN=${BINARY_PATH:-"$PLUGIN_ROOT/bin/safe-local-files"}
DATA_DIR=${XDG_CONFIG_HOME:-"$HOME/.config"}/safe-local-files
CONFIG=${1:-"$DATA_DIR/config.json"}
TOKEN_FILE="$DATA_DIR/token"
PID_FILE="$DATA_DIR/server.pid"
mkdir -p "$DATA_DIR"

if [ ! -x "$BIN" ]; then
  echo "Binary not found: $BIN. Run scripts/build.sh or install a release archive." >&2
  exit 1
fi

if [ ! -f "$CONFIG" ]; then
  cat >"$CONFIG" <<EOF
{
  "root": "$PLUGIN_ROOT/example-data",
  "listen": "127.0.0.1",
  "port": 47381,
  "token_env": "SAFE_LOCAL_FILES_TOKEN",
  "audit_log": "$DATA_DIR/audit.jsonl",
  "allow_remote": false,
  "follow_symlinks": false,
  "deny_globs": [".git", ".git/**", ".ssh", ".ssh/**", ".aws", ".aws/**", ".env", ".env.*", "*.pem", "*.key", "*.pfx", "*.p12", "*credential*", "*secret*"],
  "allow_extensions": [".txt", ".md", ".json", ".jsonl", ".csv", ".tsv", ".yaml", ".yml", ".toml", ".log", ".xml", ".html", ".css", ".js", ".ts", ".py", ".go", ".rs", ".java", ".c", ".h", ".cpp", ".hpp", ".cs", ".sql", ".sh", ".ps1"],
  "max_file_bytes": 1048576,
  "max_response_bytes": 262144,
  "max_search_files": 10000,
  "max_search_results": 100,
  "search_timeout_ms": 2000,
  "max_concurrency": 4,
  "requests_per_minute": 60,
  "sensitive_content_action": "deny",
  "max_write_bytes": 1048576,
  "write_permissions": {"enabled": false, "create_files": false, "overwrite_files": false, "create_directories": false}
}
EOF
  chmod 600 "$CONFIG"
fi

if [ -z "${SAFE_LOCAL_FILES_TOKEN:-}" ]; then
  if [ ! -f "$TOKEN_FILE" ]; then
    "$BIN" token >"$TOKEN_FILE"
    chmod 600 "$TOKEN_FILE"
  fi
  SAFE_LOCAL_FILES_TOKEN=$(cat "$TOKEN_FILE")
  export SAFE_LOCAL_FILES_TOKEN
fi

if command -v launchctl >/dev/null 2>&1; then
  launchctl setenv SAFE_LOCAL_FILES_TOKEN "$SAFE_LOCAL_FILES_TOKEN"
elif command -v systemctl >/dev/null 2>&1; then
  systemctl --user set-environment SAFE_LOCAL_FILES_TOKEN="$SAFE_LOCAL_FILES_TOKEN" 2>/dev/null || true
fi

if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
  echo "Safe Local Files is already running (PID $(cat "$PID_FILE"))."
  exit 0
fi

"$BIN" validate --config "$CONFIG"
nohup "$BIN" serve --config "$CONFIG" >"$DATA_DIR/server.stdout.log" 2>"$DATA_DIR/server.stderr.log" &
echo $! >"$PID_FILE"
echo "Safe Local Files started (PID $!). Config: $CONFIG"
echo "Restart Codex/ChatGPT once after first setup so it receives SAFE_LOCAL_FILES_TOKEN."

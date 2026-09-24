#!/usr/bin/env sh
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PLUGIN_ROOT="$REPO_ROOT/plugins/safe-local-files"
chmod +x "$PLUGIN_ROOT"/scripts/*.sh
"$PLUGIN_ROOT/scripts/build.sh"
codex plugin marketplace add "$REPO_ROOT"
codex plugin add safe-local-files@personal
"$PLUGIN_ROOT/scripts/start.sh"
"$PLUGIN_ROOT/scripts/stop.sh"
DATA_DIR=${XDG_CONFIG_HOME:-"$HOME/.config"}/safe-local-files
codex mcp remove safe_local_files >/dev/null 2>&1 || true
codex mcp add safe_local_files -- "$PLUGIN_ROOT/bin/safe-local-files" stdio --config "$DATA_DIR/config.json"
echo "Installed Safe Local Files with on-demand stdio MCP. Start a new task."

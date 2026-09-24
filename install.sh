#!/usr/bin/env sh
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PLUGIN_ROOT="$REPO_ROOT/plugins/safe-local-files"
chmod +x "$PLUGIN_ROOT"/scripts/*.sh
"$PLUGIN_ROOT/scripts/build.sh"
codex plugin marketplace add "$REPO_ROOT"
codex plugin add safe-local-files@personal
"$PLUGIN_ROOT/scripts/start.sh"
echo "Installed Safe Local Files. Restart the ChatGPT desktop app and use a new task."

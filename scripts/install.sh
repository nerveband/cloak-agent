#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BUNDLE="$(dirname "$SCRIPT_DIR")"
INSTALL_DIR="${CLOAK_AGENT_INSTALL_DIR:-$HOME/.cloak-agent}"
if [[ ! -x "$BUNDLE/cloak-agent" || ! -f "$BUNDLE/daemon/dist/daemon.js" ]]; then
    if [[ ! -x "$SCRIPT_DIR/build.sh" ]]; then
        echo "Incomplete release archive: executable and daemon/dist are required." >&2
        exit 1
    fi
    "$SCRIPT_DIR/build.sh"
fi
CLOAK_AGENT_INSTALL_DIR="$INSTALL_DIR" "$BUNDLE/cloak-agent" install --from "$BUNDLE"
LINK_DIR="${CLOAK_AGENT_LINK_DIR:-$HOME/.local/bin}"
mkdir -p "$LINK_DIR"
ln -sfn "$INSTALL_DIR/bin/cloak-agent" "$LINK_DIR/cloak-agent"
printf 'Command: %s/cloak-agent\nAdd %s to PATH if needed.\n' "$LINK_DIR" "$LINK_DIR"

#!/bin/bash
set -euo pipefail

INSTALL_DIR="${CLOAK_AGENT_INSTALL_DIR:-$HOME/.cloak-agent}"
PURGE=0
if [ "${1:-}" = "--purge" ]; then
    PURGE=1
elif [ "${1:-}" != "" ]; then
    echo "Usage: $0 [--purge]" >&2
    exit 64
fi

remove_link_if_owned() {
    local path="$1"
    if [ -L "$path" ] && [ "$(readlink "$path")" = "$INSTALL_DIR/bin/cloak-agent" ]; then
        rm -f "$path"
    fi
}

remove_link_if_owned "$HOME/.local/bin/cloak-agent"
if [ -n "${CLOAK_AGENT_LINK_DIR:-}" ]; then
    remove_link_if_owned "$CLOAK_AGENT_LINK_DIR/cloak-agent"
fi

if [ "$PURGE" -eq 1 ]; then
    echo "Removing cloak-agent installation, profiles, and private state from $INSTALL_DIR"
    rm -rf -- "$INSTALL_DIR"
    CONFIG_HOME="${XDG_CONFIG_HOME:-$HOME/.config}"
    STATE_HOME="${XDG_STATE_HOME:-$HOME/.local/state}"
    rm -rf -- "$CONFIG_HOME/cloak-agent" "$STATE_HOME/cloak-agent"
else
    echo "Removing cloak-agent executable and daemon (profiles/config preserved)"
    rm -rf -- "$INSTALL_DIR/bin" "$INSTALL_DIR/daemon"
    rmdir "$INSTALL_DIR" 2>/dev/null || true
fi

echo "cloak-agent uninstall complete"

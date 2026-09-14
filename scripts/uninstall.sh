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

# Do not remove code still used by an active daemon or an in-progress install.
SOCKET_DIR="${CLOAK_AGENT_SOCKET_DIR:-${XDG_RUNTIME_DIR:-$INSTALL_DIR}}"
if [ -z "${CLOAK_AGENT_SOCKET_DIR:-}" ] && [ -n "${XDG_RUNTIME_DIR:-}" ]; then
    SOCKET_DIR="$XDG_RUNTIME_DIR/cloak-agent"
fi
if [ -d "$INSTALL_DIR/.install-lock" ]; then
    echo "An installation lock exists. Finish or recover that installation first." >&2
    exit 1
fi
for pidfile in "$SOCKET_DIR"/*.pid; do
    [ -f "$pidfile" ] || continue
    read -r pid < "$pidfile" || continue
    case "$pid" in ''|*[!0-9]*) continue ;; esac
    if kill -0 "$pid" 2>/dev/null; then
        echo "A cloak-agent daemon is active. Close its session before uninstalling." >&2
        exit 1
    fi
done

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
    rm -rf -- "$INSTALL_DIR/bin" "$INSTALL_DIR/daemon" "$INSTALL_DIR/releases"
    rm -f -- "$INSTALL_DIR/current.json"
    rmdir "$INSTALL_DIR" 2>/dev/null || true
fi

echo "cloak-agent uninstall complete"

#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
INSTALL_DIR="${CLOAK_AGENT_INSTALL_DIR:-$HOME/.cloak-agent}"
BIN_DIR="$INSTALL_DIR/bin"
BIN_PATH="$BIN_DIR/cloak-agent"

require_command() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "Error: $1 not found in PATH. $2" >&2
        exit 1
    fi
}

require_command node "Install Node.js 20+ to bootstrap cloak-agent."
require_command npm "Install npm to bootstrap cloak-agent."
require_command npx "Install npm to run cloakbrowser install."

echo "Building cloak-agent..."
"$SCRIPT_DIR/build.sh"

echo ""
echo "Installing to $INSTALL_DIR..."

# Install CLI binary
mkdir -p "$BIN_DIR"
cp "$PROJECT_DIR/cloak-agent" "$BIN_PATH"

# Install daemon
mkdir -p "$INSTALL_DIR/daemon"
cp -r "$PROJECT_DIR/daemon/dist" "$INSTALL_DIR/daemon/"
cp "$PROJECT_DIR/daemon/package.json" "$INSTALL_DIR/daemon/"
cd "$INSTALL_DIR/daemon" && npm install --omit=dev --quiet
cd "$INSTALL_DIR/daemon" && npx cloakbrowser install

path_contains() {
    case ":$PATH:" in
        *":$1:"*) return 0 ;;
        *) return 1 ;;
    esac
}

link_command() {
    local target_dir="$1"
    mkdir -p "$target_dir"
    ln -sf "$BIN_PATH" "$target_dir/cloak-agent"
    echo "Linked: $target_dir/cloak-agent"
}

if path_contains "$BIN_DIR"; then
    echo "Command available: $BIN_PATH"
else
    LINKED=0
    IFS=':' read -r -a PATH_DIRS <<< "$PATH"
    for path_dir in "${PATH_DIRS[@]}"; do
        if [ -n "$path_dir" ] && [ -d "$path_dir" ] && [ -w "$path_dir" ]; then
            link_command "$path_dir"
            LINKED=1
            break
        fi
    done

    if [ "$LINKED" -eq 0 ]; then
        USER_LINK_DIR="${CLOAK_AGENT_LINK_DIR:-$HOME/.local/bin}"
        link_command "$USER_LINK_DIR"
        if ! path_contains "$USER_LINK_DIR"; then
            echo "Add to PATH: export PATH=\"$USER_LINK_DIR:\$PATH\""
        fi
    fi
fi

echo ""
echo "Installation complete!"
echo "Run: cloak-agent open https://example.com"

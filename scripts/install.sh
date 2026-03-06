#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
INSTALL_DIR="${CLOAK_AGENT_INSTALL_DIR:-$HOME/.cloak-agent}"

echo "Building cloak-agent..."
"$SCRIPT_DIR/build.sh"

echo ""
echo "Installing to $INSTALL_DIR..."

# Install CLI binary
mkdir -p "$INSTALL_DIR/bin"
cp "$PROJECT_DIR/cloak-agent" "$INSTALL_DIR/bin/"

# Install daemon
mkdir -p "$INSTALL_DIR/daemon"
cp -r "$PROJECT_DIR/daemon/dist" "$INSTALL_DIR/daemon/"
cp "$PROJECT_DIR/daemon/package.json" "$INSTALL_DIR/daemon/"
cd "$INSTALL_DIR/daemon" && npm install --omit=dev --quiet

# Create symlink
LINK_DIR="/usr/local/bin"
if [ -w "$LINK_DIR" ]; then
    ln -sf "$INSTALL_DIR/bin/cloak-agent" "$LINK_DIR/cloak-agent"
    echo "Linked: $LINK_DIR/cloak-agent"
else
    echo "Add to PATH: export PATH=\"$INSTALL_DIR/bin:\$PATH\""
fi

echo ""
echo "Installation complete!"
echo "Run: cloak-agent open https://example.com"

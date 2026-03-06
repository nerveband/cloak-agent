#!/bin/bash
set -e

echo "Building cloak-agent daemon..."
cd "$(dirname "$0")/../daemon"
npm run build
cd ..

echo "Building cloak-agent CLI..."
go build -o cloak-agent .

echo ""
echo "Build complete!"
echo "  Binary: ./cloak-agent"
echo "  Daemon: ./daemon/dist/"

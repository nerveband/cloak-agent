#!/bin/bash
set -e

echo "Building cloak-agent daemon..."
cd "$(dirname "$0")/../daemon"
npm run build
cd ..

echo "Building cloak-agent CLI..."
VERSION=$(grep 'var Version' cmd/root.go | head -1 | sed 's/.*"\(.*\)"/\1/')
go build -ldflags "-X github.com/nerveband/cloak-agent/cmd.Version=${VERSION}" -o cloak-agent .

echo ""
echo "Build complete!"
echo "  Binary: ./cloak-agent"
echo "  Daemon: ./daemon/dist/"

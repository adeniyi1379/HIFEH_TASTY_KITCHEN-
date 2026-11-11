#!/bin/bash
# ============================================================================
# Build the Hifeh Tasty Kitchen Windows installer
# Produces: HifehTastyKitchen_Setup.exe (single file, ~8MB compressed)
# ============================================================================
set -e

cd "$(dirname "$0")"

echo "╔══════════════════════════════════════════╗"
echo "║  Building Hifeh Tasty Kitchen Installer  ║"
echo "╚══════════════════════════════════════════╝"
echo ""

# Step 1: Build the POS server for Windows
echo "→ Building posd.exe (Windows amd64)..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o cmd/installer/embed/posd.exe ./cmd/posd
echo "  ✓ posd.exe built ($(du -h cmd/installer/embed/posd.exe | cut -f1))"

# Step 2: Build the installer (embeds posd.exe, no console window)
echo "→ Building installer..."
GOOS=windows GOARCH=amd64 go build \
    -ldflags="-s -w -H windowsgui" \
    -o HifehTastyKitchen_Setup.exe \
    ./cmd/installer
echo "  ✓ HifehTastyKitchen_Setup.exe built ($(du -h HifehTastyKitchen_Setup.exe | cut -f1))"

# Clean up embedded copy
rm -f cmd/installer/embed/posd.exe

echo ""
echo "Done! Send this file to your users:"
echo "  → HifehTastyKitchen_Setup.exe"
echo ""

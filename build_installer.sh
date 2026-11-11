#!/bin/bash
# ============================================================================
# Build the Hifeh Tasty Kitchen Windows installer
# Produces: HifehTastyKitchen_Setup.exe (single file, self-contained)
#
# This builds THREE things:
#   1. posd.exe        — the POS server
#   2. uninstall.exe   — the uninstaller (registered in Add/Remove Programs)
#   3. Setup exe       — embeds both above, shows native Win32 GUI
# ============================================================================
set -e

cd "$(dirname "$0")"

EMBED_DIR="cmd/installer/embed"
mkdir -p "$EMBED_DIR"

echo "╔══════════════════════════════════════════╗"
echo "║  Building Hifeh Tasty Kitchen Installer  ║"
echo "╚══════════════════════════════════════════╝"
echo ""

# Step 1: Build the POS server
echo "→ [1/3] Building posd.exe..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$EMBED_DIR/posd.exe" ./cmd/posd
echo "  ✓ posd.exe ($(du -h "$EMBED_DIR/posd.exe" | cut -f1))"

# Step 2: Build the uninstaller
echo "→ [2/3] Building uninstall.exe..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H windowsgui" -o "$EMBED_DIR/uninstall.exe" ./cmd/uninstaller
echo "  ✓ uninstall.exe ($(du -h "$EMBED_DIR/uninstall.exe" | cut -f1))"

# Step 3: Build the installer (embeds both)
echo "→ [3/3] Building installer..."
GOOS=windows GOARCH=amd64 go build \
    -ldflags="-s -w -H windowsgui" \
    -o HifehTastyKitchen_Setup.exe \
    ./cmd/installer
echo "  ✓ HifehTastyKitchen_Setup.exe ($(du -h HifehTastyKitchen_Setup.exe | cut -f1))"

# Clean up embedded copies
rm -f "$EMBED_DIR/posd.exe" "$EMBED_DIR/uninstall.exe"

echo ""
echo "════════════════════════════════════════════"
echo "  Done! Distribute this single file:"
echo "  → HifehTastyKitchen_Setup.exe"
echo "════════════════════════════════════════════"
echo ""

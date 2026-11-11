#!/bin/bash
# ============================================================================
# Build the Hifeh Tasty Kitchen Windows installer
# Produces: HifehTastyKitchen_Setup.exe (single file, self-contained)
#
# Embeds the branded app icon into:
#   - posd.exe (embedded inside installer)
#   - standalone posd.exe
#   - uninstall.exe
#   - HifehTastyKitchen_Setup.exe
#   - Windows desktop / Start menu shortcuts & Add/Remove Programs
# ============================================================================
set -e

cd "$(dirname "$0")"

EMBED_DIR="cmd/installer/embed"
mkdir -p "$EMBED_DIR" assets

# Ensure icon exists
if [ ! -f assets/app.png ]; then
    cp web/static/images/logo.png assets/app.png
fi
if [ ! -f assets/app.ico ]; then
    convert assets/app.png -define icon:auto-resize=256,128,64,48,32,16 assets/app.ico
fi

# Copy icon into installer embed folder
cp assets/app.ico "$EMBED_DIR/app.ico"

echo "╔══════════════════════════════════════════╗"
echo "║  Building Hifeh Tasty Kitchen Installer  ║"
echo "╚══════════════════════════════════════════╝"
echo ""

# Generate Windows resources (icons + manifests)
echo "→ Generating Windows PE icon resources..."
(cd cmd/posd && /home/balogun/go/bin/go-winres simply --icon ../../assets/app.png --arch amd64 --product-name "Hifeh Tasty Kitchen" --file-description "Hifeh Tasty Kitchen POS Server")
(cd cmd/uninstaller && /home/balogun/go/bin/go-winres simply --icon ../../assets/app.png --arch amd64 --manifest gui --product-name "Hifeh Tasty Kitchen" --file-description "Hifeh Tasty Kitchen Uninstaller")
(cd cmd/installer && /home/balogun/go/bin/go-winres simply --icon ../../assets/app.png --arch amd64 --manifest gui --product-name "Hifeh Tasty Kitchen" --file-description "Hifeh Tasty Kitchen Setup")

# Step 1: Build the POS server for Windows
echo "→ [1/3] Building posd.exe..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$EMBED_DIR/posd.exe" ./cmd/posd
echo "  ✓ posd.exe ($(du -h "$EMBED_DIR/posd.exe" | cut -f1))"

# Also update root standalone posd.exe
cp "$EMBED_DIR/posd.exe" posd.exe

# Step 2: Build the uninstaller
echo "→ [2/3] Building uninstall.exe..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H windowsgui" -o "$EMBED_DIR/uninstall.exe" ./cmd/uninstaller
echo "  ✓ uninstall.exe ($(du -h "$EMBED_DIR/uninstall.exe" | cut -f1))"

# Step 3: Build the installer (embeds posd.exe, uninstall.exe, app.ico)
echo "→ [3/3] Building installer..."
GOOS=windows GOARCH=amd64 go build \
    -ldflags="-s -w -H windowsgui" \
    -o HifehTastyKitchen_Setup.exe \
    ./cmd/installer
echo "  ✓ HifehTastyKitchen_Setup.exe ($(du -h HifehTastyKitchen_Setup.exe | cut -f1))"

# Clean up temporary embedded binaries
rm -f "$EMBED_DIR/posd.exe" "$EMBED_DIR/uninstall.exe" "$EMBED_DIR/app.ico"

echo ""
echo "════════════════════════════════════════════"
echo "  Done! Distribute this single file:"
echo "  → HifehTastyKitchen_Setup.exe"
echo "════════════════════════════════════════════"
echo ""

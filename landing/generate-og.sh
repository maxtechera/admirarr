#!/usr/bin/env bash
# Generates og.png from og.svg using rsvg-convert (librsvg) or Inkscape
# Run once when building the site

set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if command -v rsvg-convert &>/dev/null; then
  rsvg-convert -w 1200 -h 630 "$SCRIPT_DIR/og.svg" -o "$SCRIPT_DIR/og.png"
  echo "✓ og.png generated via rsvg-convert"
elif command -v inkscape &>/dev/null; then
  inkscape --export-type=png --export-width=1200 --export-height=630 --export-filename="$SCRIPT_DIR/og.png" "$SCRIPT_DIR/og.svg"
  echo "✓ og.png generated via inkscape"
elif command -v convert &>/dev/null; then
  # ImageMagick fallback
  convert -background none "$SCRIPT_DIR/og.svg" -resize 1200x630 "$SCRIPT_DIR/og.png"
  echo "✓ og.png generated via ImageMagick"
else
  echo "⚠ No SVG→PNG converter found. Install rsvg-convert: sudo apt-get install librsvg2-bin"
  echo "  The og.svg is ready and browsers/crawlers can use it — rename reference in index.html if needed."
  exit 0
fi

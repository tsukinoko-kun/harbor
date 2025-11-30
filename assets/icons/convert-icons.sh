#!/bin/sh
# Convert SVG icons to PNG format for Raylib compatibility
# Requires rsvg-convert (librsvg) or falls back to ImageMagick

set -e

cd "$(dirname "$0")"

# Check if rsvg-convert is available
if command -v rsvg-convert >/dev/null 2>&1; then
    for svg in *.svg; do
        png="${svg%.svg}.png"
        echo "Converting $svg -> $png (using rsvg-convert)"
        rsvg-convert -w 48 -h 48 -f png -o "$png" "$svg"
    done
else
    # Fallback to ImageMagick
    for svg in *.svg; do
        png="${svg%.svg}.png"
        echo "Converting $svg -> $png (using ImageMagick)"
        magick "$svg" \
            -background transparent \
            -density 300 \
            -resize 48x48 \
            -colorspace sRGB \
            -depth 8 \
            -alpha on \
            -define png:format=png32 \
            -type TrueColorAlpha \
            "$png"
    done
fi

echo "Done."


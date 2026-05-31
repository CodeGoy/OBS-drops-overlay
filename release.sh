#!/bin/bash

set -e

APP_NAME="overlay"
OUTPUT_DIR="release"
VERSION="$(grep -P -o 'version\s+=\s+"\K[^"]+' main.go)"

rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

PLATFORMS=(
    "windows/amd64"
    "windows/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "linux/amd64"
    "linux/arm64"
)

for PLATFORM in "${PLATFORMS[@]}"; do
    IFS="/" read -r GOOS GOARCH <<< "$PLATFORM"
    OUTPUT_NAME="$OUTPUT_DIR/${APP_NAME}-v${VERSION}-${GOOS}-${GOARCH}"
    if [ "$GOOS" = "windows" ]; then
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
    fi
    echo "Building for $GOOS ($GOARCH)..."
    env CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -ldflags="-s -w" -o "$OUTPUT_NAME" .
done

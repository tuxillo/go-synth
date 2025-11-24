#!/bin/sh
# Build script for dsynth-go

set -e

VERSION=${VERSION:-"2.0.0-dev"}
GOOS=${GOOS:-$(uname -s | tr '[:upper:]' '[:lower:]')}
GOARCH=${GOARCH:-"amd64"}

echo "Building dsynth-go ${VERSION} for ${GOOS}/${GOARCH}..."

go build \
    -ldflags "-X main.Version=${VERSION}" \
    -o dsynth \
    .

echo "Build complete: ./dsynth"
echo ""
echo "To install:"
echo "  sudo install -m 0755 dsynth /usr/local/bin/"
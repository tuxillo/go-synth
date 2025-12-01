#!/bin/sh
# Build script for go-synth

set -e

VERSION=${VERSION:-"2.0.0-dev"}
GOOS=${GOOS:-$(uname -s | tr '[:upper:]' '[:lower:]')}
GOARCH=${GOARCH:-"amd64"}

echo "Building go-synth ${VERSION} for ${GOOS}/${GOARCH}..."

go build \
    -ldflags "-X main.Version=${VERSION}" \
    -o go-synth \
    .

echo "Build complete: ./go-synth"
echo ""
echo "To install:"
echo "  sudo install -m 0755 go-synth /usr/local/bin/"

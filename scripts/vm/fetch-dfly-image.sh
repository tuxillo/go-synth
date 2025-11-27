#!/bin/bash
# Fetch DragonFlyBSD ISO image for VM testing
set -euo pipefail

VM_DIR="${HOME}/.go-synth/vm"
IMAGE_URL="https://mirror-master.dragonflybsd.org/iso-images/dfly-x86_64-6.4.0_REL.iso.bz2"
IMAGE_FILE="${VM_DIR}/dfly-6.4.0.iso"

mkdir -p "${VM_DIR}"

if [ -f "${IMAGE_FILE}" ]; then
    echo "✓ DragonFlyBSD image already exists: ${IMAGE_FILE}"
    echo "  Size: $(du -h "${IMAGE_FILE}" | cut -f1)"
    exit 0
fi

echo "📥 Downloading DragonFlyBSD 6.4.0 ISO..."
echo "   URL: ${IMAGE_URL}"
echo "   This may take a few minutes (~300 MB compressed)"

if ! curl -L -o "${IMAGE_FILE}.bz2" "${IMAGE_URL}"; then
    echo "❌ Download failed"
    rm -f "${IMAGE_FILE}.bz2"
    exit 1
fi

echo "📦 Extracting image..."
if ! bunzip2 "${IMAGE_FILE}.bz2"; then
    echo "❌ Extraction failed"
    rm -f "${IMAGE_FILE}" "${IMAGE_FILE}.bz2"
    exit 1
fi

echo "✓ Image ready: ${IMAGE_FILE}"
echo "  Size: $(du -h "${IMAGE_FILE}" | cut -f1)"

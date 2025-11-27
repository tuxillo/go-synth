#!/bin/bash
# Fetch DragonFlyBSD ISO image for VM testing
set -euo pipefail

# Load VM configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/config.sh"

mkdir -p "${VM_DIR}"

if [ -f "${VM_IMAGE}" ]; then
    echo "✓ DragonFlyBSD ${DFLY_VERSION} image already exists: ${VM_IMAGE}"
    echo "  Size: $(du -h "${VM_IMAGE}" | cut -f1)"
    exit 0
fi

echo "📥 Downloading DragonFlyBSD ${DFLY_VERSION} ISO..."
echo "   URL: ${VM_DOWNLOAD_URL}"
echo "   This may take a few minutes (~300 MB compressed)"

if ! curl -L -o "${VM_IMAGE}.bz2" "${VM_DOWNLOAD_URL}"; then
    echo "❌ Download failed"
    echo "   Check if version ${DFLY_VERSION} exists at:"
    echo "   https://mirror-master.dragonflybsd.org/iso-images/"
    rm -f "${VM_IMAGE}.bz2"
    exit 1
fi

echo "📦 Extracting image..."
if ! bunzip2 "${VM_IMAGE}.bz2"; then
    echo "❌ Extraction failed"
    rm -f "${VM_IMAGE}" "${VM_IMAGE}.bz2"
    exit 1
fi

echo "✓ Image ready: ${VM_IMAGE}"
echo "  Size: $(du -h "${VM_IMAGE}" | cut -f1)"

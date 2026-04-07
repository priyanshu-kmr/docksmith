#!/bin/bash
# setup-base-image.sh — Downloads Alpine Linux minirootfs and imports it as
# a base image into ~/.docksmith/
#
# Usage: sudo bash scripts/setup-base-image.sh
# Run once before any build.

set -euo pipefail

ALPINE_VERSION="3.18"
ALPINE_RELEASE="3.18.0"
ARCH="x86_64"
ALPINE_URL="https://dl-cdn.alpinelinux.org/alpine/v${ALPINE_VERSION}/releases/${ARCH}/alpine-minirootfs-${ALPINE_RELEASE}-${ARCH}.tar.gz"

DOCKSMITH_DIR="${HOME}/.docksmith"
IMAGES_DIR="${DOCKSMITH_DIR}/images"
LAYERS_DIR="${DOCKSMITH_DIR}/layers"
CACHE_DIR="${DOCKSMITH_DIR}/cache"
TMP_DIR=$(mktemp -d)

cleanup() {
    rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

echo "=== Docksmith Base Image Setup ==="
echo ""

# Create state directories
mkdir -p "${IMAGES_DIR}" "${LAYERS_DIR}" "${CACHE_DIR}"

# Check if image already exists
if [ -f "${IMAGES_DIR}/alpine_${ALPINE_VERSION}.json" ]; then
    echo "Base image alpine:${ALPINE_VERSION} already exists."
    echo "To reimport, first run: rm ${IMAGES_DIR}/alpine_${ALPINE_VERSION}.json"
    exit 0
fi

# Download Alpine minirootfs
ROOTFS_TAR="${TMP_DIR}/alpine-minirootfs.tar.gz"
echo "Downloading Alpine Linux ${ALPINE_RELEASE} minirootfs..."
if command -v curl &> /dev/null; then
    curl -fsSL -o "${ROOTFS_TAR}" "${ALPINE_URL}"
elif command -v wget &> /dev/null; then
    wget -q -O "${ROOTFS_TAR}" "${ALPINE_URL}"
else
    echo "Error: curl or wget required"
    exit 1
fi
echo "Downloaded $(du -h "${ROOTFS_TAR}" | cut -f1) rootfs archive."

# Extract to a temp rootfs directory
ROOTFS_DIR="${TMP_DIR}/rootfs"
mkdir -p "${ROOTFS_DIR}"
echo "Extracting rootfs..."
tar xzf "${ROOTFS_TAR}" -C "${ROOTFS_DIR}"

# Create a deterministic tar from the rootfs (sorted entries, zeroed timestamps)
# We use the docksmith binary for this if available, otherwise manual tar
LAYER_TAR="${TMP_DIR}/layer.tar"

echo "Creating deterministic layer tar..."
# Use find + sort to get deterministic order, zero timestamps
cd "${ROOTFS_DIR}"
find . -mindepth 1 | sort | while read -r f; do
    # strip the leading ./
    true
done

# Create tar with deterministic properties
tar --sort=name \
    --mtime='1970-01-01 00:00:00' \
    --owner=0 --group=0 \
    --numeric-owner \
    --pax-option=exthdr.name=%d/PaxHeaders/%f,delete=atime,delete=ctime \
    -cf "${LAYER_TAR}" -C "${ROOTFS_DIR}" .

# Compute SHA-256 digest
DIGEST=$(sha256sum "${LAYER_TAR}" | cut -d' ' -f1)
LAYER_SIZE=$(stat -c%s "${LAYER_TAR}")

echo "Layer digest: sha256:${DIGEST}"
echo "Layer size: ${LAYER_SIZE} bytes"

# Store the layer
LAYER_DEST="${LAYERS_DIR}/${DIGEST}.tar"
if [ ! -f "${LAYER_DEST}" ]; then
    cp "${LAYER_TAR}" "${LAYER_DEST}"
    echo "Layer stored at ${LAYER_DEST}"
else
    echo "Layer already exists at ${LAYER_DEST}"
fi

# Create manifest JSON
# We compute the manifest digest by: serialize with digest="" then SHA-256 it
CREATED=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
MANIFEST_NO_DIGEST=$(cat <<EOF
{"name":"alpine","tag":"${ALPINE_VERSION}","digest":"","created":"${CREATED}","config":{"Env":[],"Cmd":["/bin/sh"],"WorkingDir":"/"},"layers":[{"digest":"sha256:${DIGEST}","size":${LAYER_SIZE},"createdBy":"Alpine Linux ${ALPINE_RELEASE} base layer"}]}
EOF
)

MANIFEST_DIGEST=$(echo -n "${MANIFEST_NO_DIGEST}" | sha256sum | cut -d' ' -f1)

# Write final manifest with computed digest
cat > "${IMAGES_DIR}/alpine_${ALPINE_VERSION}.json" <<EOF
{
  "name": "alpine",
  "tag": "${ALPINE_VERSION}",
  "digest": "sha256:${MANIFEST_DIGEST}",
  "created": "${CREATED}",
  "config": {
    "Env": [],
    "Cmd": ["/bin/sh"],
    "WorkingDir": "/"
  },
  "layers": [
    {
      "digest": "sha256:${DIGEST}",
      "size": ${LAYER_SIZE},
      "createdBy": "Alpine Linux ${ALPINE_RELEASE} base layer"
    }
  ]
}
EOF

echo ""
echo "=== Setup Complete ==="
echo "Base image: alpine:${ALPINE_VERSION}"
echo "Manifest:   ${IMAGES_DIR}/alpine_${ALPINE_VERSION}.json"
echo "Layer:      ${LAYER_DEST}"
echo ""
echo "You can now build images using: FROM alpine:${ALPINE_VERSION}"

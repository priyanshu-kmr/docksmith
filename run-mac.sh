#!/bin/bash
# run-mac.sh — Run Docksmith on macOS via Docker.
#
# Usage:
#   ./run-mac.sh setup                          # One-time base image import
#   ./run-mac.sh build -t myapp:latest sample-app/
#   ./run-mac.sh images
#   ./run-mac.sh rmi myapp:latest
#   ./run-mac.sh run myapp:latest
#
# Requires: Docker Desktop for Mac

set -euo pipefail

IMAGE_NAME="docksmith-dev"
VOLUME_NAME="docksmith-state"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Ensure Docker is running
if ! docker info &>/dev/null; then
    echo "Error: Docker is not running. Please start Docker Desktop."
    exit 1
fi

# Build the Docker image if it doesn't exist or if sources changed
build_image() {
    echo "Building docksmith Docker image..."
    docker build -t "$IMAGE_NAME" "$SCRIPT_DIR"
    echo ""
}

# Create persistent volume for ~/.docksmith state
ensure_volume() {
    if ! docker volume inspect "$VOLUME_NAME" &>/dev/null; then
        docker volume create "$VOLUME_NAME" >/dev/null
    fi
}

# First argument determines the action
if [ $# -lt 1 ]; then
    echo "Usage:"
    echo "  ./run-mac.sh setup                            # Import base image (one-time)"
    echo "  ./run-mac.sh build -t <name:tag> <context>    # Build an image"
    echo "  ./run-mac.sh images                           # List images"
    echo "  ./run-mac.sh rmi <name:tag>                   # Remove an image"
    echo "  ./run-mac.sh run [-e K=V] <name:tag> [cmd]    # Run a container"
    echo "  ./run-mac.sh rebuild                          # Force rebuild Docker image"
    exit 1
fi

ensure_volume

case "$1" in
    rebuild)
        build_image
        echo "Docker image rebuilt."
        exit 0
        ;;
    setup)
        # Build image first if needed
        if ! docker image inspect "$IMAGE_NAME" &>/dev/null; then
            build_image
        fi
        # Run the setup script — override entrypoint since this isn't a docksmith command
        docker run --rm \
            --privileged \
            --entrypoint bash \
            -v "$VOLUME_NAME":/root/.docksmith \
            "$IMAGE_NAME" \
            /usr/local/bin/setup-base-image.sh
        ;;
    *)
        # Build image first if needed
        if ! docker image inspect "$IMAGE_NAME" &>/dev/null; then
            build_image
        fi
        # Run docksmith with the provided arguments
        # Mount the project directory as /workspace for build contexts
        docker run --rm -it \
            --privileged \
            -v "$VOLUME_NAME":/root/.docksmith \
            -v "$SCRIPT_DIR":/workspace \
            "$IMAGE_NAME" \
            "$@"
        ;;
esac

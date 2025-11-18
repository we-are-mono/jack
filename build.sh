#!/bin/bash
set -e

# Default to arm64, but allow override via environment or parameter
GOARCH=${GOARCH:-${1:-arm64}}
GOOS=${GOOS:-linux}

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')

echo "Building Jack for ${GOOS}/${GOARCH}..."
echo "Version: ${VERSION}"
echo "Build Time: ${BUILD_TIME}"
echo ""

# Create bin directory for all binaries
mkdir -p bin

# Build main daemon
echo "Building main daemon..."
CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build \
    -ldflags "-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o bin/jack \
    .

echo "✓ Build complete: bin/jack"
echo "  Size: $(ls -lh bin/jack | awk '{print $5}')"

# Auto-discover and build all plugins in plugins/core/
echo ""
echo "Auto-discovering plugins in plugins/core/..."

PLUGIN_COUNT=0
for plugin_dir in plugins/core/*/; do
    if [ ! -d "$plugin_dir" ]; then
        continue
    fi

    plugin_name=$(basename "$plugin_dir")
    plugin_binary="jack-plugin-${plugin_name}"

    echo ""
    echo "  Building ${plugin_binary}..."

    (cd "$plugin_dir" && \
        CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build \
        -ldflags "-s -w" \
        -o "../../../bin/${plugin_binary}" \
        .)

    echo "  ✓ Plugin build complete: bin/${plugin_binary}"
    echo "    Size: $(ls -lh "bin/${plugin_binary}" | awk '{print $5}')"

    PLUGIN_COUNT=$((PLUGIN_COUNT + 1))
done

echo ""
echo "════════════════════════════════════════════════════════"
echo "Build Summary"
echo "════════════════════════════════════════════════════════"
echo "  Platform: ${GOOS}/${GOARCH}"
echo "  Version: ${VERSION}"
echo "  Main daemon: bin/jack"
echo "  Plugins built: ${PLUGIN_COUNT}"
echo ""
echo "Total binaries:"
ls -lh bin/ | tail -n +2

echo ""
echo "To deploy manually:"
echo "  scp bin/jack root@gateway:/usr/local/bin/"
echo "  scp bin/jack-plugin-* root@gateway:/usr/lib/jack/plugins/"
echo ""
echo "Or use deployment scripts:"
echo "  ./deploy.sh <gateway-ip>         # Quick deployment"
echo "  ./deploy.sh <gateway-ip> deb     # Build and install .deb package"

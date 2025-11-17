#!/bin/bash
set -e

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')

GOOS=linux
GOARCH=arm64

echo "Building Jack for ${GOOS}/${GOARCH}..."
echo "Version: ${VERSION}"
echo "Build Time: ${BUILD_TIME}"

CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build \
    -ldflags "-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o jack \
    .

echo "✓ Build complete: jack"
echo "  Size: $(ls -lh jack | awk '{print $5}')"

# Build plugins
echo ""
echo "Building plugins..."

# Create bin directory for plugin binaries
mkdir -p bin

# Build nftables plugin
echo "  Building jack-plugin-nftables..."
(cd plugins/core/nftables && \
    CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build \
    -ldflags "-s -w" \
    -o ../../../bin/jack-plugin-nftables \
    .)

echo "✓ Plugin build complete: bin/jack-plugin-nftables"
echo "  Size: $(ls -lh bin/jack-plugin-nftables | awk '{print $5}')"

# Build dnsmasq plugin
echo "  Building jack-plugin-dnsmasq..."
(cd plugins/core/dnsmasq && \
    CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build \
    -ldflags "-s -w" \
    -o ../../../bin/jack-plugin-dnsmasq \
    .)

echo "✓ Plugin build complete: bin/jack-plugin-dnsmasq"
echo "  Size: $(ls -lh bin/jack-plugin-dnsmasq | awk '{print $5}')"

# Build wireguard plugin
echo "  Building jack-plugin-wireguard..."
(cd plugins/core/wireguard && \
    CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build \
    -ldflags "-s -w" \
    -o ../../../bin/jack-plugin-wireguard \
    .)

echo "✓ Plugin build complete: bin/jack-plugin-wireguard"
echo "  Size: $(ls -lh bin/jack-plugin-wireguard | awk '{print $5}')"

# Build monitoring plugin
echo "  Building jack-plugin-monitoring..."
(cd plugins/core/monitoring && \
    CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build \
    -ldflags "-s -w" \
    -o ../../../bin/jack-plugin-monitoring \
    .)

echo "✓ Plugin build complete: bin/jack-plugin-monitoring"
echo "  Size: $(ls -lh bin/jack-plugin-monitoring | awk '{print $5}')"

# Build LED plugin
echo "  Building jack-plugin-leds..."
(cd plugins/core/leds && \
    CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build \
    -ldflags "-s -w" \
    -o ../../../bin/jack-plugin-leds \
    .)

echo "✓ Plugin build complete: bin/jack-plugin-leds"
echo "  Size: $(ls -lh bin/jack-plugin-leds | awk '{print $5}')"

echo ""
echo "To deploy:"
echo "  scp jack root@gateway:/usr/local/bin/"
echo "  scp bin/jack-plugin-* root@gateway:/usr/lib/jack/plugins/"
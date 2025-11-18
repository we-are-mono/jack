#!/bin/bash
# Copyright (C) 2025 Mono Technologies Inc.
#
# This program is free software; you can redistribute it and/or
# modify it under the terms of the GNU General Public License
# as published by the Free Software Foundation; version 2.
#
# Package Jack releases as .deb and .tar.gz for multiple architectures

set -e

# Configuration
VERSION="${1:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"
ARCHITECTURES="${JACK_RELEASE_ARCHS:-arm64 amd64}"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  Jack Release Packaging${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════${NC}"
echo -e "Version: ${GREEN}${VERSION}${NC}"
echo -e "Formats: ${GREEN}.deb, .tar.gz${NC}"
echo ""

# Check if release directory exists
if [ ! -d "release" ]; then
    echo -e "${RED}Error: release/ directory not found${NC}"
    echo "Please run ./scripts/build-release.sh first"
    exit 1
fi

# Create packages directory
mkdir -p release/packages

echo -e "${BLUE}Creating tar.gz packages...${NC}"
echo ""

# Create tar.gz packages
for arch in $ARCHITECTURES; do
    BUILD_DIR="release/jack-${VERSION}-linux-${arch}"

    if [ ! -d "$BUILD_DIR" ]; then
        echo -e "${YELLOW}Warning: $BUILD_DIR not found, skipping${NC}"
        continue
    fi

    echo -e "${GREEN}Packaging ${arch}...${NC}"

    # Create temporary packaging directory
    PKG_DIR="release/tmp/jack-${VERSION}-linux-${arch}"
    rm -rf "$PKG_DIR"
    mkdir -p "$PKG_DIR"

    # Copy binaries
    cp "$BUILD_DIR"/* "$PKG_DIR/"

    # Copy install script
    cp scripts/install.sh "$PKG_DIR/"

    # Copy systemd service
    if [ -f "debian/jack.service" ]; then
        cp debian/jack.service "$PKG_DIR/"
    fi

    # Copy config templates if they exist
    if [ -d "examples" ]; then
        mkdir -p "$PKG_DIR/templates"
        [ -f "examples/jack.json" ] && cp examples/jack.json "$PKG_DIR/templates/" || true
        [ -f "examples/config/interfaces.json" ] && cp examples/config/interfaces.json "$PKG_DIR/templates/" || true
    fi

    if [ -d "config/defaults" ]; then
        mkdir -p "$PKG_DIR/templates"
        cp config/defaults/*.json "$PKG_DIR/templates/" 2>/dev/null || true
    fi

    # Create README for the package
    cat > "$PKG_DIR/README.txt" << EOF
Jack Network Gateway - Version ${VERSION}
Architecture: linux/${arch}

Installation:
  sudo ./install.sh

Manual Installation:
  1. Copy binaries:
     sudo cp jack /usr/local/bin/
     sudo cp jack-plugin-* /usr/lib/jack/plugins/

  2. Copy systemd service:
     sudo cp jack.service /lib/systemd/system/
     sudo systemctl daemon-reload

  3. Create directories:
     sudo mkdir -p /etc/jack /var/lib/jack /var/log/jack /run/jack

  4. Start service:
     sudo systemctl enable jack
     sudo systemctl start jack

For more information: https://github.com/we-are-mono/jack
EOF

    # Create tarball
    TAR_NAME="jack-${VERSION}-linux-${arch}.tar.gz"
    (cd release/tmp && tar czf "../packages/${TAR_NAME}" "jack-${VERSION}-linux-${arch}")

    # Calculate checksum
    (cd release/packages && sha256sum "${TAR_NAME}" > "${TAR_NAME}.sha256")

    TAR_SIZE=$(du -sh "release/packages/${TAR_NAME}" | cut -f1)
    echo -e "  ${GREEN}✓ Created ${TAR_NAME} (${TAR_SIZE})${NC}"
done

# Clean up temp directory
rm -rf release/tmp

echo ""
echo -e "${BLUE}Creating Debian packages...${NC}"
echo ""

# Map Go arch names to Debian arch names
declare -A DEB_ARCH_MAP
DEB_ARCH_MAP["amd64"]="amd64"
DEB_ARCH_MAP["arm64"]="arm64"
DEB_ARCH_MAP["arm"]="armhf"

# Create .deb packages
for arch in $ARCHITECTURES; do
    deb_arch="${DEB_ARCH_MAP[$arch]:-$arch}"

    echo -e "${GREEN}Building .deb for ${deb_arch}...${NC}"

    # Clean previous build artifacts
    rm -rf debian/jack
    rm -f jack jack-plugin-*

    # Use the binaries we already built
    BUILD_DIR="release/jack-${VERSION}-linux-${arch}"
    if [ ! -d "$BUILD_DIR" ]; then
        echo -e "${YELLOW}Warning: $BUILD_DIR not found, skipping${NC}"
        continue
    fi

    # Copy binaries to build directory for debian packaging
    cp "$BUILD_DIR"/* ./

    # Build the package for specific architecture
    DEB_BUILD_OPTIONS=nocheck dpkg-buildpackage \
        -a"${deb_arch}" \
        -us -uc \
        -b 2>&1 | grep -v "warning:" || true

    # Move the .deb to packages directory
    DEB_FILE="../jack_${VERSION}_${deb_arch}.deb"
    if [ -f "$DEB_FILE" ]; then
        mv "$DEB_FILE" "release/packages/"
        DEB_NAME=$(basename "$DEB_FILE")

        # Create checksum
        (cd release/packages && sha256sum "${DEB_NAME}" > "${DEB_NAME}.sha256")

        DEB_SIZE=$(du -sh "release/packages/${DEB_NAME}" | cut -f1)
        echo -e "  ${GREEN}✓ Created ${DEB_NAME} (${DEB_SIZE})${NC}"
    else
        echo -e "  ${YELLOW}Warning: .deb package not created${NC}"
    fi

    # Clean up
    rm -f jack jack-plugin-*
    rm -rf debian/jack
done

echo ""
echo -e "${BLUE}════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}✓ Packaging complete${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════${NC}"
echo ""
echo "Release packages created in: release/packages/"
echo ""
ls -lh release/packages/
echo ""
echo "Next steps:"
echo "  ./scripts/create-github-release.sh ${VERSION}"

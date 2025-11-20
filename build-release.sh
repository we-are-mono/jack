#!/bin/bash
# Copyright (C) 2025 Mono Technologies Inc.
#
# This program is free software; you can redistribute it and/or
# modify it under the terms of the GNU General Public License
# as published by the Free Software Foundation; version 2.
#
# Build release packages for Jack (both .deb and .tar.gz for amd64 and arm64)

set -e

# Get version from debian/changelog (this is what dpkg-buildpackage uses)
DEB_VERSION=$(dpkg-parsechangelog -S Version)
# Use git version for tarball naming (more descriptive)
GIT_VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
VERSION="${DEB_VERSION}"
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
RELEASE_DIR="release"

echo "════════════════════════════════════════════════════════"
echo "Jack Release Builder"
echo "════════════════════════════════════════════════════════"
echo "Version: ${VERSION}"
echo "Build Time: ${BUILD_TIME}"
echo ""

# Clean previous builds
echo "Cleaning previous builds..."
rm -rf "${RELEASE_DIR}"
mkdir -p "${RELEASE_DIR}"
make clean

# Function to build .deb package for a specific architecture
build_deb() {
    local arch=$1
    local native_arch=$(dpkg --print-architecture)
    echo ""
    echo "════════════════════════════════════════════════════════"
    echo "Building .deb package for ${arch}..."
    echo "════════════════════════════════════════════════════════"

    # Clean before building
    debian/rules clean

    # Build package for specified architecture
    # Use -d to skip build dependency checks when cross-compiling (CGO_ENABLED=0 works without arch-specific go)
    if [ "${arch}" != "${native_arch}" ]; then
        echo "Cross-compiling from ${native_arch} to ${arch}, skipping build-dep check..."
        dpkg-buildpackage -a${arch} -b -us -uc -d
    else
        dpkg-buildpackage -a${arch} -b -us -uc
    fi

    # Move .deb to release directory
    mv ../jack_${VERSION}_${arch}.deb "${RELEASE_DIR}/"

    # Clean up other generated files
    rm -f ../jack_${VERSION}_${arch}.buildinfo
    rm -f ../jack_${VERSION}_${arch}.changes

    echo "✓ .deb package created: ${RELEASE_DIR}/jack_${VERSION}_${arch}.deb"
}

# Function to build .tar.gz package for a specific architecture
build_tarball() {
    local arch=$1
    echo ""
    echo "════════════════════════════════════════════════════════"
    echo "Building .tar.gz package for ${arch}..."
    echo "════════════════════════════════════════════════════════"

    # Build binaries for this architecture
    GOARCH=${arch} ./build.sh

    # Create temporary directory structure
    local tarball_dir="jack-${VERSION}-linux-${arch}"
    rm -rf "${tarball_dir}"
    mkdir -p "${tarball_dir}"

    # Copy binaries
    cp bin/jack "${tarball_dir}/"
    cp bin/jack-plugin-* "${tarball_dir}/"

    # Copy installation script
    cp scripts/install.sh "${tarball_dir}/"

    # Copy systemd service file
    cp debian/jack.service "${tarball_dir}/"

    # Copy config templates
    mkdir -p "${tarball_dir}/templates"
    cp examples/jack.json "${tarball_dir}/templates/"
    cp config/defaults/firewall.json "${tarball_dir}/templates/"
    cp config/defaults/dhcp.json "${tarball_dir}/templates/"
    cp config/defaults/dns.json "${tarball_dir}/templates/"
    cp config/defaults/routes.json "${tarball_dir}/templates/"
    cp config/defaults/vpn.json "${tarball_dir}/templates/wireguard.json"
    cp examples/config/interfaces.json "${tarball_dir}/templates/"

    # Create README for tarball
    cat > "${tarball_dir}/README.txt" << 'EOF'
Jack Network Gateway Management Daemon
=======================================

Installation:
  sudo ./install.sh

This will install:
  - /usr/bin/jack - Main daemon
  - /usr/lib/jack/plugins/ - Plugin binaries
  - /lib/systemd/system/jack.service - Systemd service
  - /etc/jack/templates/ - Configuration templates

After installation:
  1. Copy configuration templates from /etc/jack/templates/ to /etc/jack/
  2. Edit configurations as needed
  3. Enable and start the service:
     sudo systemctl enable jack
     sudo systemctl start jack

For more information, visit:
https://github.com/we-are-mono/jack
EOF

    # Create tarball
    tar czf "${RELEASE_DIR}/jack_${VERSION}_linux_${arch}.tar.gz" "${tarball_dir}"

    # Clean up temporary directory
    rm -rf "${tarball_dir}"

    echo "✓ Tarball created: ${RELEASE_DIR}/jack_${VERSION}_linux_${arch}.tar.gz"
}

# Build packages for both architectures
for arch in amd64 arm64; do
    build_deb "${arch}"
    build_tarball "${arch}"
done

# Generate checksums
echo ""
echo "════════════════════════════════════════════════════════"
echo "Generating checksums..."
echo "════════════════════════════════════════════════════════"
cd "${RELEASE_DIR}"
sha256sum * > SHA256SUMS
cd ..

# Display summary
echo ""
echo "════════════════════════════════════════════════════════"
echo "Release Build Complete!"
echo "════════════════════════════════════════════════════════"
echo "Version: ${VERSION}"
echo ""
echo "Release artifacts in ${RELEASE_DIR}/:"
ls -lh "${RELEASE_DIR}/"
echo ""
echo "SHA256 checksums:"
cat "${RELEASE_DIR}/SHA256SUMS"
echo ""
echo "Ready to upload to GitHub release!"

#!/bin/bash
# Copyright (C) 2025 Mono Technologies Inc.
#
# This program is free software; you can redistribute it and/or
# modify it under the terms of the GNU General Public License
# as published by the Free Software Foundation; version 2.
#
# Build Jack for multiple architectures in preparation for a release

set -e

# Configuration
ARCHITECTURES="${JACK_RELEASE_ARCHS:-arm64 amd64}"
VERSION="${1:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  Jack Multi-Architecture Release Build${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════${NC}"
echo -e "Version: ${GREEN}${VERSION}${NC}"
echo -e "Architectures: ${GREEN}${ARCHITECTURES}${NC}"
echo ""

# Clean previous builds
if [ -d "bin" ]; then
    echo -e "${YELLOW}Cleaning previous builds...${NC}"
    rm -rf bin
fi

if [ -d "release" ]; then
    echo -e "${YELLOW}Cleaning previous release artifacts...${NC}"
    rm -rf release
fi

# Create release directory structure
mkdir -p release

# Build for each architecture
for arch in $ARCHITECTURES; do
    echo ""
    echo -e "${BLUE}────────────────────────────────────────────────────────${NC}"
    echo -e "${BLUE}Building for linux/${arch}${NC}"
    echo -e "${BLUE}────────────────────────────────────────────────────────${NC}"

    # Create architecture-specific output directory
    OUTPUT_DIR="release/jack-${VERSION}-linux-${arch}"
    mkdir -p "$OUTPUT_DIR"

    # Build using the main build script
    GOARCH=$arch ./build.sh

    # Copy binaries to release directory
    echo -e "\n${GREEN}Copying binaries to ${OUTPUT_DIR}/${NC}"
    cp -v bin/* "$OUTPUT_DIR/"

    # Calculate total size
    TOTAL_SIZE=$(du -sh "$OUTPUT_DIR" | cut -f1)
    echo -e "${GREEN}✓ Build complete for ${arch} (${TOTAL_SIZE})${NC}"
done

echo ""
echo -e "${BLUE}════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}✓ Multi-architecture build complete${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════${NC}"
echo ""
echo "Release artifacts created in: release/"
ls -lh release/
echo ""
echo "Next steps:"
echo "  1. Run ./scripts/package-release.sh to create .deb and .tar.gz packages"
echo "  2. Run ./scripts/create-github-release.sh to upload to GitHub"

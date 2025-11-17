#!/bin/bash
set -e

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")

echo "Building Jack Debian package version ${VERSION}..."
echo ""

# Check if we're on Debian/Ubuntu
if ! command -v dpkg-buildpackage &> /dev/null; then
    echo "Error: dpkg-buildpackage not found. Please install build-essential and devscripts:"
    echo "  sudo apt-get install build-essential devscripts debhelper"
    exit 1
fi

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Error: Go compiler not found. Please install Go 1.24 or later:"
    echo "  sudo apt-get install golang-go"
    exit 1
fi

# Clean previous builds
echo "Cleaning previous builds..."
rm -f ../jack_*.deb ../jack_*.changes ../jack_*.buildinfo
rm -rf debian/jack

# Ensure go.mod dependencies are up to date
echo "Updating Go dependencies..."
go mod tidy

# Build the package
echo "Building package..."
dpkg-buildpackage -us -uc -b

echo ""
echo "✓ Build complete!"
echo ""
echo "Package created:"
ls -lh ../jack_*.deb

echo ""
echo "To install:"
echo "  sudo dpkg -i ../jack_${VERSION}_arm64.deb"
echo ""
echo "Or to install with dependencies:"
echo "  sudo apt-get install ../jack_${VERSION}_arm64.deb"

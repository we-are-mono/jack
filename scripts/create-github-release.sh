#!/bin/bash
# Copyright (C) 2025 Mono Technologies Inc.
#
# This program is free software; you can redistribute it and/or
# modify it under the terms of the GNU General Public License
# as published by the Free Software Foundation; version 2.
#
# Create a GitHub release and upload packages

set -e

# Configuration
VERSION="${1}"
PRERELEASE="${2:-false}"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check if version is provided
if [ -z "$VERSION" ]; then
    echo -e "${RED}Error: Version not specified${NC}"
    echo "Usage: $0 <version> [prerelease]"
    echo ""
    echo "Examples:"
    echo "  $0 v0.1.0           # Create stable release"
    echo "  $0 v0.2.0-beta.1 true  # Create pre-release"
    exit 1
fi

echo -e "${BLUE}════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  Jack GitHub Release Creator${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════${NC}"
echo -e "Version: ${GREEN}${VERSION}${NC}"
echo -e "Pre-release: ${GREEN}${PRERELEASE}${NC}"
echo ""

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo -e "${RED}Error: GitHub CLI (gh) is not installed${NC}"
    echo "Install it from: https://cli.github.com/"
    exit 1
fi

# Check if authenticated
if ! gh auth status &> /dev/null; then
    echo -e "${RED}Error: Not authenticated with GitHub${NC}"
    echo "Run: gh auth login"
    exit 1
fi

# Check if packages exist
if [ ! -d "release/packages" ] || [ -z "$(ls -A release/packages/*.tar.gz 2>/dev/null)" ]; then
    echo -e "${RED}Error: No packages found in release/packages/${NC}"
    echo "Please run:"
    echo "  ./scripts/build-release.sh ${VERSION}"
    echo "  ./scripts/package-release.sh ${VERSION}"
    exit 1
fi

# Check if tag exists
if ! git rev-parse "${VERSION}" >/dev/null 2>&1; then
    echo -e "${YELLOW}Tag ${VERSION} does not exist locally.${NC}"
    echo -e "${YELLOW}Creating tag...${NC}"
    git tag -a "${VERSION}" -m "Release ${VERSION}"
    echo -e "${GREEN}✓ Tag created${NC}"
    echo ""

    echo -e "${YELLOW}Pushing tag to origin...${NC}"
    git push origin "${VERSION}"
    echo -e "${GREEN}✓ Tag pushed${NC}"
    echo ""
fi

# Generate release notes
RELEASE_NOTES_FILE="release/release-notes-${VERSION}.md"
echo -e "${BLUE}Generating release notes...${NC}"

cat > "$RELEASE_NOTES_FILE" << EOF
## Jack ${VERSION}

### Installation

#### Debian/Ubuntu (.deb packages)

\`\`\`bash
# Download for your architecture
wget https://github.com/we-are-mono/jack/releases/download/${VERSION}/jack_${VERSION#v}_arm64.deb
# or
wget https://github.com/we-are-mono/jack/releases/download/${VERSION}/jack_${VERSION#v}_amd64.deb

# Install
sudo apt install ./jack_${VERSION#v}_<arch>.deb
\`\`\`

#### Other Linux distributions (.tar.gz)

\`\`\`bash
# Download for your architecture
wget https://github.com/we-are-mono/jack/releases/download/${VERSION}/jack-${VERSION}-linux-arm64.tar.gz
# or
wget https://github.com/we-are-mono/jack/releases/download/${VERSION}/jack-${VERSION}-linux-amd64.tar.gz

# Extract
tar xzf jack-${VERSION}-linux-<arch>.tar.gz
cd jack-${VERSION}-linux-<arch>

# Install
sudo ./install.sh
\`\`\`

### Supported Architectures

- **linux/arm64** - ARM 64-bit (Raspberry Pi 3/4/5, most modern ARM servers)
- **linux/amd64** - x86_64 (Intel/AMD 64-bit)

### Verifying Downloads

All release artifacts include SHA256 checksums. Verify your download:

\`\`\`bash
# Download checksum file
wget https://github.com/we-are-mono/jack/releases/download/${VERSION}/jack-${VERSION}-linux-arm64.tar.gz.sha256

# Verify
sha256sum -c jack-${VERSION}-linux-arm64.tar.gz.sha256
\`\`\`

### What's Included

This release includes:
- **jack** - Main daemon binary
- **6 core plugins**:
  - jack-plugin-nftables (firewall management)
  - jack-plugin-dnsmasq (DHCP/DNS)
  - jack-plugin-wireguard (VPN)
  - jack-plugin-monitoring (system monitoring)
  - jack-plugin-leds (LED control)
  - jack-plugin-sqlite3 (logging and data storage)

### Documentation

- [Installation Guide](https://github.com/we-are-mono/jack#installation)
- [Configuration Reference](https://github.com/we-are-mono/jack/tree/main/docs)
- [Contributing Guide](https://github.com/we-are-mono/jack/blob/main/docs/development/contributing.md)

---

**Full Changelog**: https://github.com/we-are-mono/jack/commits/${VERSION}
EOF

echo -e "${GREEN}✓ Release notes generated${NC}"
echo ""

# List packages to upload
echo -e "${BLUE}Packages to upload:${NC}"
ls -lh release/packages/
echo ""

# Confirm release
echo -e "${YELLOW}Ready to create GitHub release ${VERSION}${NC}"
echo -e "${YELLOW}Press Enter to continue or Ctrl+C to cancel...${NC}"
read -r

# Create release
echo -e "${BLUE}Creating GitHub release...${NC}"

PRERELEASE_FLAG=""
if [ "$PRERELEASE" = "true" ]; then
    PRERELEASE_FLAG="--prerelease"
fi

gh release create "${VERSION}" \
    --title "Jack ${VERSION}" \
    --notes-file "$RELEASE_NOTES_FILE" \
    $PRERELEASE_FLAG \
    release/packages/*.tar.gz \
    release/packages/*.tar.gz.sha256 \
    release/packages/*.deb \
    release/packages/*.deb.sha256

echo ""
echo -e "${BLUE}════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}✓ GitHub release created successfully${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════${NC}"
echo ""
echo "Release URL:"
gh release view "${VERSION}" --web --json url -q .url
echo ""
echo -e "${GREEN}Release ${VERSION} is now published!${NC}"

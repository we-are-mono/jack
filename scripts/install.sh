#!/bin/bash
# Copyright (C) 2025 Mono Technologies Inc.
#
# This program is free software; you can redistribute it and/or
# modify it under the terms of the GNU General Public License
# as published by the Free Software Foundation; version 2.
#
# Installation script for Jack (tar.gz distribution)

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}Error: This script must be run as root${NC}"
    echo "Please run: sudo $0"
    exit 1
fi

echo -e "${BLUE}════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  Jack Network Gateway - Installation${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════${NC}"
echo ""

# Detect if this is an upgrade
UPGRADE=false
if [ -f "/usr/bin/jack" ]; then
    UPGRADE=true
    OLD_VERSION=$(/usr/bin/jack --version 2>/dev/null | grep -oP 'version \K[^ ]+' || echo "unknown")
    echo -e "${YELLOW}Existing installation detected (version: ${OLD_VERSION})${NC}"
    echo -e "${YELLOW}This will be an upgrade${NC}"
    echo ""
fi

# Stop jack service if running
if systemctl is-active --quiet jack 2>/dev/null; then
    echo -e "${YELLOW}Stopping jack service...${NC}"
    systemctl stop jack
fi

# Install main binary
echo -e "${GREEN}Installing jack daemon...${NC}"
install -D -m 0755 jack /usr/bin/jack

# Install plugins
echo -e "${GREEN}Installing plugins...${NC}"
mkdir -p /usr/lib/jack/plugins
install -m 0755 jack-plugin-* /usr/lib/jack/plugins/

# Count installed plugins
PLUGIN_COUNT=$(ls -1 jack-plugin-* 2>/dev/null | wc -l)
echo -e "  ${GREEN}✓ Installed ${PLUGIN_COUNT} plugins${NC}"

# Install systemd service if available
if [ -f "jack.service" ]; then
    echo -e "${GREEN}Installing systemd service...${NC}"
    install -D -m 0644 jack.service /lib/systemd/system/jack.service
    systemctl daemon-reload
elif [ ! -f "/lib/systemd/system/jack.service" ]; then
    echo -e "${YELLOW}Warning: jack.service not found in package${NC}"
    echo -e "${YELLOW}Creating minimal systemd service...${NC}"

    cat > /lib/systemd/system/jack.service << 'EOF'
[Unit]
Description=Jack Network Gateway Daemon
After=network.target
Documentation=https://github.com/we-are-mono/jack

[Service]
Type=simple
ExecStart=/usr/bin/jack daemon --apply
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
fi

# Create required directories
echo -e "${GREEN}Creating directories...${NC}"
mkdir -p /var/lib/jack
mkdir -p /run/jack
mkdir -p /var/log/jack
mkdir -p /etc/jack

# Install config templates if available
if [ -d "templates" ]; then
    echo -e "${GREEN}Installing configuration templates...${NC}"
    mkdir -p /etc/jack/templates
    cp -r templates/* /etc/jack/templates/ 2>/dev/null || true
fi

# Set permissions
chown -R root:root /usr/bin/jack /usr/lib/jack
chmod 755 /var/lib/jack /run/jack /var/log/jack /etc/jack

echo ""
echo -e "${BLUE}════════════════════════════════════════════════════════${NC}"
if [ "$UPGRADE" = true ]; then
    echo -e "${GREEN}✓ Jack has been upgraded successfully${NC}"
else
    echo -e "${GREEN}✓ Jack has been installed successfully${NC}"
fi
echo -e "${BLUE}════════════════════════════════════════════════════════${NC}"
echo ""

# Show version
NEW_VERSION=$(/usr/bin/jack --version 2>/dev/null | grep -oP 'version \K[^ ]+' || echo "unknown")
echo -e "Installed version: ${GREEN}${NEW_VERSION}${NC}"
echo -e "Installed plugins: ${GREEN}${PLUGIN_COUNT}${NC}"
echo ""

# Provide next steps
echo "Next steps:"
echo ""
echo "  1. Configure Jack:"
echo "     - Edit /etc/jack/jack.json (main configuration)"
echo "     - See /etc/jack/templates/ for examples"
echo ""
echo "  2. Enable and start the service:"
echo "     sudo systemctl enable jack"
echo "     sudo systemctl start jack"
echo ""
echo "  3. Check status:"
echo "     sudo systemctl status jack"
echo "     jack status"
echo ""

if [ "$UPGRADE" = true ]; then
    echo -e "${YELLOW}Note: If jack was running, you may want to restart it:${NC}"
    echo "     sudo systemctl start jack"
    echo ""
fi

echo "For documentation, visit: https://github.com/we-are-mono/jack"

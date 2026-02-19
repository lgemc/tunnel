#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${RED}Tunnel Service Destroy Script${NC}"
echo "=============================="
echo ""
echo -e "${YELLOW}WARNING: This will destroy all infrastructure and delete all data!${NC}"
echo ""

read -p "Are you sure you want to continue? (yes/no): " -r
echo

if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
    echo "Aborted."
    exit 1
fi

echo "Destroying infrastructure..."
cd infra
tofu destroy
cd ..

echo ""
echo -e "${GREEN}✓ Infrastructure destroyed${NC}"
echo ""
echo "You may also want to:"
echo "1. Clean build artifacts: make clean"
echo "2. Remove CLI configuration: rm -rf ~/.tunnel"

#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Tunnel Service Deployment Script${NC}"
echo "=================================="
echo ""

# Check if required tools are installed
check_dependencies() {
    echo "Checking dependencies..."

    if ! command -v tofu &> /dev/null; then
        echo -e "${RED}Error: OpenTofu (tofu) is not installed${NC}"
        echo -e "${YELLOW}Install from: https://opentofu.org/${NC}"
        exit 1
    fi

    if ! command -v go &> /dev/null; then
        echo -e "${RED}Error: go is not installed${NC}"
        exit 1
    fi

    if ! command -v aws &> /dev/null; then
        echo -e "${YELLOW}Warning: AWS CLI is not installed. You may need it for some operations.${NC}"
    fi

    echo -e "${GREEN}✓ All dependencies found${NC}"
    echo ""
}

# Build Lambda functions
build_lambdas() {
    echo "Building Lambda functions..."
    make build-lambdas
    echo ""
}

# Initialize OpenTofu
init_tofu() {
    echo "Initializing OpenTofu..."
    cd infra
    tofu init
    cd ..
    echo -e "${GREEN}✓ OpenTofu initialized${NC}"
    echo ""
}

# Apply OpenTofu
apply_tofu() {
    echo "Applying OpenTofu configuration..."
    cd infra
    tofu apply
    cd ..
    echo -e "${GREEN}✓ OpenTofu applied${NC}"
    echo ""
}

# Get outputs
get_outputs() {
    echo "Deployment outputs:"
    echo "==================="
    cd infra
    tofu output
    cd ..
    echo ""
}

# Main deployment flow
main() {
    check_dependencies
    build_lambdas
    init_tofu
    apply_tofu
    get_outputs

    echo -e "${GREEN}Deployment complete!${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Build the CLI: make build-cli"
    echo "2. Register a client: ./build/tunnel register --api-endpoint=<API_ENDPOINT> --ws-endpoint=<WS_ENDPOINT>"
    echo "3. Start a tunnel: ./build/tunnel start 3000"
}

main

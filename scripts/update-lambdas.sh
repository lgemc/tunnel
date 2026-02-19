#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get script directory and project root
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"

cd "$PROJECT_ROOT"

REGION="us-east-1"
BUILD_DIR="build/lambdas"

# Lambda functions to deploy
FUNCTIONS=(
    "register-client:tunnel-register-client-dev"
    "create-tunnel:tunnel-create-tunnel-dev"
    "delete-tunnel:tunnel-delete-tunnel-dev"
    "list-tunnels:tunnel-list-tunnels-dev"
    "authorize-connection:tunnel-authorize-connection-dev"
    "tunnel-connect:tunnel-tunnel-connect-dev"
    "tunnel-disconnect:tunnel-tunnel-disconnect-dev"
    "tunnel-proxy:tunnel-tunnel-proxy-dev"
    "http-proxy:tunnel-http-proxy-dev"
    "s3-upload-notify:tunnel-s3-upload-notify-dev"
)

echo -e "${GREEN}Deploying Lambda functions to AWS${NC}"
echo "===================================="
echo ""

# Check if build directory exists
if [ ! -d "$BUILD_DIR" ]; then
    echo "Error: Build directory not found. Run 'make build-lambdas' first."
    exit 1
fi

# Deploy each function
for func in "${FUNCTIONS[@]}"; do
    IFS=':' read -r zip_name lambda_name <<< "$func"
    zip_file="$BUILD_DIR/${zip_name}.zip"

    if [ ! -f "$zip_file" ]; then
        echo -e "${YELLOW}Warning: $zip_file not found, skipping...${NC}"
        continue
    fi

    echo "Deploying $lambda_name..."
    aws lambda update-function-code \
        --region "$REGION" \
        --function-name "$lambda_name" \
        --zip-file "fileb://$zip_file" \
        --output text --query 'LastModified' > /dev/null

    echo -e "${GREEN}✓ $lambda_name deployed${NC}"
done

echo ""
echo -e "${GREEN}Deployment complete!${NC}"

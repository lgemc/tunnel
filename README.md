# Tunnel Service

A serverless tunneling service built on AWS that allows clients to expose local services to the internet through secure tunnels, similar to ngrok or tunnel.to.

## Features

- 🚀 **Serverless Architecture** - Built entirely on AWS serverless infrastructure (Lambda, API Gateway, DynamoDB)
- 🔒 **Secure Tunnels** - TLS-encrypted connections with API key authentication
- 🌐 **Custom Domains** - Support for custom subdomains or auto-generated random domains
- 📦 **WebSocket-based** - Efficient bidirectional communication for tunnel traffic
- 🛠️ **Easy CLI** - Simple command-line interface for managing tunnels
- ☁️ **CloudFront CDN** - Global distribution with low latency

## Architecture

### Components

- **API Gateway (REST)** - Control plane for client registration and tunnel management
- **API Gateway (WebSocket)** - Data plane for tunnel traffic
- **Lambda Functions** - Serverless compute for all operations
- **DynamoDB** - NoSQL database for clients, tunnels, and domain mappings
- **CloudFront** - CDN for serving tunnel traffic globally
- **Route53** - DNS management for wildcard subdomains
- **ACM** - SSL/TLS certificates

### Project Structure

```
/
├── infra/              # Terraform infrastructure as code
│   ├── main.tf
│   ├── variables.tf
│   ├── dynamodb.tf
│   ├── lambda.tf
│   ├── apigateway.tf
│   ├── cloudfront.tf
│   └── outputs.tf
├── lambdas/            # Go Lambda functions
│   ├── shared/         # Shared libraries
│   │   ├── auth/
│   │   ├── db/
│   │   └── models/
│   ├── register-client/
│   ├── create-tunnel/
│   ├── delete-tunnel/
│   ├── list-tunnels/
│   ├── authorize-connection/
│   ├── tunnel-connect/
│   ├── tunnel-disconnect/
│   └── tunnel-proxy/
├── cli/                # Go CLI application
│   ├── cmd/            # CLI commands
│   ├── internal/       # Internal packages
│   │   ├── client/
│   │   ├── proxy/
│   │   └── config/
│   └── main.go
├── scripts/            # Deployment scripts
│   ├── deploy.sh
│   └── destroy.sh
├── Makefile
└── README.md
```

## Prerequisites

- **Go 1.22+** - For building Lambda functions and CLI
- **OpenTofu (tofu)** - For infrastructure deployment (open-source Terraform alternative)
- **AWS Account** - With appropriate permissions
- **AWS CLI** - Configured with credentials (optional but recommended)
- **Domain Name** - For custom tunnel domains (optional)

> **Note:** This project uses OpenTofu instead of Terraform. Install it from https://opentofu.org/

## Quick Start

### 1. Clone the Repository

```bash
git clone <repository-url>
cd tunnel
```

### 2. Configure Infrastructure

Edit `infra/variables.tf` or create a `terraform.tfvars` file:

```hcl
aws_region   = "us-east-1"
environment  = "dev"
domain_name  = "tunnel.example.com"  # Your domain
```

### 3. Deploy Infrastructure

```bash
# Option 1: Using the deployment script
./scripts/deploy.sh

# Option 2: Using Makefile
make deps                  # Download dependencies
make build-lambdas        # Build Lambda functions
make deploy-init          # Initialize OpenTofu
make deploy-apply         # Deploy infrastructure
```

### 4. Build the CLI

```bash
make build-cli
```

The CLI binary will be available at `build/tunnel`.

### 5. Register a Client

```bash
./build/tunnel register \
  --api-endpoint=<REST_API_ENDPOINT> \
  --ws-endpoint=<WEBSOCKET_API_ENDPOINT>
```

Get the endpoints from OpenTofu outputs:
```bash
cd infra && tofu output
```

### 6. Start a Tunnel

```bash
# Start tunnel with random subdomain
./build/tunnel start 3000

# Start tunnel with custom subdomain
./build/tunnel start 8080 --domain myapp
```

## CLI Usage

### Commands

```bash
tunnel register                    # Register a new client
tunnel start [port]                # Start a tunnel
tunnel start [port] --domain NAME  # Start with custom subdomain
tunnel list                        # List all tunnels
tunnel stop [tunnel-id]            # Stop a specific tunnel
tunnel status                      # Show configuration status
```

### Examples

```bash
# Register with the service
tunnel register --api-endpoint=https://api.example.com --ws-endpoint=wss://ws.example.com

# Expose local web server on port 3000
tunnel start 3000

# Expose with custom subdomain
tunnel start 8080 --domain myapp
# Now accessible at: https://myapp.tunnel.example.com

# List active tunnels
tunnel list

# Stop a tunnel
tunnel stop abc123def456

# Check status
tunnel status
```

## Development

### Building

```bash
# Build Lambda functions
make build-lambdas

# Build CLI for current platform
make build-cli

# Build CLI for all platforms
make build-cli-all

# Build everything
make build-lambdas build-cli
```

### Testing

```bash
# Run Lambda tests
make test-lambdas

# Run CLI tests
make test-cli

# Run all tests
make test
```

### Code Quality

```bash
# Format code
make fmt

# Lint code (requires golangci-lint)
make lint
```

### Cleaning

```bash
# Remove build artifacts and OpenTofu state
make clean
```

## Deployment

### Initial Deployment

1. Configure AWS credentials:
   ```bash
   aws configure
   ```

2. Set up your domain (if using custom domains):
   - Create a Route53 hosted zone
   - Request an ACM certificate for `*.tunnel.example.com` in `us-east-1`
   - Update `infra/variables.tf` with zone ID and certificate ARN

3. Deploy:
   ```bash
   ./scripts/deploy.sh
   ```

> **Note:** All scripts use OpenTofu (tofu command) instead of Terraform.

### Updating Infrastructure

```bash
# Make changes to infrastructure files
cd infra

# Preview changes
tofu plan

# Apply changes
tofu apply
```

### Updating Lambda Functions

```bash
# Rebuild and redeploy
make build-lambdas
cd infra && tofu apply
```

### Destroying Infrastructure

```bash
# Option 1: Using script
./scripts/destroy.sh

# Option 2: Using Makefile
make deploy-destroy
```

## Configuration

### Environment Variables

Lambda functions use the following environment variables (automatically set by OpenTofu):

- `CLIENTS_TABLE` - DynamoDB clients table name
- `TUNNELS_TABLE` - DynamoDB tunnels table name
- `DOMAINS_TABLE` - DynamoDB domains table name
- `DOMAIN_NAME` - Base domain for tunnels
- `WEBSOCKET_API_URL` - WebSocket API endpoint
- `WEBSOCKET_API_STAGE` - WebSocket API stage name

### CLI Configuration

The CLI stores configuration in `~/.tunnel/config.yaml`:

```yaml
api_endpoint: https://api.example.com
websocket_endpoint: wss://ws.example.com
api_key: tk_xxxxxxxxxxxxxxxxxxxxx
client_id: abc123def456
```

## Security Considerations

1. **API Key Authentication** - All API requests require a valid API key
2. **TLS Encryption** - All connections use TLS/SSL
3. **WebSocket Authorization** - Custom authorizer validates connections
4. **Client Isolation** - Clients can only manage their own tunnels
5. **Secure Storage** - API keys are hashed using bcrypt before storage

## Cost Estimation

AWS costs will vary based on usage. Main cost factors:

- **API Gateway** - Per million requests
- **Lambda** - Per million requests and compute time
- **DynamoDB** - Pay-per-request or provisioned capacity
- **CloudFront** - Data transfer and requests
- **Route53** - Hosted zone and queries

For development/testing with low usage, expect costs under $5-10/month.

## Troubleshooting

### Tunnel won't connect

1. Check that the tunnel was created: `tunnel list`
2. Verify your local service is running on the specified port
3. Check API key is valid: `tunnel status`
4. Review CloudWatch logs for Lambda functions

### Domain not resolving

1. Verify Route53 wildcard record is configured
2. Check ACM certificate is validated and in `us-east-1`
3. Allow time for DNS propagation (up to 24-48 hours)

### Build errors

1. Ensure Go 1.22+ is installed: `go version`
2. Download dependencies: `make deps`
3. Clean and rebuild: `make clean && make build-lambdas build-cli`

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

[Add your license here]

## Acknowledgments

Inspired by ngrok and other tunneling services.

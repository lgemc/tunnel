# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Serverless HTTP tunneling service (similar to ngrok) built on AWS. Exposes local services via public subdomains using API Gateway WebSockets + Lambda + DynamoDB.

**IaC Tool**: This project uses **OpenTofu** (`tofu` command), NOT Terraform.

## Two Go Modules

This repo contains two independent Go modules — always run Go commands from within the correct directory:

- `lambdas/` — Lambda functions (Go 1.23, `github.com/lmanrique/tunnel/lambdas`)
- `cli/` — CLI application (Go 1.22, `github.com/lmanrique/tunnel/cli`)

## Common Commands

```bash
# Build all Lambda functions (GOOS=linux GOARCH=amd64)
make build-lambdas

# Build CLI binary
make build-cli

# Run tests
make test             # both modules
make test-lambdas     # lambdas only
make test-cli         # CLI only

# Format code
make fmt

# Lint (requires golangci-lint)
make lint

# Install CLI to /usr/local/bin/tunnel
make install-cli

# Infrastructure
make deploy-plan      # cd infra && tofu plan
make deploy-apply     # cd infra && tofu apply

# Fast Lambda update (no tofu apply needed)
make build-lambdas && ./scripts/update-lambdas.sh

# Full deploy from scratch
./scripts/deploy.sh

# Tear down
./scripts/destroy.sh

# Local test server (echo HTTP server on port 3001)
node test-server.js
```

## Architecture

### Request Flow

```
External HTTP → REST API GW (/t/{subdomain}/{proxy+})
  → http-proxy Lambda
    → DynamoDB: domain → tunnel_id → connection_id
    → Store PendingRequest in DynamoDB (TTL 5min)
    → Forward request via WebSocket (apigatewaymanagementapi.PostToConnection)
    → Poll DynamoDB every 200ms (25s timeout) for response
    → Return response to caller

CLI (tunnel start <port>) ← WebSocket ← tunnel-proxy Lambda
  → Receives "proxy" message
  → Forwards to localhost:{port}
  → Sends "proxy_response" back via WebSocket
  → tunnel-proxy updates PendingRequest in DynamoDB to "completed"
```

### REST API (Control Plane)

| Route | Lambda | Purpose |
|-------|--------|---------|
| `POST /clients` | `register-client` | Create client; API key shown once |
| `POST /tunnels` | `create-tunnel` | Create tunnel + domain record |
| `GET /tunnels` | `list-tunnels` | List client's tunnels (GSI on client_id) |
| `DELETE /tunnels/{tunnel_id}` | `delete-tunnel` | Delete tunnel + domain |
| `ANY /t/{subdomain}/{proxy+}` | `http-proxy` | Proxy HTTP through active tunnel |

### WebSocket API (Data Plane)

| Route | Lambda | Purpose |
|-------|--------|---------|
| `$connect` | `authorize-connection` + `tunnel-connect` | Auth via API key, associate connection_id |
| `$disconnect` | `tunnel-disconnect` | Mark tunnel inactive |
| `$default` | `tunnel-proxy` | Handle PING/RESPONSE/proxy_response messages |

### DynamoDB Tables (suffix: `-dev`)

- `tunnel-clients-dev` — client_id → bcrypt hash of API key
- `tunnel-tunnels-dev` — tunnel_id → connection_id, status; GSI on client_id
- `tunnel-domains-dev` — domain → tunnel_id
- `tunnel-pending-requests-dev` — request_id → request/response correlation (TTL-enabled)

### Authentication

API keys are prefixed `tk_`, generated with 32 random bytes, stored as bcrypt hashes. Auth uses `Authorization: Bearer <key>` header. **Known limitation**: auth verification does a full DynamoDB table scan (not production-grade).

### Shared Lambda Code (`lambdas/shared/`)

- `auth/auth.go` — API key generation/hashing, ID generation, subdomain validation
- `db/db.go` — DynamoDB client wrapper (PutItem, GetItem, DeleteItem, Query, UpdateItem, Scan)
- `models/models.go` — Domain models (Client, Tunnel, Domain) and WebSocket message types

### CLI Config

Stored at `~/.tunnel/config.yaml` (managed by Viper). CLI commands: `register`, `start <port>`, `list`, `stop <tunnel-id>`, `status`.

## AWS Environment

- Region: `us-east-1`
- Environment: `dev`
- Domain: `tunnel.atelier.run`

## Lambda Build Details

Lambdas are compiled with `GOOS=linux GOARCH=amd64 CGO_ENABLED=0` and tagged `-tags lambda.norpc`. Each Lambda is zipped as `bootstrap` in `build/lambdas/<name>.zip`.

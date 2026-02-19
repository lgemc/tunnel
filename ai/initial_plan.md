# Tunnel Service - Initial Plan

## Overview
A tunneling service similar to tunnel.to/ngrok that allows clients to expose local services to the internet through secure tunnels. Built on AWS serverless infrastructure.

## Architecture Components

### 1. Cloud Infrastructure (AWS + Terraform)

#### API Gateway
- REST API for control plane operations (client registration, tunnel management)
- WebSocket API for tunnel data plane (persistent connections for tunnel traffic)

#### Lambda Functions
- **Control Plane**:
  - `register-client`: Handle client registration and authentication
  - `create-tunnel`: Create new tunnel, assign domain, update DynamoDB
  - `delete-tunnel`: Remove tunnel and cleanup resources
  - `list-tunnels`: Get tunnels for a client
  - `authorize-connection`: Validate client credentials for WebSocket connections

- **Data Plane**:
  - `tunnel-connect`: Handle WebSocket connection establishment
  - `tunnel-disconnect`: Handle WebSocket disconnection cleanup
  - `tunnel-proxy`: Forward HTTP/HTTPS requests to client through WebSocket

#### DynamoDB Tables
- **clients**:
  - PK: `client_id` (UUID)
  - Attributes: `created_at`, `api_key_hash`, `status`

- **tunnels**:
  - PK: `tunnel_id` (UUID)
  - GSI: `client_id-index` for querying tunnels by client
  - Attributes: `client_id`, `domain`, `subdomain`, `status`, `created_at`, `connection_id` (WebSocket)

- **domains**:
  - PK: `domain` (e.g., "abc123.tunnel.example.com")
  - Attributes: `tunnel_id`, `client_id`, `created_at`

#### CloudFront
- Distribution for serving tunnel traffic
- Custom domain support (*.tunnel.example.com)
- Origin: API Gateway (WebSocket for tunnel proxy)
- Cache behaviors for static content if needed

#### Additional AWS Resources
- **Route53**: DNS management for custom domains and wildcard subdomains
- **ACM**: SSL/TLS certificates for *.tunnel.example.com
- **CloudWatch**: Logging and monitoring
- **IAM**: Roles and policies for Lambda execution

### 2. CLI Client (Go)

#### Commands
```
tunnel auth login                    # Authenticate client, get/store API key
tunnel auth logout                   # Remove stored credentials
tunnel register                      # Register new client ID
tunnel start [port] [--domain]       # Start tunnel on local port
tunnel list                          # List active tunnels
tunnel stop [tunnel-id]              # Stop specific tunnel
tunnel status                        # Show connection status
```

#### Functionality
- Establish WebSocket connection to AWS
- Proxy local HTTP traffic to/from tunnel
- Handle reconnection and keep-alive
- Configuration management (~/.tunnel/config)
- Request/response logging (optional)

### 3. Backend Services (Go Lambdas)

#### Tunnel Protocol
- WebSocket-based bidirectional communication
- Message types:
  - `CONNECT`: Client initiates tunnel
  - `REQUEST`: Incoming HTTP request from internet
  - `RESPONSE`: Client response to proxied request
  - `PING/PONG`: Keep-alive
  - `ERROR`: Error conditions

## Data Flow

### Tunnel Creation
1. Client runs `tunnel start 3000 --domain myapp`
2. Client calls REST API `/tunnels` with authentication
3. Lambda validates client, generates/validates subdomain
4. Lambda creates DynamoDB records (tunnels, domains tables)
5. Lambda returns tunnel info (domain, tunnel_id, websocket_url)
6. Client establishes WebSocket connection
7. Lambda updates tunnel record with connection_id

### Request Proxying
1. External user hits `myapp.tunnel.example.com`
2. CloudFront → API Gateway → WebSocket connection
3. Lambda identifies tunnel by domain, looks up connection_id
4. Lambda sends REQUEST message to client via WebSocket
5. Client receives request, forwards to localhost:3000
6. Client sends RESPONSE message back via WebSocket
7. Lambda forwards response to original requester

### Tunnel Cleanup
1. Client disconnects or runs `tunnel stop`
2. Disconnect Lambda triggers
3. Update tunnel status to "inactive" in DynamoDB
4. Clean up domain mapping if needed

## Domain Management

### Random Domains
- Generate random subdomain (e.g., `a4f8k2.tunnel.example.com`)
- Check uniqueness in DynamoDB domains table
- Retry if collision

### Custom Domains
- Validate format (alphanumeric, hyphens)
- Check availability in domains table
- Optional: allow only for premium clients

## Security Considerations

1. **Authentication**:
   - API key based authentication for clients
   - Store hashed keys in DynamoDB
   - JWT tokens for WebSocket connections

2. **Authorization**:
   - Verify client owns tunnel before proxying
   - Rate limiting per client

3. **Data Security**:
   - TLS for all connections (CloudFront, API Gateway)
   - Validate request/response sizes
   - Timeout handling for hung requests

## Implementation Phases

### Phase 1: Infrastructure Foundation
- [ ] Set up Terraform project structure
- [ ] Define DynamoDB tables
- [ ] Create API Gateway (REST + WebSocket)
- [ ] Set up CloudFront distribution
- [ ] Configure Route53 and ACM
- [ ] Deploy basic Lambda placeholders

### Phase 2: Control Plane
- [ ] Implement client registration Lambda
- [ ] Implement tunnel CRUD Lambdas
- [ ] Add authentication/authorization
- [ ] Test control plane APIs

### Phase 3: CLI Client
- [ ] Set up Go project structure
- [ ] Implement auth commands
- [ ] Implement register command
- [ ] Implement basic WebSocket client
- [ ] Add configuration management

### Phase 4: Data Plane
- [ ] Implement WebSocket connect/disconnect handlers
- [ ] Implement tunnel proxy Lambda
- [ ] Add request routing logic
- [ ] Integrate CloudFront with tunnels

### Phase 5: CLI Tunneling
- [ ] Implement tunnel start command
- [ ] Add local proxy server
- [ ] Handle request/response forwarding
- [ ] Add reconnection logic
- [ ] Implement tunnel list/stop commands

### Phase 6: Polish & Production
- [ ] Add comprehensive logging
- [ ] Implement monitoring and alerts
- [ ] Add rate limiting
- [ ] Performance optimization
- [ ] Documentation
- [ ] Load testing

## Project Structure

```
/
├── infra/
│   ├── main.tf
│   ├── variables.tf
│   ├── dynamodb.tf
│   ├── lambda.tf
│   ├── apigateway.tf
│   ├── cloudfront.tf
│   └── outputs.tf
├── lambdas/
│   ├── register-client/
│   │   └── main.go
│   ├── create-tunnel/
│   │   └── main.go
│   ├── tunnel-proxy/
│   │   └── main.go
│   └── shared/
│       ├── auth/
│       ├── dynamodb/
│       └── models/
├── cli/
│   ├── cmd/
│   │   ├── root.go
│   │   ├── auth.go
│   │   ├── start.go
│   │   └── ...
│   ├── internal/
│   │   ├── client/
│   │   ├── proxy/
│   │   └── config/
│   └── main.go
└── ai/
    └── initial_plan.md (this file)
```

## Technology Stack Summary

- **Language**: Go 1.22+
- **Infrastructure**: Terraform
- **Cloud Provider**: AWS
  - Lambda (Go runtime)
  - DynamoDB
  - API Gateway (REST + WebSocket)
  - CloudFront
  - Route53
  - ACM
  - CloudWatch
- **CLI Framework**: cobra
- **WebSocket**: gorilla/websocket
- **Testing**: Go standard testing + localstack for infra testing

## Next Steps

1. Initialize Terraform project
2. Create basic DynamoDB schema
3. Develop and deploy first Lambda (register-client)
4. Build CLI skeleton with auth command
5. Iterate through implementation phases

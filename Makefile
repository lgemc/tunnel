.PHONY: help build-lambdas build-cli clean deploy test

LAMBDA_FUNCTIONS := register-client create-tunnel delete-tunnel list-tunnels authorize-connection tunnel-connect tunnel-disconnect tunnel-proxy http-proxy
BUILD_DIR := build
LAMBDA_DIR := lambdas
CLI_DIR := cli
BACKOFFICE_API_DIR := backoffice/api
BACKOFFICE_FRONTEND_DIR := backoffice/frontend
BACKOFFICE_INFRA_DIR := infra/backoffice

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build-lambdas: ## Build all Lambda functions
	@echo "Building Lambda functions..."
	@mkdir -p $(BUILD_DIR)/lambdas
	@for func in $(LAMBDA_FUNCTIONS); do \
		echo "Building $$func..."; \
		cd $(LAMBDA_DIR)/$$func && \
		GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap main.go && \
		zip -j ../../$(BUILD_DIR)/lambdas/$$func.zip bootstrap && \
		rm bootstrap && \
		cd ../..; \
	done
	@echo "✓ Lambda functions built successfully!"

build-cli: ## Build CLI for current platform
	@echo "Building CLI..."
	@mkdir -p $(BUILD_DIR)
	@cd $(CLI_DIR) && go build -o ../$(BUILD_DIR)/tunnel main.go
	@echo "✓ CLI built successfully: $(BUILD_DIR)/tunnel"

build-cli-all: ## Build CLI for all platforms
	@echo "Building CLI for all platforms..."
	@mkdir -p $(BUILD_DIR)/{linux,darwin,windows}
	@echo "Building for Linux..."
	@cd $(CLI_DIR) && GOOS=linux GOARCH=amd64 go build -o ../$(BUILD_DIR)/linux/tunnel main.go
	@echo "Building for macOS (Intel)..."
	@cd $(CLI_DIR) && GOOS=darwin GOARCH=amd64 go build -o ../$(BUILD_DIR)/darwin/tunnel-amd64 main.go
	@echo "Building for macOS (Apple Silicon)..."
	@cd $(CLI_DIR) && GOOS=darwin GOARCH=arm64 go build -o ../$(BUILD_DIR)/darwin/tunnel-arm64 main.go
	@echo "Building for Windows..."
	@cd $(CLI_DIR) && GOOS=windows GOARCH=amd64 go build -o ../$(BUILD_DIR)/windows/tunnel.exe main.go
	@echo "✓ CLI built for all platforms!"

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -rf $(LAMBDA_DIR)/**/bootstrap
	@rm -rf infra/.terraform
	@rm -rf infra/.terraform.lock.hcl
	@rm -rf infra/terraform.tfstate*
	@rm -rf infra/.tofu
	@rm -rf infra/.tofu.lock.hcl
	@echo "✓ Clean complete!"

test-lambdas: ## Run tests for Lambda functions
	@echo "Running Lambda tests..."
	@cd $(LAMBDA_DIR) && go test ./... -v

test-cli: ## Run tests for CLI
	@echo "Running CLI tests..."
	@cd $(CLI_DIR) && go test ./... -v

test: test-lambdas test-cli ## Run all tests

deploy-init: ## Initialize OpenTofu
	@echo "Initializing OpenTofu..."
	@cd infra && tofu init
	@echo "✓ OpenTofu initialized!"

deploy-plan: ## Plan OpenTofu deployment
	@echo "Planning OpenTofu deployment..."
	@cd infra && tofu plan
	@echo "✓ Plan complete!"

deploy-apply: build-lambdas ## Apply OpenTofu deployment
	@echo "Applying OpenTofu deployment..."
	@echo "⚠️  This will create resources in AWS and may incur costs."
	@cd infra && tofu apply
	@echo "✓ Deployment complete!"

deploy-destroy: ## Destroy OpenTofu infrastructure
	@echo "Destroying OpenTofu infrastructure..."
	@echo "⚠️  This will delete all resources in AWS."
	@cd infra && tofu destroy
	@echo "✓ Infrastructure destroyed!"

install-cli: build-cli ## Install CLI to /usr/local/bin
	@echo "Installing CLI..."
	@sudo cp $(BUILD_DIR)/tunnel /usr/local/bin/tunnel
	@sudo chmod +x /usr/local/bin/tunnel
	@echo "✓ CLI installed to /usr/local/bin/tunnel"

deps-lambdas: ## Download Lambda dependencies
	@echo "Downloading Lambda dependencies..."
	@cd $(LAMBDA_DIR) && go mod download
	@echo "✓ Lambda dependencies downloaded!"

deps-cli: ## Download CLI dependencies
	@echo "Downloading CLI dependencies..."
	@cd $(CLI_DIR) && go mod download
	@echo "✓ CLI dependencies downloaded!"

deps: deps-lambdas deps-cli ## Download all dependencies

fmt: ## Format Go code
	@echo "Formatting code..."
	@cd $(LAMBDA_DIR) && go fmt ./...
	@cd $(CLI_DIR) && go fmt ./...
	@echo "✓ Code formatted!"

lint: ## Lint Go code
	@echo "Linting code..."
	@cd $(LAMBDA_DIR) && golangci-lint run ./...
	@cd $(CLI_DIR) && golangci-lint run ./...
	@echo "✓ Code linted!"

## ── Backoffice ────────────────────────────────────────────────────────────────

build-backoffice-api: ## Build the backoffice API Lambda
	@echo "Building backoffice API Lambda..."
	@mkdir -p $(BUILD_DIR)/lambdas
	@cd $(BACKOFFICE_API_DIR) && \
		GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap main.go && \
		zip -j ../../$(BUILD_DIR)/lambdas/backoffice-api.zip bootstrap && \
		rm bootstrap
	@echo "✓ Backoffice API Lambda built: $(BUILD_DIR)/lambdas/backoffice-api.zip"

build-backoffice-frontend: ## Build the backoffice React frontend
	@echo "Building backoffice frontend..."
	@cd $(BACKOFFICE_FRONTEND_DIR) && npm install && npm run build
	@echo "✓ Backoffice frontend built: $(BACKOFFICE_FRONTEND_DIR)/dist"

build-backoffice: build-backoffice-api build-backoffice-frontend ## Build both backoffice API and frontend

deploy-backoffice-init: ## Initialize OpenTofu for backoffice infra
	@echo "Initializing backoffice OpenTofu..."
	@cd $(BACKOFFICE_INFRA_DIR) && tofu init

deploy-backoffice-plan: ## Plan backoffice OpenTofu deployment
	@cd $(BACKOFFICE_INFRA_DIR) && tofu plan

deploy-backoffice-apply: build-backoffice ## Apply backoffice infra + deploy code
	@cd $(BACKOFFICE_INFRA_DIR) && tofu apply
	@$(MAKE) update-backoffice

update-backoffice: build-backoffice-api ## Fast-update: rebuild API Lambda + push to S3 + invalidate CDN
	@echo "Updating backoffice Lambda..."
	@FUNC=$$(cd $(BACKOFFICE_INFRA_DIR) && tofu output -raw backoffice_lambda_name 2>/dev/null) && \
		aws lambda update-function-code \
			--function-name "$$FUNC" \
			--zip-file fileb://$(BUILD_DIR)/lambdas/backoffice-api.zip \
			--no-cli-pager && \
		echo "✓ Lambda updated: $$FUNC"
	@echo "Syncing frontend to S3..."
	@BUCKET=$$(cd $(BACKOFFICE_INFRA_DIR) && tofu output -raw frontend_s3_bucket 2>/dev/null) && \
		aws s3 sync $(BACKOFFICE_FRONTEND_DIR)/dist s3://$$BUCKET --delete && \
		echo "✓ Frontend synced to s3://$$BUCKET"
	@echo "Invalidating CloudFront cache..."
	@DIST_ID=$$(cd $(BACKOFFICE_INFRA_DIR) && tofu output -raw cloudfront_distribution_id 2>/dev/null) && \
		aws cloudfront create-invalidation --distribution-id "$$DIST_ID" --paths "/*" --no-cli-pager && \
		echo "✓ CloudFront cache invalidated"

deploy-backoffice-destroy: ## Destroy backoffice infrastructure
	@cd $(BACKOFFICE_INFRA_DIR) && tofu destroy

.DEFAULT_GOAL := help

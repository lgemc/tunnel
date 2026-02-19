package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/lmanrique/tunnel/lambdas/shared/auth"
	"github.com/lmanrique/tunnel/lambdas/shared/db"
	"github.com/lmanrique/tunnel/lambdas/shared/models"
)

var (
	clientsTable      string
	tunnelsTable      string
	domainsTable      string
	domainName        string
	websocketAPIURL   string
	websocketAPIStage string
	dbClient          *db.DynamoDBClient
)

func init() {
	clientsTable = os.Getenv("CLIENTS_TABLE")
	tunnelsTable = os.Getenv("TUNNELS_TABLE")
	domainsTable = os.Getenv("DOMAINS_TABLE")
	domainName = os.Getenv("DOMAIN_NAME")
	websocketAPIURL = os.Getenv("WEBSOCKET_API_URL")
	websocketAPIStage = os.Getenv("WEBSOCKET_API_STAGE")

	if clientsTable == "" || tunnelsTable == "" || domainsTable == "" || domainName == "" {
		panic("Required environment variables are missing")
	}
}

type CreateTunnelRequest struct {
	Subdomain string `json:"subdomain,omitempty"`
}

type CreateTunnelResponse struct {
	TunnelID      string `json:"tunnel_id"`
	Domain        string `json:"domain"`
	Subdomain     string `json:"subdomain"`
	WebsocketURL  string `json:"websocket_url"`
	Status        string `json:"status"`
	Message       string `json:"message"`
}

func handler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	// Initialize DB client if not already done
	if dbClient == nil {
		var err error
		dbClient, err = db.NewDynamoDBClient(ctx)
		if err != nil {
			return errorResponse(500, fmt.Sprintf("Failed to initialize database: %v", err))
		}
	}

	// Extract and verify API key
	authHeader := request.Headers["authorization"]
	if authHeader == "" {
		authHeader = request.Headers["Authorization"]
	}

	apiKey, err := auth.ExtractBearerToken(authHeader)
	if err != nil {
		return errorResponse(401, "Invalid authorization header")
	}

	// Verify client exists and get client ID
	clientID, err := verifyClientAPIKey(ctx, apiKey)
	if err != nil {
		return errorResponse(401, "Invalid API key")
	}

	// Parse request body
	var req CreateTunnelRequest
	if request.Body != "" {
		if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
			return errorResponse(400, "Invalid request body")
		}
	}

	// Generate or validate subdomain
	var subdomain string
	if req.Subdomain != "" {
		if !auth.ValidateSubdomain(req.Subdomain) {
			return errorResponse(400, "Invalid subdomain format")
		}
		subdomain = strings.ToLower(req.Subdomain)

		// Check if subdomain is available
		available, err := isSubdomainAvailable(ctx, subdomain)
		if err != nil {
			return errorResponse(500, fmt.Sprintf("Failed to check subdomain availability: %v", err))
		}
		if !available {
			return errorResponse(409, "Subdomain is already taken")
		}
	} else {
		// Generate random subdomain
		subdomain, err = generateUniqueSubdomain(ctx)
		if err != nil {
			return errorResponse(500, fmt.Sprintf("Failed to generate subdomain: %v", err))
		}
	}

	// Generate tunnel ID
	tunnelID, err := auth.GenerateTunnelID()
	if err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to generate tunnel ID: %v", err))
	}

	// Create full domain
	fullDomain := fmt.Sprintf("%s.%s", subdomain, domainName)

	// Create tunnel record
	tunnel := models.Tunnel{
		TunnelID:  tunnelID,
		ClientID:  clientID,
		Domain:    fullDomain,
		Subdomain: subdomain,
		Status:    models.TunnelStatusInactive, // Will be active when WebSocket connects
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create domain record
	domain := models.Domain{
		Domain:    fullDomain,
		TunnelID:  tunnelID,
		ClientID:  clientID,
		CreatedAt: time.Now(),
	}

	// Save to DynamoDB
	if err := dbClient.PutItem(ctx, tunnelsTable, tunnel); err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to save tunnel: %v", err))
	}

	if err := dbClient.PutItem(ctx, domainsTable, domain); err != nil {
		// Rollback tunnel creation
		_ = deleteTunnel(ctx, tunnelID)
		return errorResponse(500, fmt.Sprintf("Failed to save domain: %v", err))
	}

	// Build WebSocket URL
	wsURL := fmt.Sprintf("%s/%s?tunnel_id=%s", websocketAPIURL, websocketAPIStage, tunnelID)

	// Return response
	response := CreateTunnelResponse{
		TunnelID:     tunnelID,
		Domain:       fullDomain,
		Subdomain:    subdomain,
		WebsocketURL: wsURL,
		Status:       tunnel.Status,
		Message:      "Tunnel created successfully. Connect via WebSocket to activate.",
	}

	return successResponse(201, response)
}

func verifyClientAPIKey(ctx context.Context, apiKey string) (string, error) {
	// This is a simplified implementation. In production, you might want to cache this
	// or use a more efficient lookup method.
	// For now, we'll scan all clients (not recommended for production)
	var clients []models.Client
	if err := dbClient.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(clientsTable),
	}, &clients); err != nil {
		return "", err
	}

	for _, client := range clients {
		if auth.VerifyAPIKey(apiKey, client.APIKeyHash) && client.Status == models.ClientStatusActive {
			return client.ClientID, nil
		}
	}

	return "", fmt.Errorf("client not found or inactive")
}

func isSubdomainAvailable(ctx context.Context, subdomain string) (bool, error) {
	fullDomain := fmt.Sprintf("%s.%s", subdomain, domainName)

	key := map[string]types.AttributeValue{
		"domain": &types.AttributeValueMemberS{Value: fullDomain},
	}

	var domain models.Domain
	err := dbClient.GetItem(ctx, domainsTable, key, &domain)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return true, nil
		}
		return false, err
	}

	return false, nil
}

func generateUniqueSubdomain(ctx context.Context) (string, error) {
	maxAttempts := 10
	for i := 0; i < maxAttempts; i++ {
		subdomain, err := auth.GenerateRandomSubdomain()
		if err != nil {
			return "", err
		}

		available, err := isSubdomainAvailable(ctx, subdomain)
		if err != nil {
			return "", err
		}

		if available {
			return subdomain, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique subdomain after %d attempts", maxAttempts)
}

func deleteTunnel(ctx context.Context, tunnelID string) error {
	key := map[string]types.AttributeValue{
		"tunnel_id": &types.AttributeValueMemberS{Value: tunnelID},
	}

	return dbClient.DeleteItem(ctx, tunnelsTable, key)
}

func successResponse(statusCode int, data interface{}) (events.APIGatewayV2HTTPResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return errorResponse(500, "Failed to marshal response")
	}

	return events.APIGatewayV2HTTPResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}, nil
}

func errorResponse(statusCode int, message string) (events.APIGatewayV2HTTPResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"error": message,
	})

	return events.APIGatewayV2HTTPResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}, nil
}

func main() {
	lambda.Start(handler)
}

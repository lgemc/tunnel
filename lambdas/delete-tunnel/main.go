package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

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
	clientsTable string
	tunnelsTable string
	domainsTable string
	dbClient     *db.DynamoDBClient
)

func init() {
	clientsTable = os.Getenv("CLIENTS_TABLE")
	tunnelsTable = os.Getenv("TUNNELS_TABLE")
	domainsTable = os.Getenv("DOMAINS_TABLE")

	if clientsTable == "" || tunnelsTable == "" || domainsTable == "" {
		panic("Required environment variables are missing")
	}
}

type DeleteTunnelResponse struct {
	Message string `json:"message"`
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

	// Get tunnel ID from path parameters
	tunnelID := request.PathParameters["tunnel_id"]
	if tunnelID == "" {
		return errorResponse(400, "Tunnel ID is required")
	}

	// Get tunnel from database
	key := map[string]types.AttributeValue{
		"tunnel_id": &types.AttributeValueMemberS{Value: tunnelID},
	}

	var tunnel models.Tunnel
	err = dbClient.GetItem(ctx, tunnelsTable, key, &tunnel)
	if err != nil {
		return errorResponse(404, "Tunnel not found")
	}

	// Verify tunnel belongs to client
	if tunnel.ClientID != clientID {
		return errorResponse(403, "Unauthorized to delete this tunnel")
	}

	// Delete domain record
	domainKey := map[string]types.AttributeValue{
		"domain": &types.AttributeValueMemberS{Value: tunnel.Domain},
	}

	err = dbClient.DeleteItem(ctx, domainsTable, domainKey)
	if err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to delete domain: %v", err))
	}

	// Delete tunnel record
	err = dbClient.DeleteItem(ctx, tunnelsTable, key)
	if err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to delete tunnel: %v", err))
	}

	// Return success response
	response := DeleteTunnelResponse{
		Message: "Tunnel deleted successfully",
	}

	return successResponse(200, response)
}

func verifyClientAPIKey(ctx context.Context, apiKey string) (string, error) {
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

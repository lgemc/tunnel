package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/lmanrique/tunnel/lambdas/shared/auth"
	"github.com/lmanrique/tunnel/lambdas/shared/db"
	"github.com/lmanrique/tunnel/lambdas/shared/models"
)

var (
	clientsTable string
	dbClient     *db.DynamoDBClient
)

func init() {
	clientsTable = os.Getenv("CLIENTS_TABLE")
	if clientsTable == "" {
		panic("CLIENTS_TABLE environment variable is required")
	}
}

type RegisterClientResponse struct {
	ClientID string `json:"client_id"`
	APIKey   string `json:"api_key"`
	Message  string `json:"message"`
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

	// Generate client ID
	clientID, err := auth.GenerateClientID()
	if err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to generate client ID: %v", err))
	}

	// Generate API key
	apiKey, err := auth.GenerateAPIKey()
	if err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to generate API key: %v", err))
	}

	// Hash API key
	apiKeyHash, err := auth.HashAPIKey(apiKey)
	if err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to hash API key: %v", err))
	}

	// Create client record
	client := models.Client{
		ClientID:   clientID,
		APIKeyHash: apiKeyHash,
		Status:     models.ClientStatusActive,
		CreatedAt:  time.Now(),
	}

	// Save to DynamoDB
	err = dbClient.PutItem(ctx, clientsTable, client)
	if err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to save client: %v", err))
	}

	// Return response with API key (only time it's shown)
	response := RegisterClientResponse{
		ClientID: clientID,
		APIKey:   apiKey,
		Message:  "Client registered successfully. Please save your API key securely.",
	}

	return successResponse(201, response)
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

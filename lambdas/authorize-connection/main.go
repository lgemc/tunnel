package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
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

func handler(ctx context.Context, request events.APIGatewayCustomAuthorizerRequestTypeRequest) (events.APIGatewayCustomAuthorizerResponse, error) {
	// Initialize DB client if not already done
	if dbClient == nil {
		var err error
		dbClient, err = db.NewDynamoDBClient(ctx)
		if err != nil {
			return denyPolicy(request.MethodArn), fmt.Errorf("failed to initialize database: %w", err)
		}
	}

	// Extract API key from Authorization header
	authHeader := request.Headers["Authorization"]
	if authHeader == "" {
		authHeader = request.Headers["authorization"]
	}

	if authHeader == "" {
		return denyPolicy(request.MethodArn), fmt.Errorf("authorization header is missing")
	}

	apiKey, err := auth.ExtractBearerToken(authHeader)
	if err != nil {
		return denyPolicy(request.MethodArn), fmt.Errorf("invalid authorization header: %w", err)
	}

	// Verify client API key
	clientID, err := verifyClientAPIKey(ctx, apiKey)
	if err != nil {
		return denyPolicy(request.MethodArn), fmt.Errorf("invalid API key: %w", err)
	}

	// Return allow policy with client ID in context
	return allowPolicy(request.MethodArn, clientID), nil
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

func allowPolicy(methodArn, clientID string) events.APIGatewayCustomAuthorizerResponse {
	return events.APIGatewayCustomAuthorizerResponse{
		PrincipalID: clientID,
		PolicyDocument: events.APIGatewayCustomAuthorizerPolicy{
			Version: "2012-10-17",
			Statement: []events.IAMPolicyStatement{
				{
					Action:   []string{"execute-api:Invoke"},
					Effect:   "Allow",
					Resource: []string{methodArn},
				},
			},
		},
		Context: map[string]interface{}{
			"clientId": clientID,
		},
	}
}

func denyPolicy(methodArn string) events.APIGatewayCustomAuthorizerResponse {
	return events.APIGatewayCustomAuthorizerResponse{
		PrincipalID: "user",
		PolicyDocument: events.APIGatewayCustomAuthorizerPolicy{
			Version: "2012-10-17",
			Statement: []events.IAMPolicyStatement{
				{
					Action:   []string{"execute-api:Invoke"},
					Effect:   "Deny",
					Resource: []string{methodArn},
				},
			},
		},
	}
}

func main() {
	lambda.Start(handler)
}

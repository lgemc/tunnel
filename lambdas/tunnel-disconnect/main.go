package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/lmanrique/tunnel/lambdas/shared/db"
	"github.com/lmanrique/tunnel/lambdas/shared/models"
)

var (
	tunnelsTable string
	dbClient     *db.DynamoDBClient
)

func init() {
	tunnelsTable = os.Getenv("TUNNELS_TABLE")
	if tunnelsTable == "" {
		panic("TUNNELS_TABLE environment variable is required")
	}
}

func handler(ctx context.Context, request events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Initialize DB client if not already done
	if dbClient == nil {
		var err error
		dbClient, err = db.NewDynamoDBClient(ctx)
		if err != nil {
			return errorResponse(500, fmt.Sprintf("Failed to initialize database: %v", err))
		}
	}

	// Get connection ID
	connectionID := request.RequestContext.ConnectionID

	// Find tunnel by connection ID
	tunnelID, err := findTunnelByConnectionID(ctx, connectionID)
	if err != nil {
		// Connection might not be associated with a tunnel, which is okay
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Body:       `{"message": "Disconnected"}`,
		}, nil
	}

	// Update tunnel status to inactive and remove connection ID
	key := map[string]types.AttributeValue{
		"tunnel_id": &types.AttributeValueMemberS{Value: tunnelID},
	}

	updateInput := &dynamodb.UpdateItemInput{
		TableName: aws.String(tunnelsTable),
		Key:       key,
		UpdateExpression: aws.String("SET #status = :status, updated_at = :updated_at REMOVE connection_id"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status":     &types.AttributeValueMemberS{Value: models.TunnelStatusInactive},
			":updated_at": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		},
	}

	err = dbClient.UpdateItem(ctx, updateInput)
	if err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to update tunnel: %v", err))
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       `{"message": "Disconnected successfully"}`,
	}, nil
}

func findTunnelByConnectionID(ctx context.Context, connectionID string) (string, error) {
	// Scan tunnels table to find tunnel with matching connection ID
	// In production, consider using a GSI for better performance
	var tunnels []models.Tunnel
	err := dbClient.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(tunnelsTable),
		FilterExpression: aws.String("connection_id = :connection_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":connection_id": &types.AttributeValueMemberS{Value: connectionID},
		},
	}, &tunnels)

	if err != nil {
		return "", err
	}

	if len(tunnels) == 0 {
		return "", fmt.Errorf("tunnel not found for connection ID: %s", connectionID)
	}

	return tunnels[0].TunnelID, nil
}

func errorResponse(statusCode int, message string) (events.APIGatewayProxyResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"error": message,
	})

	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Body:       string(body),
	}, nil
}

func main() {
	lambda.Start(handler)
}

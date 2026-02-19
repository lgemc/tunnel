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

	// Get client ID from authorizer context
	var clientID string
	if authContext, ok := request.RequestContext.Authorizer.(map[string]interface{}); ok {
		if cid, exists := authContext["clientId"]; exists {
			clientID, _ = cid.(string)
		}
	}
	if clientID == "" {
		return errorResponse(401, "Client ID not found in context")
	}

	// Get tunnel ID from query parameters
	tunnelID := request.QueryStringParameters["tunnel_id"]
	if tunnelID == "" {
		return errorResponse(400, "Tunnel ID is required")
	}

	// Get connection ID
	connectionID := request.RequestContext.ConnectionID

	// Verify tunnel exists and belongs to client
	key := map[string]types.AttributeValue{
		"tunnel_id": &types.AttributeValueMemberS{Value: tunnelID},
	}

	var tunnel models.Tunnel
	err := dbClient.GetItem(ctx, tunnelsTable, key, &tunnel)
	if err != nil {
		return errorResponse(404, "Tunnel not found")
	}

	if tunnel.ClientID != clientID {
		return errorResponse(403, "Unauthorized to connect to this tunnel")
	}

	// Update tunnel with connection ID and set status to active
	updateInput := &dynamodb.UpdateItemInput{
		TableName: aws.String(tunnelsTable),
		Key:       key,
		UpdateExpression: aws.String("SET connection_id = :connection_id, #status = :status, updated_at = :updated_at"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":connection_id": &types.AttributeValueMemberS{Value: connectionID},
			":status":        &types.AttributeValueMemberS{Value: models.TunnelStatusActive},
			":updated_at":    &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		},
	}

	err = dbClient.UpdateItem(ctx, updateInput)
	if err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to update tunnel: %v", err))
	}

	// Return success response
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       `{"message": "Connected successfully"}`,
	}, nil
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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/lmanrique/tunnel/lambdas/shared/db"
	"github.com/lmanrique/tunnel/lambdas/shared/models"
)

var (
	tunnelsTable         string
	domainsTable         string
	pendingRequestsTable string
	dbClient             *db.DynamoDBClient
	apiGatewayClient     *apigatewaymanagementapi.Client
)

func init() {
	tunnelsTable = os.Getenv("TUNNELS_TABLE")
	domainsTable = os.Getenv("DOMAINS_TABLE")
	pendingRequestsTable = os.Getenv("PENDING_REQUESTS_TABLE")

	if tunnelsTable == "" || domainsTable == "" {
		panic("Required environment variables are missing")
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

	// Parse incoming message
	var message models.WebSocketMessage
	if err := json.Unmarshal([]byte(request.Body), &message); err != nil {
		return errorResponse(400, "Invalid message format")
	}

	// Handle different message types
	switch message.Action {
	case models.MessageTypePing:
		return handlePing(ctx, request.RequestContext.ConnectionID)
	case models.MessageTypeResponse:
		return handleResponse(ctx, message)
	case "proxy_response":
		return handleProxyResponse(ctx, message)
	default:
		return errorResponse(400, fmt.Sprintf("Unknown message action: %s", message.Action))
	}
}

func handlePing(ctx context.Context, connectionID string) (events.APIGatewayProxyResponse, error) {
	// Initialize API Gateway Management API client
	if apiGatewayClient == nil {
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return errorResponse(500, "Failed to load AWS config")
		}

		// Get the API Gateway endpoint from request context
		// In production, this should come from environment variable
		apiGatewayClient = apigatewaymanagementapi.NewFromConfig(cfg)
	}

	// Send PONG response
	pongMessage := models.WebSocketMessage{
		Action: models.MessageTypePong,
	}

	messageBytes, err := json.Marshal(pongMessage)
	if err != nil {
		return errorResponse(500, "Failed to marshal PONG message")
	}

	_, err = apiGatewayClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connectionID),
		Data:         messageBytes,
	})

	if err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to send PONG: %v", err))
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       `{"message": "PONG sent"}`,
	}, nil
}

func handleResponse(ctx context.Context, message models.WebSocketMessage) (events.APIGatewayProxyResponse, error) {
	// This would handle HTTP responses from the client
	// In a full implementation, this would:
	// 1. Look up the pending request by request_id
	// 2. Parse the HTTP response from message.Data
	// 3. Return the response to the original HTTP requester
	// 4. Clean up the pending request

	// For now, just acknowledge receipt
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       `{"message": "Response received"}`,
	}, nil
}

func handleProxyResponse(ctx context.Context, message models.WebSocketMessage) (events.APIGatewayProxyResponse, error) {
	if pendingRequestsTable == "" {
		log.Printf("proxy_response: PENDING_REQUESTS_TABLE not configured")
		return errorResponse(500, "PENDING_REQUESTS_TABLE not configured")
	}

	// Extract response data
	requestID, _ := message.Data["request_id"].(string)
	if requestID == "" {
		log.Printf("proxy_response: missing request_id, data keys: %v", message.Data)
		return errorResponse(400, "Request ID is required")
	}

	log.Printf("proxy_response: received for request_id=%s", requestID)

	statusCode := 200
	if sc, ok := message.Data["status_code"].(float64); ok {
		statusCode = int(sc)
	}

	responseHeaders := make(map[string]string)
	if headers, ok := message.Data["response_headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			if strVal, ok := v.(string); ok {
				responseHeaders[k] = strVal
			}
		}
	}

	responseBody, _ := message.Data["response_body"].(string)

	// Build DynamoDB map for response headers
	headersAV := map[string]types.AttributeValue{}
	for k, v := range responseHeaders {
		headersAV[k] = &types.AttributeValueMemberS{Value: v}
	}

	// Use UpdateItem to atomically set only the response fields (no GetItem needed)
	err := dbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(pendingRequestsTable),
		Key: map[string]types.AttributeValue{
			"request_id": &types.AttributeValueMemberS{Value: requestID},
		},
		UpdateExpression: aws.String("SET #s = :status, response_status = :code, response_headers = :headers, response_body = :body"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status":  &types.AttributeValueMemberS{Value: "completed"},
			":code":    &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", statusCode)},
			":headers": &types.AttributeValueMemberM{Value: headersAV},
			":body":    &types.AttributeValueMemberS{Value: responseBody},
		},
	})
	if err != nil {
		log.Printf("proxy_response: failed to update request_id=%s: %v", requestID, err)
		return errorResponse(500, fmt.Sprintf("Failed to update pending request: %v", err))
	}

	log.Printf("proxy_response: successfully marked request_id=%s as completed (status=%d)", requestID, statusCode)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       `{"message": "Proxy response processed"}`,
	}, nil
}

// handleHTTPRequest would be called when an external HTTP request comes in
// This would typically be triggered by CloudFront or a separate Lambda
func handleHTTPRequest(ctx context.Context, domain string, httpReq models.HTTPRequest) error {
	// 1. Look up tunnel by domain
	domainKey := map[string]types.AttributeValue{
		"domain": &types.AttributeValueMemberS{Value: domain},
	}

	var domainRecord models.Domain
	err := dbClient.GetItem(ctx, domainsTable, domainKey, &domainRecord)
	if err != nil {
		return fmt.Errorf("domain not found: %w", err)
	}

	// 2. Get tunnel details
	tunnelKey := map[string]types.AttributeValue{
		"tunnel_id": &types.AttributeValueMemberS{Value: domainRecord.TunnelID},
	}

	var tunnel models.Tunnel
	err = dbClient.GetItem(ctx, tunnelsTable, tunnelKey, &tunnel)
	if err != nil {
		return fmt.Errorf("tunnel not found: %w", err)
	}

	if tunnel.Status != models.TunnelStatusActive || tunnel.ConnectionID == "" {
		return fmt.Errorf("tunnel is not active")
	}

	// 3. Send request to client via WebSocket
	requestMessage := models.WebSocketMessage{
		Action:    models.MessageTypeRequest,
		RequestID: generateRequestID(),
		Data: map[string]interface{}{
			"method":  httpReq.Method,
			"path":    httpReq.Path,
			"headers": httpReq.Headers,
			"body":    httpReq.Body,
		},
	}

	messageBytes, err := json.Marshal(requestMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	_, err = apiGatewayClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(tunnel.ConnectionID),
		Data:         messageBytes,
	})

	if err != nil {
		return fmt.Errorf("failed to send request to client: %w", err)
	}

	// 4. In production, store pending request and wait for response
	// This would typically use DynamoDB or Redis to track pending requests

	return nil
}

func generateRequestID() string {
	// In production, use a proper UUID generator
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
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

package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/lmanrique/tunnel/lambdas/shared/db"
	"github.com/lmanrique/tunnel/lambdas/shared/models"
)

var (
	domainsTable         string
	tunnelsTable         string
	pendingRequestsTable string
	websocketEndpoint    string
	domainName           string
	dbClient             *db.DynamoDBClient
)

func init() {
	domainsTable = os.Getenv("DOMAINS_TABLE")
	tunnelsTable = os.Getenv("TUNNELS_TABLE")
	pendingRequestsTable = os.Getenv("PENDING_REQUESTS_TABLE")
	websocketEndpoint = os.Getenv("WEBSOCKET_ENDPOINT")
	domainName = os.Getenv("DOMAIN_NAME")

	if domainsTable == "" || tunnelsTable == "" || pendingRequestsTable == "" || websocketEndpoint == "" || domainName == "" {
		panic("Required environment variables are missing")
	}
}

type ProxyRequest struct {
	RequestID string            `json:"request_id"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Headers   map[string]string `json:"headers"`
	Body      string            `json:"body"`
}

type PendingRequest struct {
	RequestID       string            `dynamodbav:"request_id" json:"request_id"`
	TunnelID        string            `dynamodbav:"tunnel_id" json:"tunnel_id"`
	Method          string            `dynamodbav:"method" json:"method"`
	Path            string            `dynamodbav:"path" json:"path"`
	Headers         map[string]string `dynamodbav:"headers" json:"headers"`
	Body            string            `dynamodbav:"body" json:"body"`
	Status          string            `dynamodbav:"status" json:"status"` // "pending" or "completed"
	ResponseStatus  int               `dynamodbav:"response_status,omitempty" json:"response_status,omitempty"`
	ResponseHeaders map[string]string `dynamodbav:"response_headers,omitempty" json:"response_headers,omitempty"`
	ResponseBody    string            `dynamodbav:"response_body,omitempty" json:"response_body,omitempty"`
	CreatedAt       time.Time         `dynamodbav:"created_at" json:"created_at"`
	TTL             int64             `dynamodbav:"ttl" json:"ttl"` // Unix timestamp for auto-deletion
}

func generateRequestID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
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

	// Extract subdomain from path parameters
	subdomain := request.PathParameters["subdomain"]
	if subdomain == "" {
		return errorResponse(400, "Subdomain is required")
	}

	// Get proxy path
	proxyPath := request.PathParameters["proxy"]
	if proxyPath == "" {
		proxyPath = "/"
	} else {
		proxyPath = "/" + proxyPath
	}
	if request.RawQueryString != "" {
		proxyPath = proxyPath + "?" + request.RawQueryString
	}

	// Decode body if API Gateway base64-encoded it
	body := request.Body
	if request.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(body)
		if err != nil {
			return errorResponse(400, "Failed to decode request body")
		}
		body = string(decoded)
	}

	// Look up domain to get tunnel ID
	fullDomain := fmt.Sprintf("%s.%s", subdomain, domainName)
	key := map[string]types.AttributeValue{
		"domain": &types.AttributeValueMemberS{Value: fullDomain},
	}

	var domain models.Domain
	err := dbClient.GetItem(ctx, domainsTable, key, &domain)
	if err != nil {
		return errorResponse(404, "Tunnel not found")
	}

	// Get tunnel details
	tunnelKey := map[string]types.AttributeValue{
		"tunnel_id": &types.AttributeValueMemberS{Value: domain.TunnelID},
	}

	var tunnel models.Tunnel
	err = dbClient.GetItem(ctx, tunnelsTable, tunnelKey, &tunnel)
	if err != nil {
		return errorResponse(404, "Tunnel not found")
	}

	// Check if tunnel is active
	if tunnel.Status != models.TunnelStatusActive {
		return errorResponse(503, "Tunnel is not active")
	}

	// Check if tunnel has a connection
	if tunnel.ConnectionID == "" {
		return errorResponse(503, "Tunnel is not connected")
	}

	// Generate unique request ID
	requestID, err := generateRequestID()
	if err != nil {
		return errorResponse(500, "Failed to generate request ID")
	}

	// Store request in DynamoDB
	pendingReq := PendingRequest{
		RequestID: requestID,
		TunnelID:  domain.TunnelID,
		Method:    request.RequestContext.HTTP.Method,
		Path:      proxyPath,
		Headers:   request.Headers,
		Body:      body,
		Status:    "pending",
		CreatedAt: time.Now(),
		TTL:       time.Now().Add(5 * time.Minute).Unix(), // Auto-delete after 5 minutes
	}

	if err := dbClient.PutItem(ctx, pendingRequestsTable, pendingReq); err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to store request: %v", err))
	}

	// Send request to WebSocket connection
	proxyReq := ProxyRequest{
		RequestID: requestID,
		Method:    request.RequestContext.HTTP.Method,
		Path:      proxyPath,
		Headers:   request.Headers,
		Body:      body,
	}

	payloadBytes, err := json.Marshal(map[string]interface{}{
		"action": "proxy",
		"data":   proxyReq,
	})
	if err != nil {
		return errorResponse(500, "Failed to marshal request")
	}

	// Send request through WebSocket
	cfg, err := dbClient.GetAWSConfig(ctx)
	if err != nil {
		return errorResponse(500, "Failed to get AWS config")
	}

	apigwClient := apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = aws.String(websocketEndpoint)
	})

	_, err = apigwClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(tunnel.ConnectionID),
		Data:         payloadBytes,
	})
	if err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to send request to tunnel: %v", err))
	}

	// Poll for response (timeout after 3 minutes)
	timeout := time.After(180 * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return errorResponse(504, "Gateway timeout - no response from tunnel")
		case <-ticker.C:
			// Check if response is available
			reqKey := map[string]types.AttributeValue{
				"request_id": &types.AttributeValueMemberS{Value: requestID},
			}

			var updatedReq PendingRequest
			err := dbClient.GetItem(ctx, pendingRequestsTable, reqKey, &updatedReq)
			if err != nil {
				continue // Keep polling
			}

			if updatedReq.Status == "completed" {
				// Return the response
				statusCode := updatedReq.ResponseStatus
				if statusCode == 0 {
					statusCode = 200
				}

				return events.APIGatewayV2HTTPResponse{
					StatusCode: statusCode,
					Headers:    updatedReq.ResponseHeaders,
					Body:       updatedReq.ResponseBody,
				}, nil
			}
		}
	}
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

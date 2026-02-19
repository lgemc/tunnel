package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
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

func handler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (*events.LambdaFunctionURLStreamingResponse, error) {
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

	// Send request through WebSocket
	cfg, err := dbClient.GetAWSConfig(ctx)
	if err != nil {
		return errorResponse(500, "Failed to get AWS config")
	}

	apigwClient := apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = aws.String(websocketEndpoint)
	})

	const wsChunkSize = 90 * 1024 // 90KB — stays under API Gateway's 128KB WebSocket message limit

	// If body is large, send it in chunks before the main proxy message
	totalChunks := 0
	proxyBody := body
	if len(body) > wsChunkSize {
		totalChunks = (len(body) + wsChunkSize - 1) / wsChunkSize
		for i := 0; i < totalChunks; i++ {
			start := i * wsChunkSize
			end := start + wsChunkSize
			if end > len(body) {
				end = len(body)
			}
			chunkPayload, err := json.Marshal(map[string]interface{}{
				"action": "proxy_chunk",
				"data": map[string]interface{}{
					"request_id":  requestID,
					"chunk_index": i,
					"data":        body[start:end],
				},
			})
			if err != nil {
				return errorResponse(500, "Failed to marshal request chunk")
			}
			_, err = apigwClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
				ConnectionId: aws.String(tunnel.ConnectionID),
				Data:         chunkPayload,
			})
			if err != nil {
				return errorResponse(500, fmt.Sprintf("Failed to send request chunk to tunnel: %v", err))
			}
		}
		proxyBody = "" // body will be assembled by CLI from chunks
	}

	// Send request to WebSocket connection
	proxyReq := map[string]interface{}{
		"request_id":   requestID,
		"method":       request.RequestContext.HTTP.Method,
		"path":         proxyPath,
		"headers":      request.Headers,
		"body":         proxyBody,
		"total_chunks": totalChunks,
	}

	payloadBytes, err := json.Marshal(map[string]interface{}{
		"action": "proxy",
		"data":   proxyReq,
	})
	if err != nil {
		return errorResponse(500, "Failed to marshal request")
	}

	_, err = apigwClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(tunnel.ConnectionID),
		Data:         payloadBytes,
	})
	if err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to send request to tunnel: %v", err))
	}

	// Poll for response — detect streaming or buffered completion (50ms interval, 3min timeout)
	pollTimeout := time.After(180 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	reqKey := map[string]types.AttributeValue{
		"request_id": &types.AttributeValueMemberS{Value: requestID},
	}

	for {
		select {
		case <-pollTimeout:
			return errorResponse(504, "Gateway timeout - no response from tunnel")
		case <-ticker.C:
			rawItem, err := dbClient.GetRawItem(ctx, pendingRequestsTable, reqKey)
			if err != nil {
				continue
			}

			// SSE / streaming response detected
			if isStreamingAV, ok := rawItem["is_streaming"]; ok {
				if bv, ok := isStreamingAV.(*types.AttributeValueMemberBOOL); ok && bv.Value {
					return buildStreamingResponse(ctx, requestID, rawItem)
				}
			}

			// Buffered (non-streaming) response completed
			if statusAV, ok := rawItem["status"]; ok {
				if sv, ok := statusAV.(*types.AttributeValueMemberS); ok && sv.Value == "completed" {
					return buildBufferedResponse(rawItem)
				}
			}
		}
	}
}

// buildStreamingResponse creates a pipe-backed streaming response that forwards
// SSE chunks from DynamoDB to the HTTP caller as they arrive.
func buildStreamingResponse(ctx context.Context, requestID string, firstItem map[string]types.AttributeValue) (*events.LambdaFunctionURLStreamingResponse, error) {
	statusCode := 200
	if sc, ok := firstItem["stream_status"]; ok {
		if nv, ok := sc.(*types.AttributeValueMemberN); ok {
			statusCode, _ = strconv.Atoi(nv.Value)
		}
	}

	headers := map[string]string{}
	if h, ok := firstItem["stream_headers"]; ok {
		if mv, ok := h.(*types.AttributeValueMemberM); ok {
			for k, v := range mv.Value {
				if sv, ok := v.(*types.AttributeValueMemberS); ok {
					headers[k] = sv.Value
				}
			}
		}
	}

	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()

		streamTimeout := time.After(180 * time.Second)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		nextChunk := 0
		reqKey := map[string]types.AttributeValue{
			"request_id": &types.AttributeValueMemberS{Value: requestID},
		}

		for {
			select {
			case <-streamTimeout:
				return
			case <-ticker.C:
				rawItem, err := dbClient.GetRawItem(ctx, pendingRequestsTable, reqKey)
				if err != nil {
					continue
				}

				// Forward all newly available chunks
				for {
					attrName := fmt.Sprintf("stream_chunk_%d", nextChunk)
					av, ok := rawItem[attrName]
					if !ok {
						break
					}
					if sv, ok := av.(*types.AttributeValueMemberS); ok {
						if _, err := pw.Write([]byte(sv.Value)); err != nil {
							return
						}
						nextChunk++
					} else {
						break
					}
				}

				// Stop when CLI signals end of stream
				if doneAV, ok := rawItem["stream_done"]; ok {
					if bv, ok := doneAV.(*types.AttributeValueMemberBOOL); ok && bv.Value {
						return
					}
				}
			}
		}
	}()

	return &events.LambdaFunctionURLStreamingResponse{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       pr,
	}, nil
}

// buildBufferedResponse returns the full body at once for non-streaming responses.
func buildBufferedResponse(rawItem map[string]types.AttributeValue) (*events.LambdaFunctionURLStreamingResponse, error) {
	statusCode := 200
	if sc, ok := rawItem["response_status"]; ok {
		if nv, ok := sc.(*types.AttributeValueMemberN); ok {
			statusCode, _ = strconv.Atoi(nv.Value)
		}
	}

	headers := map[string]string{}
	if h, ok := rawItem["response_headers"]; ok {
		if mv, ok := h.(*types.AttributeValueMemberM); ok {
			for k, v := range mv.Value {
				if sv, ok := v.(*types.AttributeValueMemberS); ok {
					headers[k] = sv.Value
				}
			}
		}
	}

	responseBody := ""
	if bodyAV, ok := rawItem["response_body"]; ok {
		if sv, ok := bodyAV.(*types.AttributeValueMemberS); ok {
			responseBody = sv.Value
		}
	}

	return &events.LambdaFunctionURLStreamingResponse{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       bytes.NewReader([]byte(responseBody)),
	}, nil
}

func errorResponse(statusCode int, message string) (*events.LambdaFunctionURLStreamingResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"error": message,
	})

	return &events.LambdaFunctionURLStreamingResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: bytes.NewReader(body),
	}, nil
}

func main() {
	lambda.Start(handler)
}

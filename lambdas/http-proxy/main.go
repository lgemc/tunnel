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
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/lmanrique/tunnel/lambdas/shared/db"
	"github.com/lmanrique/tunnel/lambdas/shared/models"
)

var (
	domainsTable         string
	tunnelsTable         string
	pendingRequestsTable string
	websocketEndpoint    string
	domainName           string
	uploadsBucket        string
	reconnectGracePeriod time.Duration
	dbClient             *db.DynamoDBClient
	s3Client             *s3.Client
	s3PresignClient      *s3.PresignClient
)

func init() {
	domainsTable = os.Getenv("DOMAINS_TABLE")
	tunnelsTable = os.Getenv("TUNNELS_TABLE")
	pendingRequestsTable = os.Getenv("PENDING_REQUESTS_TABLE")
	websocketEndpoint = os.Getenv("WEBSOCKET_ENDPOINT")
	domainName = os.Getenv("DOMAIN_NAME")
	uploadsBucket = os.Getenv("UPLOADS_BUCKET")

	if domainsTable == "" || tunnelsTable == "" || pendingRequestsTable == "" || websocketEndpoint == "" || domainName == "" {
		panic("Required environment variables are missing")
	}

	// Parse reconnect grace period (default: 30s)
	gracePeriodStr := os.Getenv("TUNNEL_RECONNECT_GRACE_PERIOD")
	if gracePeriodStr == "" {
		reconnectGracePeriod = 30 * time.Second
	} else {
		parsed, err := time.ParseDuration(gracePeriodStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid TUNNEL_RECONNECT_GRACE_PERIOD: %v, using default 30s\n", err)
			reconnectGracePeriod = 30 * time.Second
		} else {
			reconnectGracePeriod = parsed
		}
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
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// waitForTunnelReconnect waits for an inactive tunnel to become active again.
// Returns the updated tunnel if it becomes active, or an error if the grace period expires.
// Only waits if the tunnel was recently active (updated within last 5 minutes).
func waitForTunnelReconnect(ctx context.Context, tunnelID string, tunnel *models.Tunnel) (*models.Tunnel, error) {
	// Only apply grace period if tunnel was recently active (within 5 minutes)
	if time.Since(tunnel.UpdatedAt) > 5*time.Minute {
		return nil, fmt.Errorf("tunnel has been inactive for too long")
	}

	fmt.Printf("Tunnel %s is inactive but was recently connected, waiting up to %v for reconnect...\n", tunnelID, reconnectGracePeriod)

	deadline := time.Now().Add(reconnectGracePeriod)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	tunnelKey := map[string]types.AttributeValue{
		"tunnel_id": &types.AttributeValueMemberS{Value: tunnelID},
	}

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("request cancelled while waiting for tunnel reconnect")
		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("tunnel did not reconnect within grace period")
			}

			var updatedTunnel models.Tunnel
			if err := dbClient.GetItem(ctx, tunnelsTable, tunnelKey, &updatedTunnel); err != nil {
				continue
			}

			if updatedTunnel.Status == models.TunnelStatusActive && updatedTunnel.ConnectionID != "" {
				fmt.Printf("Tunnel %s reconnected successfully!\n", tunnelID)
				return &updatedTunnel, nil
			}
		}
	}
}

func initClients(ctx context.Context) error {
	if dbClient == nil {
		var err error
		dbClient, err = db.NewDynamoDBClient(ctx)
		if err != nil {
			return fmt.Errorf("failed to initialize DynamoDB client: %w", err)
		}
	}
	if s3Client == nil && uploadsBucket != "" {
		cfg, err := dbClient.GetAWSConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to get AWS config: %w", err)
		}
		s3Client = s3.NewFromConfig(cfg)
		s3PresignClient = s3.NewPresignClient(s3Client)
	}
	return nil
}

func handler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (*events.LambdaFunctionURLStreamingResponse, error) {
	if err := initClients(ctx); err != nil {
		return errorResponse(500, err.Error())
	}

	// DEBUG: log incoming request details
	fmt.Printf("DEBUG path=%q rawPath=%q host=%q x-tunnel-subdomain=%q method=%q\n",
		request.RawPath, request.RawPath,
		request.Headers["host"],
		request.Headers["x-tunnel-subdomain"],
		request.RequestContext.HTTP.Method,
	)

	path := request.RawPath

	// ── Poll endpoint: GET /poll/{request_id} ────────────────────────────────
	if strings.HasPrefix(path, "/poll/") {
		requestID := strings.TrimPrefix(path, "/poll/")
		if requestID == "" {
			return errorResponse(400, "request_id is required")
		}
		return handlePollResponse(ctx, requestID)
	}

	// ── Upload-URL endpoint: POST /upload-url/{subdomain}[/{proxy+}] ─────────
	if strings.HasPrefix(path, "/upload-url/") {
		return handleUploadURL(ctx, request)
	}

	// ── Normal proxy: /t/{subdomain}[/{proxy+}] ──────────────────────────────
	return handleProxy(ctx, request)
}

// handleProxy is the main tunnel proxy path (unchanged behaviour for normal requests).
func handleProxy(ctx context.Context, request events.APIGatewayV2HTTPRequest) (*events.LambdaFunctionURLStreamingResponse, error) {
	// Extract subdomain — from path parameters (API Gateway) or raw path (Lambda Function URL)
	subdomain := request.PathParameters["subdomain"]
	proxyPath := ""
	if subdomain == "" {
		trimmed := strings.TrimPrefix(request.RawPath, "/t/")
		if trimmed == request.RawPath || trimmed == "" {
			return errorResponse(400, "Subdomain is required")
		}
		slashIdx := strings.Index(trimmed, "/")
		if slashIdx == -1 {
			subdomain = trimmed
			proxyPath = "/"
		} else {
			subdomain = trimmed[:slashIdx]
			proxyPath = trimmed[slashIdx:]
		}
	} else {
		pp := request.PathParameters["proxy"]
		if pp == "" {
			proxyPath = "/"
		} else {
			proxyPath = "/" + pp
		}
	}
	if subdomain == "" {
		return errorResponse(400, "Subdomain is required")
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

	// Look up domain → tunnel
	fullDomain := fmt.Sprintf("%s.%s", subdomain, domainName)
	key := map[string]types.AttributeValue{
		"domain": &types.AttributeValueMemberS{Value: fullDomain},
	}

	var domain models.Domain
	if err := dbClient.GetItem(ctx, domainsTable, key, &domain); err != nil {
		return errorResponse(404, "Tunnel not found")
	}

	tunnelKey := map[string]types.AttributeValue{
		"tunnel_id": &types.AttributeValueMemberS{Value: domain.TunnelID},
	}
	var tunnel models.Tunnel
	if err := dbClient.GetItem(ctx, tunnelsTable, tunnelKey, &tunnel); err != nil {
		return errorResponse(404, "Tunnel not found")
	}

	// If tunnel is inactive, wait for reconnection (grace period)
	if tunnel.Status != models.TunnelStatusActive || tunnel.ConnectionID == "" {
		reconnectedTunnel, waitErr := waitForTunnelReconnect(ctx, domain.TunnelID, &tunnel)
		if waitErr != nil {
			// Grace period expired without reconnection
			if tunnel.Status != models.TunnelStatusActive {
				return errorResponse(503, "Tunnel is not active")
			}
			return errorResponse(503, "Tunnel is not connected")
		}
		// Use the reconnected tunnel
		tunnel = *reconnectedTunnel
	}

	requestID, err := generateRequestID()
	if err != nil {
		return errorResponse(500, "Failed to generate request ID")
	}

	// Pre-generate a presigned S3 PUT URL so the CLI can stage large/binary responses.
	s3PutURL, s3ResponseKey := "", ""
	if uploadsBucket != "" {
		s3ResponseKey = fmt.Sprintf("responses/%s/body", requestID)
		presignReq, presignErr := s3PresignClient.PresignPutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(uploadsBucket),
			Key:    aws.String(s3ResponseKey),
		}, s3.WithPresignExpires(30*time.Minute))
		if presignErr == nil {
			s3PutURL = presignReq.URL
		}
	}

	// Store pending request in DynamoDB
	pendingReq := PendingRequest{
		RequestID: requestID,
		TunnelID:  domain.TunnelID,
		Method:    request.RequestContext.HTTP.Method,
		Path:      proxyPath,
		Headers:   request.Headers,
		Body:      body,
		Status:    "pending",
		CreatedAt: time.Now(),
		TTL:       time.Now().Add(5 * time.Minute).Unix(),
	}
	if err := dbClient.PutItem(ctx, pendingRequestsTable, pendingReq); err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to store request: %v", err))
	}

	// Build API Gateway management client
	cfg, err := dbClient.GetAWSConfig(ctx)
	if err != nil {
		return errorResponse(500, "Failed to get AWS config")
	}
	apigwClient := apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = aws.String(websocketEndpoint)
	})

	const wsChunkSize = 90 * 1024

	// If request body is large, send it to the CLI in chunks before the main message
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
			if _, err = apigwClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
				ConnectionId: aws.String(tunnel.ConnectionID),
				Data:         chunkPayload,
			}); err != nil {
				return errorResponse(500, fmt.Sprintf("Failed to send request chunk to tunnel: %v", err))
			}
		}
		proxyBody = ""
	}

	// Send main proxy message (includes presigned S3 URL for large responses)
	proxyReq := map[string]interface{}{
		"request_id":      requestID,
		"method":          request.RequestContext.HTTP.Method,
		"path":            proxyPath,
		"headers":         request.Headers,
		"body":            proxyBody,
		"total_chunks":    totalChunks,
		"s3_put_url":      s3PutURL,
		"s3_response_key": s3ResponseKey,
	}

	payloadBytes, err := json.Marshal(map[string]interface{}{
		"action": "proxy",
		"data":   proxyReq,
	})
	if err != nil {
		return errorResponse(500, "Failed to marshal request")
	}
	if _, err = apigwClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(tunnel.ConnectionID),
		Data:         payloadBytes,
	}); err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to send request to tunnel: %v", err))
	}

	return pollAndReturn(ctx, requestID)
}

// handleUploadURL generates a presigned S3 PUT URL for a large request body upload.
// The client calls POST /upload-url/{subdomain}/{proxy+} with JSON metadata in the body,
// uploads the actual file to the returned presigned URL, then polls GET /poll/{request_id}.
func handleUploadURL(ctx context.Context, request events.APIGatewayV2HTTPRequest) (*events.LambdaFunctionURLStreamingResponse, error) {
	if uploadsBucket == "" {
		return errorResponse(503, "Large upload support not configured (UPLOADS_BUCKET missing)")
	}

	// Extract subdomain from path (/upload-url/{subdomain}/...) when present,
	// or from the Host header when coming through CloudFront (*.tunnel.atelier.run).
	// Dart client calls: POST myapp.tunnel.atelier.run/upload-url/transcribe
	// → path = "/upload-url/transcribe", host = "myapp.tunnel.atelier.run"
	trimmed := strings.TrimPrefix(request.RawPath, "/upload-url/")
	subdomain := ""
	proxyPath := "/"

	if trimmed != "" {
		// CloudFront injects x-tunnel-subdomain header (original Host is stripped by CloudFront).
		// Fall back to parsing it from the path for direct Lambda URL calls.
		cfSubdomain := request.Headers["x-tunnel-subdomain"]
		if cfSubdomain != "" {
			subdomain = cfSubdomain
			proxyPath = "/" + trimmed
		} else {
			// Direct path: /upload-url/{subdomain}/{proxy+}
			if idx := strings.Index(trimmed, "/"); idx != -1 {
				subdomain = trimmed[:idx]
				proxyPath = trimmed[idx:]
			} else {
				subdomain = trimmed
			}
		}
	}

	if subdomain == "" {
		return errorResponse(400, "Subdomain is required")
	}

	// Parse optional metadata from body (method, content-type, headers)
	var meta struct {
		Method      string            `json:"method"`
		ContentType string            `json:"content_type"`
		Headers     map[string]string `json:"headers"`
	}
	meta.Method = "POST"
	if request.Body != "" {
		_ = json.Unmarshal([]byte(request.Body), &meta)
	}
	if meta.Method == "" {
		meta.Method = "POST"
	}
	if request.RawQueryString != "" {
		proxyPath = proxyPath + "?" + request.RawQueryString
	}

	// Look up domain → tunnel (must be active before issuing URL)
	fullDomain := fmt.Sprintf("%s.%s", subdomain, domainName)
	var domain models.Domain
	if err := dbClient.GetItem(ctx, domainsTable, map[string]types.AttributeValue{
		"domain": &types.AttributeValueMemberS{Value: fullDomain},
	}, &domain); err != nil {
		return errorResponse(404, "Tunnel not found")
	}
	var tunnel models.Tunnel
	if err := dbClient.GetItem(ctx, tunnelsTable, map[string]types.AttributeValue{
		"tunnel_id": &types.AttributeValueMemberS{Value: domain.TunnelID},
	}, &tunnel); err != nil {
		return errorResponse(404, "Tunnel not found")
	}
	if tunnel.Status != models.TunnelStatusActive {
		return errorResponse(503, "Tunnel is not active")
	}

	requestID, err := generateRequestID()
	if err != nil {
		return errorResponse(500, "Failed to generate request ID")
	}

	// S3 key encodes the request_id so the s3-upload-notify Lambda can look it up
	s3RequestKey := fmt.Sprintf("requests/%s/body", requestID)

	// Also pre-generate a presigned PUT URL for the CLI's response (same as handleProxy)
	s3ResponseKey := fmt.Sprintf("responses/%s/body", requestID)
	s3ResponsePutURL := ""
	responsePutReq, err := s3PresignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(uploadsBucket),
		Key:    aws.String(s3ResponseKey),
	}, s3.WithPresignExpires(30*time.Minute))
	if err == nil {
		s3ResponsePutURL = responsePutReq.URL
	}

	// Build the presigned PUT URL for the request body (what the caller uses to upload)
	// No Tagging — it would be included as a signed header the client must send.
	// The request_id is already encoded in the S3 key path.
	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(uploadsBucket),
		Key:         aws.String(s3RequestKey),
		ContentType: aws.String("application/octet-stream"),
	}
	presignReq, err := s3PresignClient.PresignPutObject(ctx, putInput,
		s3.WithPresignExpires(30*time.Minute),
	)
	if err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to generate presigned URL: %v", err))
	}

	// Create pending request (status: waiting_upload)
	pendingReq := PendingRequest{
		RequestID: requestID,
		TunnelID:  domain.TunnelID,
		Method:    meta.Method,
		Path:      proxyPath,
		Headers:   meta.Headers,
		Body:      "", // body will arrive via S3
		Status:    "waiting_upload",
		CreatedAt: time.Now(),
		TTL:       time.Now().Add(30 * time.Minute).Unix(),
	}
	if meta.Headers == nil {
		pendingReq.Headers = map[string]string{}
	}
	if err := dbClient.PutItem(ctx, pendingRequestsTable, pendingReq); err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to store pending request: %v", err))
	}

	// Also store the s3_response_key and s3_response_put_url so the notify Lambda
	// can include them in the WebSocket message to the CLI
	_ = dbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(pendingRequestsTable),
		Key: map[string]types.AttributeValue{
			"request_id": &types.AttributeValueMemberS{Value: requestID},
		},
		UpdateExpression: aws.String("SET s3_request_key = :rk, s3_response_key = :respk, s3_response_put_url = :respurl"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":rk":      &types.AttributeValueMemberS{Value: s3RequestKey},
			":respk":   &types.AttributeValueMemberS{Value: s3ResponseKey},
			":respurl": &types.AttributeValueMemberS{Value: s3ResponsePutURL},
		},
	})

	resp := map[string]string{
		"request_id": requestID,
		"upload_url": presignReq.URL,
		"poll_url":   fmt.Sprintf("/poll/%s", requestID),
	}
	body, _ := json.Marshal(resp)
	return &events.LambdaFunctionURLStreamingResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       bytes.NewReader(body),
	}, nil
}

// handlePollResponse polls DynamoDB for the response to a previously initiated upload request.
func handlePollResponse(ctx context.Context, requestID string) (*events.LambdaFunctionURLStreamingResponse, error) {
	reqKey := map[string]types.AttributeValue{
		"request_id": &types.AttributeValueMemberS{Value: requestID},
	}

	// Check it exists
	rawItem, err := dbClient.GetRawItem(ctx, pendingRequestsTable, reqKey)
	if err != nil || rawItem == nil {
		return errorResponse(404, "Request not found")
	}

	statusAV, ok := rawItem["status"]
	if !ok {
		return errorResponse(404, "Request not found")
	}
	sv, _ := statusAV.(*types.AttributeValueMemberS)
	if sv == nil {
		return errorResponse(404, "Request not found")
	}

	switch sv.Value {
	case "pending", "waiting_upload":
		body, _ := json.Marshal(map[string]string{"status": sv.Value})
		return &events.LambdaFunctionURLStreamingResponse{
			StatusCode: 202,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       bytes.NewReader(body),
		}, nil
	case "completed":
		return buildBufferedResponseFromItem(ctx, rawItem)
	default:
		body, _ := json.Marshal(map[string]string{"status": sv.Value})
		return &events.LambdaFunctionURLStreamingResponse{
			StatusCode: 202,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       bytes.NewReader(body),
		}, nil
	}
}

// pollAndReturn waits for the CLI to complete the request and builds the appropriate response.
func pollAndReturn(ctx context.Context, requestID string) (*events.LambdaFunctionURLStreamingResponse, error) {
	pollTimeout := time.After(180 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	reqKey := map[string]types.AttributeValue{
		"request_id": &types.AttributeValueMemberS{Value: requestID},
	}

	for {
		select {
		case <-ctx.Done():
			return errorResponse(499, "Client disconnected")
		case <-pollTimeout:
			return errorResponse(504, "Gateway timeout - no response from tunnel")
		case <-ticker.C:
			rawItem, err := dbClient.GetRawItem(ctx, pendingRequestsTable, reqKey)
			if err != nil {
				continue
			}

			// SSE / streaming response
			if isStreamingAV, ok := rawItem["is_streaming"]; ok {
				if bv, ok := isStreamingAV.(*types.AttributeValueMemberBOOL); ok && bv.Value {
					return buildStreamingResponse(ctx, requestID, rawItem)
				}
			}

			// S3-staged response (large/binary body)
			if s3KeyAV, ok := rawItem["s3_response_key"]; ok {
				if sv, ok := s3KeyAV.(*types.AttributeValueMemberS); ok && sv.Value != "" {
					// Only act once the CLI has confirmed it uploaded to S3
					if doneAV, ok2 := rawItem["s3_response_ready"]; ok2 {
						if bv, ok3 := doneAV.(*types.AttributeValueMemberBOOL); ok3 && bv.Value {
							return buildS3StreamingResponse(ctx, rawItem, sv.Value)
						}
					}
				}
			}

			// Buffered response completed
			if statusAV, ok := rawItem["status"]; ok {
				if sv, ok := statusAV.(*types.AttributeValueMemberS); ok && sv.Value == "completed" {
					return buildBufferedResponseFromItem(ctx, rawItem)
				}
			}
		}
	}
}

// buildS3StreamingResponse fetches the response body from S3 and pipes it to the caller.
func buildS3StreamingResponse(ctx context.Context, rawItem map[string]types.AttributeValue, s3Key string) (*events.LambdaFunctionURLStreamingResponse, error) {
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

	result, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(uploadsBucket),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		return errorResponse(502, fmt.Sprintf("Failed to fetch response from S3: %v", err))
	}

	// Set Content-Length from S3 object if not already in headers
	if _, ok := headers["Content-Length"]; !ok && result.ContentLength != nil {
		headers["Content-Length"] = strconv.FormatInt(*result.ContentLength, 10)
	}

	return &events.LambdaFunctionURLStreamingResponse{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       result.Body, // S3 GetObject body is already an io.ReadCloser
	}, nil
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
			case <-ctx.Done():
				return
			case <-streamTimeout:
				return
			case <-ticker.C:
				rawItem, err := dbClient.GetRawItem(ctx, pendingRequestsTable, reqKey)
				if err != nil {
					continue
				}

				// Forward all newly available chunks and collect indices to clean up
				var toDelete []int
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
						toDelete = append(toDelete, nextChunk)
						nextChunk++
					} else {
						break
					}
				}

				// Delete consumed chunks in one UpdateItem call to keep item size flat
				if len(toDelete) > 0 {
					removeExpr := "REMOVE "
					exprNames := map[string]string{}
					for i, idx := range toDelete {
						alias := fmt.Sprintf("#c%d", i)
						exprNames[alias] = fmt.Sprintf("stream_chunk_%d", idx)
						if i > 0 {
							removeExpr += ", "
						}
						removeExpr += alias
					}
					_ = dbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
						TableName:                aws.String(pendingRequestsTable),
						Key:                      reqKey,
						UpdateExpression:         aws.String(removeExpr),
						ExpressionAttributeNames: exprNames,
					})
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

// buildBufferedResponseFromItem returns a completed buffered response.
func buildBufferedResponseFromItem(ctx context.Context, rawItem map[string]types.AttributeValue) (*events.LambdaFunctionURLStreamingResponse, error) {
	// Check for S3-staged response first (large body)
	if s3KeyAV, ok := rawItem["s3_response_key"]; ok {
		if sv, ok := s3KeyAV.(*types.AttributeValueMemberS); ok && sv.Value != "" {
			if doneAV, ok2 := rawItem["s3_response_ready"]; ok2 {
				if bv, ok3 := doneAV.(*types.AttributeValueMemberBOOL); ok3 && bv.Value {
					return buildS3StreamingResponse(ctx, rawItem, sv.Value)
				}
			}
		}
	}
	return buildBufferedResponse(rawItem)
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

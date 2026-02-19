package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const chunkSize = 90 * 1024 // 90KB — stays under API Gateway's 128KB WebSocket message limit

// s3UploadThreshold is the response body size above which the CLI stages the
// response in S3 instead of sending it through DynamoDB (400 KB item limit).
const s3UploadThreshold = 256 * 1024 // 256 KB

// isBinaryContentType reports whether ct is a binary media type that should
// be staged through S3 rather than DynamoDB regardless of size.
func isBinaryContentType(ct string) bool {
	ct = strings.ToLower(ct)
	for _, prefix := range []string{"video/", "audio/", "image/", "application/octet-stream", "application/pdf", "application/zip"} {
		if strings.Contains(ct, prefix) {
			return true
		}
	}
	return false
}

// Proxy represents a local HTTP proxy
type Proxy struct {
	LocalPort      int
	WebSocketURL   string
	APIKey         string
	TunnelID       string
	conn           *websocket.Conn
	pendingReqs    map[string]chan *HTTPResponse
	pendingReqsMux sync.RWMutex
	writeMux       sync.Mutex
	chunkBuffers   map[string]map[int]string
	chunkMux       sync.Mutex
	stopCh         chan struct{}
}

// WebSocketMessage represents a message sent over the WebSocket connection
type WebSocketMessage struct {
	Action    string                 `json:"action"`
	RequestID string                 `json:"request_id,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// HTTPRequest represents an HTTP request
type HTTPRequest struct {
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body,omitempty"`
}

// HTTPResponse represents an HTTP response
type HTTPResponse struct {
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	Body       string              `json:"body,omitempty"`
}

// NewProxy creates a new proxy instance
func NewProxy(localPort int, websocketURL, apiKey, tunnelID string) *Proxy {
	return &Proxy{
		LocalPort:    localPort,
		WebSocketURL: websocketURL,
		APIKey:       apiKey,
		TunnelID:     tunnelID,
		pendingReqs:  make(map[string]chan *HTTPResponse),
		chunkBuffers: make(map[string]map[int]string),
		stopCh:       make(chan struct{}),
	}
}

// Start starts the proxy
func (p *Proxy) Start(ctx context.Context) error {
	// Connect to WebSocket
	if err := p.connectWebSocket(ctx); err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	// Start WebSocket message handler
	go p.handleWebSocketMessages(ctx)

	// Start ping/keep-alive loop
	go p.keepAlive(ctx)

	log.Printf("Proxy connected successfully")

	// Wait for context cancellation
	<-ctx.Done()

	// Cleanup
	close(p.stopCh)
	if p.conn != nil {
		p.conn.Close()
	}

	return ctx.Err()
}

// connectWebSocket establishes a WebSocket connection
func (p *Proxy) connectWebSocket(ctx context.Context) error {
	// Parse URL and add query parameters
	u, err := url.Parse(p.WebSocketURL)
	if err != nil {
		return fmt.Errorf("invalid WebSocket URL: %w", err)
	}

	// Add tunnel_id to query parameters if not already present
	q := u.Query()
	if q.Get("tunnel_id") == "" {
		q.Set("tunnel_id", p.TunnelID)
		u.RawQuery = q.Encode()
	}

	// Set up headers with authorization
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", p.APIKey))

	// Connect
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		return fmt.Errorf("failed to dial WebSocket: %w", err)
	}

	p.conn = conn
	return nil
}

// handleWebSocketMessages handles incoming WebSocket messages
func (p *Proxy) handleWebSocketMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		default:
			_, messageBytes, err := p.conn.ReadMessage()
			if err != nil {
				log.Printf("Error reading WebSocket message: %v", err)
				return
			}

			var message WebSocketMessage
			if err := json.Unmarshal(messageBytes, &message); err != nil {
				log.Printf("Error unmarshaling message: %v", err)
				continue
			}

			// Handle different message types
			switch message.Action {
			case "REQUEST":
				go p.handleHTTPRequest(ctx, message)
			case "proxy":
				go p.handleProxyRequest(ctx, message)
			case "proxy_chunk":
				p.handleProxyChunk(message)
			case "PONG":
				// Keep-alive response, no action needed
			default:
				log.Printf("Unknown message action: %s", message.Action)
			}
		}
	}
}

// handleHTTPRequest handles an incoming HTTP request from the tunnel
func (p *Proxy) handleHTTPRequest(ctx context.Context, message WebSocketMessage) {
	requestID := message.RequestID
	if requestID == "" {
		log.Printf("Request ID is missing")
		return
	}

	// Extract request details from message data
	method, _ := message.Data["method"].(string)
	path, _ := message.Data["path"].(string)
	body, _ := message.Data["body"].(string)

	// Convert headers
	headers := make(map[string][]string)
	if headersData, ok := message.Data["headers"].(map[string]interface{}); ok {
		for k, v := range headersData {
			if vArr, ok := v.([]interface{}); ok {
				strArr := make([]string, len(vArr))
				for i, val := range vArr {
					strArr[i] = fmt.Sprintf("%v", val)
				}
				headers[k] = strArr
			}
		}
	}

	// Forward request to local service
	localURL := fmt.Sprintf("http://localhost:%d%s", p.LocalPort, path)
	req, err := http.NewRequestWithContext(ctx, method, localURL, io.NopCloser(bytes.NewReader([]byte(body))))
	if err != nil {
		log.Printf("Failed to create local request: %v", err)
		p.sendErrorResponse(requestID, fmt.Sprintf("Failed to create request: %v", err))
		return
	}

	// Copy headers
	for k, v := range headers {
		for _, val := range v {
			req.Header.Add(k, val)
		}
	}

	// Make request to local service
	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to make local request: %v", err)
		p.sendErrorResponse(requestID, fmt.Sprintf("Failed to make request: %v", err))
		return
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response body: %v", err)
		p.sendErrorResponse(requestID, fmt.Sprintf("Failed to read response: %v", err))
		return
	}

	// Send response back through WebSocket
	httpResponse := HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       string(respBody),
	}

	responseMessage := WebSocketMessage{
		Action:    "RESPONSE",
		RequestID: requestID,
		Data: map[string]interface{}{
			"status_code": httpResponse.StatusCode,
			"headers":     httpResponse.Headers,
			"body":        httpResponse.Body,
		},
	}

	if err := p.sendWebSocketMessage(responseMessage); err != nil {
		log.Printf("Failed to send response: %v", err)
	}
}

// sendErrorResponse sends an error response back through the WebSocket
func (p *Proxy) sendErrorResponse(requestID, errorMsg string) {
	message := WebSocketMessage{
		Action:    "ERROR",
		RequestID: requestID,
		Error:     errorMsg,
	}

	if err := p.sendWebSocketMessage(message); err != nil {
		log.Printf("Failed to send error response: %v", err)
	}
}

// sendWebSocketMessage sends a message through the WebSocket
func (p *Proxy) sendWebSocketMessage(message WebSocketMessage) error {
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	p.writeMux.Lock()
	defer p.writeMux.Unlock()
	return p.conn.WriteMessage(websocket.TextMessage, messageBytes)
}

// handleProxyChunk stores an incoming request body chunk
func (p *Proxy) handleProxyChunk(message WebSocketMessage) {
	requestID, _ := message.Data["request_id"].(string)
	chunkIndexF, _ := message.Data["chunk_index"].(float64)
	data, _ := message.Data["data"].(string)

	p.chunkMux.Lock()
	defer p.chunkMux.Unlock()
	if p.chunkBuffers[requestID] == nil {
		p.chunkBuffers[requestID] = make(map[int]string)
	}
	p.chunkBuffers[requestID][int(chunkIndexF)] = data
}

// handleProxyRequest handles an incoming proxy request from the HTTP proxy Lambda
func (p *Proxy) handleProxyRequest(ctx context.Context, message WebSocketMessage) {
	// Extract request details from message.Data
	dataMap := message.Data
	if dataMap == nil {
		log.Printf("Invalid proxy request format")
		return
	}

	requestID, _ := dataMap["request_id"].(string)
	if requestID == "" {
		log.Printf("Request ID is missing in proxy request")
		return
	}

	method, _ := dataMap["method"].(string)
	path, _ := dataMap["path"].(string)
	body, _ := dataMap["body"].(string)
	// Presigned S3 URL provided by the Lambda for staging large/binary responses
	s3PutURL, _ := dataMap["s3_put_url"].(string)
	s3ResponseKey, _ := dataMap["s3_response_key"].(string)
	// For large inbound uploads: the request body is in S3 instead of in the message
	s3RequestGetURL, _ := dataMap["s3_request_get_url"].(string)

	// If body is in S3 (large upload flow), download it now
	if s3RequestGetURL != "" && body == "" {
		downloaded, dlErr := p.downloadFromS3(ctx, s3RequestGetURL)
		if dlErr != nil {
			log.Printf("Failed to download request body from S3 for request %s: %v", requestID, dlErr)
			p.sendProxyErrorResponse(requestID, fmt.Sprintf("Failed to download request body: %v", dlErr))
			return
		}
		body = string(downloaded)
		log.Printf("Downloaded %d byte request body from S3 for request %s", len(body), requestID)
	}

	// If body was chunked, assemble it from buffered chunks
	if totalChunksF, ok := dataMap["total_chunks"].(float64); ok && totalChunksF > 0 {
		totalChunks := int(totalChunksF)
		p.chunkMux.Lock()
		chunks := p.chunkBuffers[requestID]
		delete(p.chunkBuffers, requestID)
		p.chunkMux.Unlock()
		var buf strings.Builder
		for i := 0; i < totalChunks; i++ {
			buf.WriteString(chunks[i])
		}
		body = buf.String()
		log.Printf("Assembled %d chunks (%d bytes) for request %s", totalChunks, len(body), requestID)
	}

	log.Printf("Handling proxy request: %s %s (ID: %s)", method, path, requestID)

	// Convert headers from map[string]string to map[string][]string
	headers := make(map[string][]string)
	if headersData, ok := dataMap["headers"].(map[string]interface{}); ok {
		for k, v := range headersData {
			if strVal, ok := v.(string); ok {
				headers[k] = []string{strVal}
			}
		}
	}

	// Forward request to local service
	localURL := fmt.Sprintf("http://localhost:%d%s", p.LocalPort, path)
	req, err := http.NewRequestWithContext(ctx, method, localURL, io.NopCloser(bytes.NewReader([]byte(body))))
	if err != nil {
		log.Printf("Failed to create local request: %v", err)
		p.sendProxyErrorResponse(requestID, fmt.Sprintf("Failed to create request: %v", err))
		return
	}

	// Copy headers
	for k, v := range headers {
		for _, val := range v {
			req.Header.Add(k, val)
		}
	}

	// Make request to local service
	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to make local request: %v", err)
		p.sendProxyErrorResponse(requestID, fmt.Sprintf("Failed to make request: %v", err))
		return
	}

	// Detect SSE streaming responses and handle progressively.
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		log.Printf("Detected SSE streaming response for request %s, forwarding progressively", requestID)
		p.streamProxyResponse(ctx, requestID, resp)
		return
	}

	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response body: %v", err)
		p.sendProxyErrorResponse(requestID, fmt.Sprintf("Failed to read response: %v", err))
		return
	}

	// Convert response headers to map[string]string
	responseHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			responseHeaders[k] = v[0]
		}
	}

	// For large or binary responses, upload the body directly to S3 and notify
	// the Lambda via the proxy_response message (s3_response_key).
	// This avoids the DynamoDB 400 KB item-size limit and the per-message chunking overhead.
	if s3PutURL != "" && s3ResponseKey != "" &&
		(len(respBody) > s3UploadThreshold || isBinaryContentType(resp.Header.Get("Content-Type"))) {
		if err := p.uploadToS3(ctx, s3PutURL, resp.Header.Get("Content-Type"), respBody); err != nil {
			log.Printf("Failed to upload response to S3 for request %s: %v — falling back to inline", requestID, err)
			// Fall through to inline path on error
		} else {
			log.Printf("Uploaded %d byte response to S3 for request %s", len(respBody), requestID)
			responseMessage := WebSocketMessage{
				Action: "proxy_response",
				Data: map[string]interface{}{
					"request_id":       requestID,
					"status_code":      resp.StatusCode,
					"response_headers": responseHeaders,
					"response_body":    "",
					"s3_response_key":  s3ResponseKey,
				},
			}
			if err := p.sendWebSocketMessage(responseMessage); err != nil {
				log.Printf("Failed to send S3 proxy response for request %s: %v", requestID, err)
			} else {
				log.Printf("Sent S3 proxy response for request %s (status: %d)", requestID, resp.StatusCode)
			}
			return
		}
	}

	bodyStr := string(respBody)

	// Check total serialized message size against the 128 KB WebSocket limit
	testMsg := WebSocketMessage{
		Action: "proxy_response",
		Data: map[string]interface{}{
			"request_id":       requestID,
			"status_code":      resp.StatusCode,
			"response_headers": responseHeaders,
			"response_body":    bodyStr,
		},
	}
	testBytes, _ := json.Marshal(testMsg)

	// If total message exceeds WebSocket message limit, send body in chunks
	if len(testBytes) > 128*1024 {
		overhead := len(testBytes) - len(bodyStr)
		effectiveChunkSize := 120*1024 - overhead
		if effectiveChunkSize <= 0 || effectiveChunkSize > chunkSize {
			effectiveChunkSize = chunkSize
		}
		totalChunks := (len(bodyStr) + effectiveChunkSize - 1) / effectiveChunkSize
		log.Printf("Response too large (%d bytes total), sending body in %d chunks for request %s", len(testBytes), totalChunks, requestID)
		for i := 0; i < totalChunks; i++ {
			start := i * effectiveChunkSize
			end := start + effectiveChunkSize
			if end > len(bodyStr) {
				end = len(bodyStr)
			}
			chunkMsg := WebSocketMessage{
				Action: "proxy_response_chunk",
				Data: map[string]interface{}{
					"request_id":  requestID,
					"chunk_index": i,
					"data":        bodyStr[start:end],
				},
			}
			if err := p.sendWebSocketMessage(chunkMsg); err != nil {
				log.Printf("Failed to send chunk %d for request %s: %v", i, requestID, err)
				p.sendProxyErrorResponse(requestID, fmt.Sprintf("Failed to send chunk: %v", err))
				return
			}
		}
		responseMessage := WebSocketMessage{
			Action: "proxy_response",
			Data: map[string]interface{}{
				"request_id":       requestID,
				"status_code":      resp.StatusCode,
				"response_headers": responseHeaders,
				"response_body":    "",
				"total_chunks":     totalChunks,
			},
		}
		if err := p.sendWebSocketMessage(responseMessage); err != nil {
			log.Printf("Failed to send chunked proxy response for request %s: %v", requestID, err)
		} else {
			log.Printf("Sent chunked proxy response for request %s (status: %d, chunks: %d)", requestID, resp.StatusCode, totalChunks)
		}
		return
	}

	// Small response — send inline via WebSocket
	p.writeMux.Lock()
	err = p.conn.WriteMessage(websocket.TextMessage, testBytes)
	p.writeMux.Unlock()
	if err != nil {
		log.Printf("Failed to send proxy response: %v", err)
	} else {
		log.Printf("Sent proxy response for request %s (status: %d)", requestID, resp.StatusCode)
	}
}

// uploadToS3 performs an HTTP PUT of body to a presigned S3 URL.
func (p *Proxy) uploadToS3(ctx context.Context, presignedURL, contentType string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, presignedURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create S3 PUT request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.ContentLength = int64(len(body))

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("S3 PUT failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("S3 PUT returned status %d", resp.StatusCode)
	}
	return nil
}

// downloadFromS3 performs an HTTP GET from a presigned S3 URL and returns the body.
func (p *Proxy) downloadFromS3(ctx context.Context, presignedURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, presignedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 GET request: %w", err)
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("S3 GET failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("S3 GET returned status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// streamProxyResponse handles SSE responses by forwarding each event progressively
// via WebSocket using proxy_stream_start, proxy_stream_chunk, and proxy_stream_end messages.
func (p *Proxy) streamProxyResponse(ctx context.Context, requestID string, resp *http.Response) {
	defer resp.Body.Close()

	// Build flat response headers map
	responseHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			responseHeaders[k] = v[0]
		}
	}

	// Signal stream start (carries status code + headers)
	startMsg := WebSocketMessage{
		Action: "proxy_stream_start",
		Data: map[string]interface{}{
			"request_id":       requestID,
			"status_code":      resp.StatusCode,
			"response_headers": responseHeaders,
		},
	}
	if err := p.sendWebSocketMessage(startMsg); err != nil {
		log.Printf("Failed to send proxy_stream_start for request %s: %v", requestID, err)
		return
	}

	// Stream body as SSE events (data line + blank separator = one chunk) to halve DynamoDB writes.
	// bufio.Scanner with ScanLines returns empty string for blank lines.
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 512*1024) // handle long SSE lines
	chunkIndex := 0
	var pending string // accumulates current SSE event lines
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// Blank line = end of SSE event; send accumulated event as one chunk
			if pending != "" {
				chunkMsg := WebSocketMessage{
					Action: "proxy_stream_chunk",
					Data: map[string]interface{}{
						"request_id":  requestID,
						"chunk_index": chunkIndex,
						"data":        pending + "\n",
					},
				}
				if err := p.sendWebSocketMessage(chunkMsg); err != nil {
					log.Printf("Failed to send proxy_stream_chunk %d for request %s: %v", chunkIndex, requestID, err)
					return
				}
				chunkIndex++
				pending = ""
			}
		} else {
			pending += line + "\n"
		}
	}
	// Flush any remaining data
	if pending != "" {
		chunkMsg := WebSocketMessage{
			Action: "proxy_stream_chunk",
			Data: map[string]interface{}{
				"request_id":  requestID,
				"chunk_index": chunkIndex,
				"data":        pending + "\n",
			},
		}
		if err := p.sendWebSocketMessage(chunkMsg); err != nil {
			log.Printf("Failed to send proxy_stream_chunk %d for request %s: %v", chunkIndex, requestID, err)
		} else {
			chunkIndex++
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading streaming body for request %s: %v", requestID, err)
	}
	log.Printf("Streamed %d chunks for request %s", chunkIndex, requestID)

	// Signal end of stream
	endMsg := WebSocketMessage{
		Action: "proxy_stream_end",
		Data: map[string]interface{}{
			"request_id": requestID,
		},
	}
	if err := p.sendWebSocketMessage(endMsg); err != nil {
		log.Printf("Failed to send proxy_stream_end for request %s: %v", requestID, err)
	}
}

// sendProxyErrorResponse sends a proxy error response
func (p *Proxy) sendProxyErrorResponse(requestID, errorMsg string) {
	message := WebSocketMessage{
		Action: "proxy_response",
		Data: map[string]interface{}{
			"request_id":      requestID,
			"status_code":     500,
			"response_headers": map[string]string{"Content-Type": "application/json"},
			"response_body":   fmt.Sprintf(`{"error":"%s"}`, errorMsg),
		},
	}

	if err := p.sendWebSocketMessage(message); err != nil {
		log.Printf("Failed to send proxy error response: %v", err)
	}
}

// keepAlive sends periodic PING messages to keep the connection alive
func (p *Proxy) keepAlive(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			message := WebSocketMessage{
				Action: "PING",
			}

			if err := p.sendWebSocketMessage(message); err != nil {
				log.Printf("Failed to send PING: %v", err)
				return
			}
		}
	}
}

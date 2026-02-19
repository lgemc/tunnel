package models

import "time"

// Client represents a registered client
type Client struct {
	ClientID   string    `json:"client_id" dynamodbav:"client_id"`
	APIKeyHash string    `json:"-" dynamodbav:"api_key_hash"`
	Status     string    `json:"status" dynamodbav:"status"`
	CreatedAt  time.Time `json:"created_at" dynamodbav:"created_at"`
}

// Tunnel represents an active or inactive tunnel
type Tunnel struct {
	TunnelID     string    `json:"tunnel_id" dynamodbav:"tunnel_id"`
	ClientID     string    `json:"client_id" dynamodbav:"client_id"`
	Domain       string    `json:"domain" dynamodbav:"domain"`
	Subdomain    string    `json:"subdomain" dynamodbav:"subdomain"`
	Status       string    `json:"status" dynamodbav:"status"`
	ConnectionID string    `json:"connection_id,omitempty" dynamodbav:"connection_id,omitempty"`
	CreatedAt    time.Time `json:"created_at" dynamodbav:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" dynamodbav:"updated_at"`
}

// Domain represents a domain mapping to a tunnel
type Domain struct {
	Domain    string    `json:"domain" dynamodbav:"domain"`
	TunnelID  string    `json:"tunnel_id" dynamodbav:"tunnel_id"`
	ClientID  string    `json:"client_id" dynamodbav:"client_id"`
	CreatedAt time.Time `json:"created_at" dynamodbav:"created_at"`
}

// Constants for status values
const (
	ClientStatusActive   = "active"
	ClientStatusInactive = "inactive"

	TunnelStatusActive   = "active"
	TunnelStatusInactive = "inactive"
)

// WebSocket message types
const (
	MessageTypeConnect  = "CONNECT"
	MessageTypeRequest  = "REQUEST"
	MessageTypeResponse = "RESPONSE"
	MessageTypePing     = "PING"
	MessageTypePong     = "PONG"
	MessageTypeError    = "ERROR"
)

// WebSocketMessage represents a message sent over the WebSocket connection
type WebSocketMessage struct {
	Action    string                 `json:"action"`
	RequestID string                 `json:"request_id,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// HTTPRequest represents an HTTP request to be proxied
type HTTPRequest struct {
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body,omitempty"`
}

// HTTPResponse represents an HTTP response from the client
type HTTPResponse struct {
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	Body       string              `json:"body,omitempty"`
}

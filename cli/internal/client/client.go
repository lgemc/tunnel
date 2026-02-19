package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client represents a REST API client
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewClient creates a new API client
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{},
	}
}

// RegisterClientResponse represents the response from client registration
type RegisterClientResponse struct {
	ClientID string `json:"client_id"`
	APIKey   string `json:"api_key"`
	Message  string `json:"message"`
}

// CreateTunnelRequest represents a request to create a tunnel
type CreateTunnelRequest struct {
	Subdomain string `json:"subdomain,omitempty"`
}

// CreateTunnelResponse represents the response from creating a tunnel
type CreateTunnelResponse struct {
	TunnelID     string `json:"tunnel_id"`
	Domain       string `json:"domain"`
	Subdomain    string `json:"subdomain"`
	WebsocketURL string `json:"websocket_url"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	Reused       bool   `json:"reused,omitempty"`
}

// Tunnel represents a tunnel
type Tunnel struct {
	TunnelID     string `json:"tunnel_id"`
	ClientID     string `json:"client_id"`
	Domain       string `json:"domain"`
	Subdomain    string `json:"subdomain"`
	Status       string `json:"status"`
	ConnectionID string `json:"connection_id,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// ListTunnelsResponse represents the response from listing tunnels
type ListTunnelsResponse struct {
	Tunnels []Tunnel `json:"tunnels"`
	Count   int      `json:"count"`
}

// ErrorResponse represents an error response from the API
type ErrorResponse struct {
	Error string `json:"error"`
}

// RegisterClient registers a new client with the API
func (c *Client) RegisterClient() (*RegisterClientResponse, error) {
	url := fmt.Sprintf("%s/clients", c.BaseURL)

	resp, err := c.HTTPClient.Post(url, "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, errResp.Error)
	}

	var result RegisterClientResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// CreateTunnel creates a new tunnel
func (c *Client) CreateTunnel(subdomain string) (*CreateTunnelResponse, error) {
	url := fmt.Sprintf("%s/tunnels", c.BaseURL)

	reqBody := CreateTunnelRequest{
		Subdomain: subdomain,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, errResp.Error)
	}

	var result CreateTunnelResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ListTunnels lists all tunnels for the client
func (c *Client) ListTunnels() (*ListTunnelsResponse, error) {
	url := fmt.Sprintf("%s/tunnels", c.BaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, errResp.Error)
	}

	var result ListTunnelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// DeleteTunnel deletes a tunnel
func (c *Client) DeleteTunnel(tunnelID string) error {
	url := fmt.Sprintf("%s/tunnels/%s", c.BaseURL, tunnelID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("API error: %s", errResp.Error)
	}

	return nil
}

package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type TunnelItem struct {
	TunnelID     string    `json:"tunnel_id" dynamodbav:"tunnel_id"`
	ClientID     string    `json:"client_id" dynamodbav:"client_id"`
	Domain       string    `json:"domain" dynamodbav:"domain"`
	Subdomain    string    `json:"subdomain" dynamodbav:"subdomain"`
	Status       string    `json:"status" dynamodbav:"status"`
	ConnectionID string    `json:"connection_id,omitempty" dynamodbav:"connection_id,omitempty"`
	CreatedAt    time.Time `json:"created_at" dynamodbav:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" dynamodbav:"updated_at"`
}

// ListTunnels returns all tunnels from DynamoDB
func (h *Handler) ListTunnels(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	table := h.tableName("tunnels")

	// Optional filter by status
	statusFilter := r.URL.Query().Get("status")

	out, err := h.ddbClient.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(table),
		Limit:     aws.Int32(200),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to scan tunnels: "+err.Error())
		return
	}

	var tunnels []TunnelItem
	for _, item := range out.Items {
		var t TunnelItem
		if err := attributevalue.UnmarshalMap(item, &t); err != nil {
			continue
		}
		if statusFilter != "" && t.Status != statusFilter {
			continue
		}
		tunnels = append(tunnels, t)
	}

	active := 0
	inactive := 0
	for _, t := range tunnels {
		if t.Status == "active" {
			active++
		} else {
			inactive++
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tunnels":  tunnels,
		"count":    len(tunnels),
		"active":   active,
		"inactive": inactive,
	})
}

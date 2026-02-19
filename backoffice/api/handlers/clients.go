package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type ClientItem struct {
	ClientID  string    `json:"client_id" dynamodbav:"client_id"`
	Status    string    `json:"status" dynamodbav:"status"`
	CreatedAt time.Time `json:"created_at" dynamodbav:"created_at"`
}

// ListClients returns all clients from DynamoDB (without API key hashes)
func (h *Handler) ListClients(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	table := h.tableName("clients")

	out, err := h.ddbClient.Scan(ctx, &dynamodb.ScanInput{
		TableName:            aws.String(table),
		ProjectionExpression: aws.String("client_id, #s, created_at"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		Limit: aws.Int32(200),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to scan clients: "+err.Error())
		return
	}

	var clients []ClientItem
	for _, item := range out.Items {
		var c ClientItem
		if err := attributevalue.UnmarshalMap(item, &c); err != nil {
			continue
		}
		clients = append(clients, c)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"clients": clients,
		"count":   len(clients),
	})
}

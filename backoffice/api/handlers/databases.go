package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
)

type TableInfo struct {
	Name          string    `json:"name"`
	Status        string    `json:"status"`
	ItemCount     int64     `json:"item_count"`
	SizeBytes     int64     `json:"size_bytes"`
	BillingMode   string    `json:"billing_mode"`
	KeySchema     []KeyAttr `json:"key_schema"`
	CreatedAt     time.Time `json:"created_at"`
	LastUpdatedAt time.Time `json:"last_updated_at"`
	GSICount      int       `json:"gsi_count"`
}

type KeyAttr struct {
	Name string `json:"name"`
	Type string `json:"type"` // HASH or RANGE
}

// ListDatabases returns info about all project DynamoDB tables
func (h *Handler) ListDatabases(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	tables := []string{
		h.tableName("clients"),
		h.tableName("tunnels"),
		h.tableName("domains"),
		h.tableName("pending-requests"),
	}

	result := make([]TableInfo, 0, len(tables))
	for _, table := range tables {
		info := h.describeTable(ctx, table)
		result = append(result, info)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tables": result,
		"count":  len(result),
	})
}

func (h *Handler) describeTable(ctx context.Context, tableName string) TableInfo {
	info := TableInfo{Name: tableName, Status: "unknown"}
	out, err := h.ddbClient.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		info.Status = "error: " + err.Error()
		return info
	}
	t := out.Table
	if t.TableStatus != "" {
		info.Status = string(t.TableStatus)
	}
	if t.ItemCount != nil {
		info.ItemCount = *t.ItemCount
	}
	if t.TableSizeBytes != nil {
		info.SizeBytes = *t.TableSizeBytes
	}
	if t.BillingModeSummary != nil {
		info.BillingMode = string(t.BillingModeSummary.BillingMode)
	}
	for _, ks := range t.KeySchema {
		info.KeySchema = append(info.KeySchema, KeyAttr{
			Name: aws.ToString(ks.AttributeName),
			Type: string(ks.KeyType),
		})
	}
	if t.CreationDateTime != nil {
		info.CreatedAt = *t.CreationDateTime
	}
	info.GSICount = len(t.GlobalSecondaryIndexes)
	return info
}

// GetTableItems returns a sample of items from a DynamoDB table
func (h *Handler) GetTableItems(w http.ResponseWriter, r *http.Request) {
	table := r.PathValue("table")
	if table == "" {
		writeError(w, http.StatusBadRequest, "table name required")
		return
	}
	// Only allow project tables
	allowedTables := map[string]bool{
		h.tableName("clients"):          true,
		h.tableName("tunnels"):          true,
		h.tableName("domains"):          true,
		h.tableName("pending-requests"): true,
	}
	if !allowedTables[table] {
		writeError(w, http.StatusForbidden, "table not accessible")
		return
	}

	ctx := context.Background()
	out, err := h.ddbClient.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(table),
		Limit:     aws.Int32(50),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to scan table: "+err.Error())
		return
	}

	var items []map[string]interface{}
	for _, item := range out.Items {
		var m map[string]interface{}
		if err := attributevalue.UnmarshalMap(item, &m); err == nil {
			items = append(items, m)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"table":       table,
		"items":       items,
		"count":       len(items),
		"scanned":     out.ScannedCount,
		"has_more":    out.LastEvaluatedKey != nil,
	})
}

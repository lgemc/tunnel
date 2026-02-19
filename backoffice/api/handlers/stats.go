package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	lambdaservice "github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

type Stats struct {
	TotalLambdas    int       `json:"total_lambdas"`
	ActiveLambdas   int       `json:"active_lambdas"`
	TotalTunnels    int       `json:"total_tunnels"`
	ActiveTunnels   int       `json:"active_tunnels"`
	TotalClients    int       `json:"total_clients"`
	TotalDomains    int       `json:"total_domains"`
	PendingRequests int       `json:"pending_requests"`
	FetchedAt       time.Time `json:"fetched_at"`
}

// GetStats returns an aggregate overview of the system
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	stats := Stats{FetchedAt: time.Now()}

	// Count Lambdas
	prefix := h.cfg.ProjectName + "-"
	var marker *string
	var fns []lambdatypes.FunctionConfiguration
	for {
		out, err := h.lambdaClient.ListFunctions(ctx, &lambdaservice.ListFunctionsInput{Marker: marker})
		if err != nil {
			break
		}
		for _, fn := range out.Functions {
			if fn.FunctionName != nil && strings.HasPrefix(*fn.FunctionName, prefix) {
				fns = append(fns, fn)
			}
		}
		if out.NextMarker == nil {
			break
		}
		marker = out.NextMarker
	}
	stats.TotalLambdas = len(fns)
	for _, fn := range fns {
		if fn.State == lambdatypes.StateActive {
			stats.ActiveLambdas++
		}
	}

	// Get item counts from DynamoDB DescribeTable
	tableNames := []string{
		h.tableName("clients"),
		h.tableName("tunnels"),
		h.tableName("domains"),
		h.tableName("pending-requests"),
	}
	tableCounts := make(map[string]int64)
	for _, table := range tableNames {
		out, err := h.ddbClient.DescribeTable(ctx, &dynamodb.DescribeTableInput{
			TableName: aws.String(table),
		})
		if err != nil {
			continue
		}
		if out.Table.ItemCount != nil {
			tableCounts[table] = *out.Table.ItemCount
		}
	}

	stats.TotalClients = int(tableCounts[h.tableName("clients")])
	stats.TotalDomains = int(tableCounts[h.tableName("domains")])
	stats.PendingRequests = int(tableCounts[h.tableName("pending-requests")])
	stats.TotalTunnels = int(tableCounts[h.tableName("tunnels")])

	// Count active tunnels with a filter scan
	activeOut, err := h.ddbClient.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(h.tableName("tunnels")),
		FilterExpression: aws.String("#s = :active"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":active": &types.AttributeValueMemberS{Value: "active"},
		},
		Select: types.SelectCount,
	})
	if err == nil {
		stats.ActiveTunnels = int(activeOut.Count)
	}

	writeJSON(w, http.StatusOK, stats)
}

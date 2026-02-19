package main

// s3-upload-notify is triggered by S3 ObjectCreated events on the requests/ prefix.
// When an external client uploads a large request body directly to S3 (after calling
// POST /upload-url), this Lambda:
//   1. Reads the request metadata from the DynamoDB pending request (keyed by request_id
//      encoded in the S3 object key as "requests/{request_id}/body").
//   2. Generates a presigned S3 GET URL for the CLI to download the body.
//   3. Sends a proxy WebSocket message to the active tunnel CLI connection.
//   4. Polls DynamoDB for the CLI response (same 3-minute polling loop as http-proxy).
//   5. Stores the final response status in DynamoDB so the caller can retrieve it via
//      GET /poll/{request_id}.
//
// The original HTTP caller is polling GET /poll/{request_id} (handled by http-proxy Lambda)
// and will receive the response once this Lambda marks the request as completed.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
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
	uploadsBucket        string
	dbClient             *db.DynamoDBClient
	s3Client             *s3.Client
	s3PresignClient      *s3.PresignClient
)

func init() {
	domainsTable = os.Getenv("DOMAINS_TABLE")
	tunnelsTable = os.Getenv("TUNNELS_TABLE")
	pendingRequestsTable = os.Getenv("PENDING_REQUESTS_TABLE")
	websocketEndpoint = os.Getenv("WEBSOCKET_ENDPOINT")
	uploadsBucket = os.Getenv("UPLOADS_BUCKET")

	if tunnelsTable == "" || pendingRequestsTable == "" || websocketEndpoint == "" || uploadsBucket == "" {
		panic("Required environment variables are missing")
	}
}

func handler(ctx context.Context, event events.S3Event) error {
	if dbClient == nil {
		var err error
		dbClient, err = db.NewDynamoDBClient(ctx)
		if err != nil {
			return fmt.Errorf("failed to initialize DB client: %w", err)
		}
	}
	if s3Client == nil {
		cfg, err := dbClient.GetAWSConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to get AWS config: %w", err)
		}
		s3Client = s3.NewFromConfig(cfg)
		s3PresignClient = s3.NewPresignClient(s3Client)
	}

	for _, record := range event.Records {
		s3Key := record.S3.Object.Key
		log.Printf("s3-upload-notify: processing S3 key %s", s3Key)
		if err := processUpload(ctx, s3Key); err != nil {
			log.Printf("s3-upload-notify: error processing %s: %v", s3Key, err)
			// Continue processing other records — don't fail the whole batch
		}
	}
	return nil
}

// processUpload handles a single uploaded request body.
// S3 key format: requests/{request_id}/body
func processUpload(ctx context.Context, s3Key string) error {
	// Extract request_id from S3 key
	trimmed := strings.TrimPrefix(s3Key, "requests/")
	slashIdx := strings.Index(trimmed, "/")
	if slashIdx == -1 {
		return fmt.Errorf("unexpected S3 key format: %s", s3Key)
	}
	requestID := trimmed[:slashIdx]
	if requestID == "" {
		return fmt.Errorf("could not extract request_id from key: %s", s3Key)
	}
	log.Printf("s3-upload-notify: request_id=%s", requestID)

	// Fetch pending request from DynamoDB
	reqKey := map[string]types.AttributeValue{
		"request_id": &types.AttributeValueMemberS{Value: requestID},
	}
	rawItem, err := dbClient.GetRawItem(ctx, pendingRequestsTable, reqKey)
	if err != nil || rawItem == nil {
		return fmt.Errorf("pending request not found for request_id=%s: %v", requestID, err)
	}

	// Extract tunnel_id
	tunnelIDAV, ok := rawItem["tunnel_id"]
	if !ok {
		return fmt.Errorf("tunnel_id missing for request_id=%s", requestID)
	}
	tunnelIDSV, ok := tunnelIDAV.(*types.AttributeValueMemberS)
	if !ok || tunnelIDSV.Value == "" {
		return fmt.Errorf("tunnel_id empty for request_id=%s", requestID)
	}
	tunnelID := tunnelIDSV.Value

	// Look up tunnel connection
	var tunnel models.Tunnel
	if err := dbClient.GetItem(ctx, tunnelsTable, map[string]types.AttributeValue{
		"tunnel_id": &types.AttributeValueMemberS{Value: tunnelID},
	}, &tunnel); err != nil {
		return fmt.Errorf("tunnel not found for tunnel_id=%s: %v", tunnelID, err)
	}
	if tunnel.Status != models.TunnelStatusActive || tunnel.ConnectionID == "" {
		return fmt.Errorf("tunnel %s is not active or has no connection", tunnelID)
	}

	// Generate presigned GET URL for the CLI to download the request body
	presignReq, err := s3PresignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(uploadsBucket),
		Key:    aws.String(s3Key),
	}, s3.WithPresignExpires(30*time.Minute))
	if err != nil {
		return fmt.Errorf("failed to generate presigned GET URL: %w", err)
	}

	// Extract stored metadata (method, path, headers, s3_response_key, s3_response_put_url)
	method := "POST"
	if mv, ok := rawItem["method"]; ok {
		if sv, ok := mv.(*types.AttributeValueMemberS); ok {
			method = sv.Value
		}
	}
	path := "/"
	if pv, ok := rawItem["path"]; ok {
		if sv, ok := pv.(*types.AttributeValueMemberS); ok {
			path = sv.Value
		}
	}
	s3ResponseKey := ""
	if rk, ok := rawItem["s3_response_key"]; ok {
		if sv, ok := rk.(*types.AttributeValueMemberS); ok {
			s3ResponseKey = sv.Value
		}
	}
	s3ResponsePutURL := ""
	if ru, ok := rawItem["s3_response_put_url"]; ok {
		if sv, ok := ru.(*types.AttributeValueMemberS); ok {
			s3ResponsePutURL = sv.Value
		}
	}
	headers := map[string]interface{}{}
	if hv, ok := rawItem["headers"]; ok {
		if mv, ok := hv.(*types.AttributeValueMemberM); ok {
			for k, v := range mv.Value {
				if sv, ok := v.(*types.AttributeValueMemberS); ok {
					headers[k] = sv.Value
				}
			}
		}
	}

	// Build WebSocket API Gateway client
	cfg, err := dbClient.GetAWSConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get AWS config: %w", err)
	}
	apigwClient := apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = aws.String(websocketEndpoint)
	})

	// Mark request as pending (was waiting_upload) so the http-proxy poller picks it up
	_ = dbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(pendingRequestsTable),
		Key:       reqKey,
		UpdateExpression: aws.String("SET #s = :status"),
		ExpressionAttributeNames: map[string]string{"#s": "status"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: "pending"},
		},
	})

	// Send proxy WebSocket message to CLI
	proxyMsg, err := json.Marshal(map[string]interface{}{
		"action": "proxy",
		"data": map[string]interface{}{
			"request_id":        requestID,
			"method":            method,
			"path":              path,
			"headers":           headers,
			"body":              "",           // body is in S3
			"total_chunks":      0,
			"s3_request_key":    s3Key,        // CLI downloads body from here
			"s3_request_get_url": presignReq.URL,
			"s3_put_url":        s3ResponsePutURL, // CLI uploads response body here
			"s3_response_key":   s3ResponseKey,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal proxy message: %w", err)
	}

	if _, err = apigwClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(tunnel.ConnectionID),
		Data:         proxyMsg,
	}); err != nil {
		return fmt.Errorf("failed to send WebSocket message to CLI: %w", err)
	}

	log.Printf("s3-upload-notify: sent proxy message for request_id=%s to connection %s", requestID, tunnel.ConnectionID)
	return nil
}

func main() {
	lambda.Start(handler)
}

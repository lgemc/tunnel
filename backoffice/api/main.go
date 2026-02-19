package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	golambda "github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/lmanrique/tunnel/backoffice/api/handlers"
)

var httpLambda *httpadapter.HandlerAdapterV2

func init() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}

	appCfg := handlers.Config{
		AWSConfig:                cfg,
		ProjectName:              getEnv("PROJECT_NAME", "tunnel"),
		Environment:              getEnv("ENVIRONMENT", "dev"),
		AdminAPIKey:              os.Getenv("ADMIN_API_KEY"),
		CloudFrontDistributionID: os.Getenv("CLOUDFRONT_DISTRIBUTION_ID"),
		Region:                   getEnv("AWS_REGION", "us-east-1"),
	}

	mux := http.NewServeMux()
	h := handlers.New(appCfg)

	// Auth middleware wraps all routes
	auth := handlers.AuthMiddleware(appCfg.AdminAPIKey)

	mux.HandleFunc("GET /api/stats", auth(h.GetStats))
	mux.HandleFunc("GET /api/lambdas", auth(h.ListLambdas))
	mux.HandleFunc("GET /api/lambdas/{name}/logs", auth(h.GetLambdaLogs))
	mux.HandleFunc("GET /api/databases", auth(h.ListDatabases))
	mux.HandleFunc("GET /api/databases/{table}/items", auth(h.GetTableItems))
	mux.HandleFunc("GET /api/cloudfront", auth(h.GetCloudFront))
	mux.HandleFunc("GET /api/tunnels", auth(h.ListTunnels))
	mux.HandleFunc("GET /api/clients", auth(h.ListClients))

	httpLambda = httpadapter.NewV2(mux)
}

func handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	return httpLambda.ProxyWithContext(ctx, req)
}

func main() {
	golambda.Start(handler)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

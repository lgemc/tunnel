package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// Config holds application configuration
type Config struct {
	AWSConfig                aws.Config
	ProjectName              string
	Environment              string
	AdminAPIKey              string
	CloudFrontDistributionID string
	Region                   string
}

// Handler holds all AWS service clients
type Handler struct {
	cfg          Config
	lambdaClient *lambda.Client
	logsClient   *cloudwatchlogs.Client
	ddbClient    *dynamodb.Client
	cfClient     *cloudfront.Client
}

// New creates a new Handler with initialized AWS clients
func New(cfg Config) *Handler {
	return &Handler{
		cfg:          cfg,
		lambdaClient: lambda.NewFromConfig(cfg.AWSConfig),
		logsClient:   cloudwatchlogs.NewFromConfig(cfg.AWSConfig),
		ddbClient:    dynamodb.NewFromConfig(cfg.AWSConfig),
		cfClient:     cloudfront.NewFromConfig(cfg.AWSConfig),
	}
}

// AuthMiddleware validates the Bearer token
func AuthMiddleware(apiKey string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if apiKey == "" {
				// No key configured, allow all (dev mode)
				next(w, r)
				return
			}
			auth := r.Header.Get("Authorization")
			token := strings.TrimPrefix(auth, "Bearer ")
			if token != apiKey {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			next(w, r)
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// tableName returns the DynamoDB table name following the project naming convention
func (h *Handler) tableName(suffix string) string {
	return h.cfg.ProjectName + "-" + suffix + "-" + h.cfg.Environment
}

// lambdaName returns the Lambda function name following the naming convention
func (h *Handler) lambdaName(suffix string) string {
	return h.cfg.ProjectName + "-" + suffix + "-" + h.cfg.Environment
}

package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

type LambdaInfo struct {
	Name         string    `json:"name"`
	FunctionArn  string    `json:"function_arn"`
	Runtime      string    `json:"runtime"`
	Handler      string    `json:"handler"`
	MemorySize   int32     `json:"memory_size_mb"`
	Timeout      int32     `json:"timeout_seconds"`
	CodeSize     int64     `json:"code_size_bytes"`
	LastModified string    `json:"last_modified"`
	State        string    `json:"state"`
	Description  string    `json:"description"`
	LogGroup     string    `json:"log_group"`
	FetchedAt    time.Time `json:"fetched_at"`
}

// ListLambdas returns all Lambda functions belonging to this project
func (h *Handler) ListLambdas(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	prefix := h.cfg.ProjectName + "-"

	var functions []lambdatypes.FunctionConfiguration
	var marker *string

	for {
		out, err := h.lambdaClient.ListFunctions(ctx, &lambda.ListFunctionsInput{
			Marker: marker,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list Lambda functions: "+err.Error())
			return
		}
		for _, fn := range out.Functions {
			if fn.FunctionName != nil && len(*fn.FunctionName) >= len(prefix) &&
				(*fn.FunctionName)[:len(prefix)] == prefix {
				functions = append(functions, fn)
			}
		}
		if out.NextMarker == nil {
			break
		}
		marker = out.NextMarker
	}

	result := make([]LambdaInfo, 0, len(functions))
	for _, fn := range functions {
		info := LambdaInfo{
			FetchedAt: time.Now(),
		}
		if fn.FunctionName != nil {
			info.Name = *fn.FunctionName
		}
		if fn.FunctionArn != nil {
			info.FunctionArn = *fn.FunctionArn
		}
		if fn.Handler != nil {
			info.Handler = *fn.Handler
		}
		if fn.MemorySize != nil {
			info.MemorySize = *fn.MemorySize
		}
		if fn.Timeout != nil {
			info.Timeout = *fn.Timeout
		}
		info.CodeSize = fn.CodeSize
		if fn.LastModified != nil {
			info.LastModified = *fn.LastModified
		}
		if fn.Description != nil {
			info.Description = *fn.Description
		}
		info.Runtime = string(fn.Runtime)
		info.State = string(fn.State)
		info.LogGroup = "/aws/lambda/" + info.Name
		result = append(result, info)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"lambdas": result,
		"count":   len(result),
	})
}

// GetLambdaConfig returns detailed config for a single Lambda function
func (h *Handler) GetLambdaConfig(ctx context.Context, name string) (*LambdaInfo, error) {
	out, err := h.lambdaClient.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: aws.String(name),
	})
	if err != nil {
		return nil, err
	}
	fn := out.Configuration
	info := &LambdaInfo{
		FetchedAt: time.Now(),
	}
	if fn.FunctionName != nil {
		info.Name = *fn.FunctionName
	}
	if fn.FunctionArn != nil {
		info.FunctionArn = *fn.FunctionArn
	}
	if fn.Handler != nil {
		info.Handler = *fn.Handler
	}
	if fn.MemorySize != nil {
		info.MemorySize = *fn.MemorySize
	}
	if fn.Timeout != nil {
		info.Timeout = *fn.Timeout
	}
	info.CodeSize = fn.CodeSize
	if fn.LastModified != nil {
		info.LastModified = *fn.LastModified
	}
	if fn.Description != nil {
		info.Description = *fn.Description
	}
	info.Runtime = string(fn.Runtime)
	info.State = string(fn.State)
	info.LogGroup = "/aws/lambda/" + info.Name
	return info, nil
}

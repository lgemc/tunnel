package db

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoDBClient wraps the AWS DynamoDB client
type DynamoDBClient struct {
	client *dynamodb.Client
	cfg    aws.Config
}

// NewDynamoDBClient creates a new DynamoDB client
func NewDynamoDBClient(ctx context.Context) (*DynamoDBClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS config: %w", err)
	}

	return &DynamoDBClient{
		client: dynamodb.NewFromConfig(cfg),
		cfg:    cfg,
	}, nil
}

// GetAWSConfig returns the AWS config
func (d *DynamoDBClient) GetAWSConfig(ctx context.Context) (aws.Config, error) {
	return d.cfg, nil
}

// PutItem puts an item into a DynamoDB table
func (d *DynamoDBClient) PutItem(ctx context.Context, tableName string, item interface{}) error {
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	_, err = d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      av,
	})
	if err != nil {
		return fmt.Errorf("failed to put item: %w", err)
	}

	return nil
}

// GetItem retrieves an item from a DynamoDB table
func (d *DynamoDBClient) GetItem(ctx context.Context, tableName string, key map[string]types.AttributeValue, result interface{}) error {
	output, err := d.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key:       key,
	})
	if err != nil {
		return fmt.Errorf("failed to get item: %w", err)
	}

	if output.Item == nil {
		return fmt.Errorf("item not found")
	}

	err = attributevalue.UnmarshalMap(output.Item, result)
	if err != nil {
		return fmt.Errorf("failed to unmarshal item: %w", err)
	}

	return nil
}

// DeleteItem deletes an item from a DynamoDB table
func (d *DynamoDBClient) DeleteItem(ctx context.Context, tableName string, key map[string]types.AttributeValue) error {
	_, err := d.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key:       key,
	})
	if err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}

	return nil
}

// Query queries items from a DynamoDB table
func (d *DynamoDBClient) Query(ctx context.Context, input *dynamodb.QueryInput, results interface{}) error {
	output, err := d.client.Query(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to query items: %w", err)
	}

	err = attributevalue.UnmarshalListOfMaps(output.Items, results)
	if err != nil {
		return fmt.Errorf("failed to unmarshal items: %w", err)
	}

	return nil
}

// UpdateItem updates an item in a DynamoDB table
func (d *DynamoDBClient) UpdateItem(ctx context.Context, input *dynamodb.UpdateItemInput) error {
	_, err := d.client.UpdateItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update item: %w", err)
	}

	return nil
}

// Scan scans items from a DynamoDB table
func (d *DynamoDBClient) Scan(ctx context.Context, input *dynamodb.ScanInput, results interface{}) error {
	output, err := d.client.Scan(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to scan items: %w", err)
	}

	err = attributevalue.UnmarshalListOfMaps(output.Items, results)
	if err != nil {
		return fmt.Errorf("failed to unmarshal items: %w", err)
	}

	return nil
}

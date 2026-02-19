output "rest_api_endpoint" {
  description = "REST API Gateway endpoint URL"
  value       = aws_apigatewayv2_api.rest_api.api_endpoint
}

output "websocket_api_endpoint" {
  description = "WebSocket API Gateway endpoint URL"
  value       = aws_apigatewayv2_api.websocket_api.api_endpoint
}

output "cloudfront_domain" {
  description = "CloudFront distribution domain name"
  value       = var.enable_cloudfront ? aws_cloudfront_distribution.tunnel[0].domain_name : ""
}

output "dynamodb_clients_table" {
  description = "DynamoDB clients table name"
  value       = aws_dynamodb_table.clients.name
}

output "dynamodb_tunnels_table" {
  description = "DynamoDB tunnels table name"
  value       = aws_dynamodb_table.tunnels.name
}

output "dynamodb_domains_table" {
  description = "DynamoDB domains table name"
  value       = aws_dynamodb_table.domains.name
}

output "lambda_deployment_bucket" {
  description = "S3 bucket for Lambda deployments"
  value       = aws_s3_bucket.lambda_deployments.bucket
}

output "certificate_arn" {
  description = "ACM certificate ARN used by CloudFront"
  value       = local.certificate_arn
}

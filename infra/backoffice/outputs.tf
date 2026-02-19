output "backoffice_api_url" {
  description = "Backoffice API Gateway endpoint URL"
  value       = aws_apigatewayv2_api.backoffice.api_endpoint
}

output "backoffice_cloudfront_url" {
  description = "Backoffice CloudFront distribution URL"
  value       = "https://${aws_cloudfront_distribution.backoffice.domain_name}"
}

output "backoffice_admin_url" {
  description = "Backoffice admin URL (custom domain if configured)"
  value       = length(aws_route53_record.backoffice) > 0 ? "https://admin.${var.domain_name}" : "https://${aws_cloudfront_distribution.backoffice.domain_name}"
}

output "frontend_s3_bucket" {
  description = "S3 bucket name for the frontend assets"
  value       = aws_s3_bucket.frontend.bucket
}

output "cloudfront_distribution_id" {
  description = "CloudFront distribution ID for cache invalidation"
  value       = aws_cloudfront_distribution.backoffice.id
}

output "backoffice_lambda_name" {
  description = "Backoffice Lambda function name"
  value       = aws_lambda_function.backoffice_api.function_name
}

# CloudWatch log group for the backoffice Lambda
resource "aws_cloudwatch_log_group" "backoffice_api" {
  name              = "/aws/lambda/${local.name_prefix}-api"
  retention_in_days = 14
  tags              = local.common_tags
}

# Placeholder zip (replaced by build + update script)
data "archive_file" "backoffice_placeholder" {
  type        = "zip"
  output_path = "${path.module}/.terraform/lambda-placeholders/backoffice-api.zip"

  source {
    content  = "placeholder"
    filename = "bootstrap"
  }
}

# Backoffice API Lambda
resource "aws_lambda_function" "backoffice_api" {
  function_name = "${local.name_prefix}-api"
  role          = aws_iam_role.backoffice_lambda.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  timeout       = 30
  memory_size   = 256

  filename         = data.archive_file.backoffice_placeholder.output_path
  source_code_hash = data.archive_file.backoffice_placeholder.output_base64sha256

  tags = local.common_tags

  environment {
    variables = {
      PROJECT_NAME               = var.project_name
      ENVIRONMENT                = var.environment
      ADMIN_API_KEY              = var.admin_api_key
      CLOUDFRONT_DISTRIBUTION_ID = var.cloudfront_distribution_id
      AWS_REGION_NAME            = var.aws_region
    }
  }

  depends_on = [
    aws_cloudwatch_log_group.backoffice_api,
    aws_iam_role_policy_attachment.basic_execution,
  ]
}

# Lambda permission for API Gateway
resource "aws_lambda_permission" "backoffice_apigw" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.backoffice_api.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.backoffice.execution_arn}/*/*"
}
